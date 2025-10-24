package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger" // 替换为实际日志包路径
	"github.com/DotNetAge/sparrow/pkg/usecase"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

/*
NOTES: 当前设计关键性考虑
1.生命周期精确控制 ：虽然Consume方法是异步的，但它缺乏对消费过程的精细控制。通过额外的goroutine和WaitGroup，我们可以更精确地跟踪和管理消费者的生命周期。
2.优雅关闭机制 ：原始的Consume没有直接提供优雅关闭机制。我们实现的Stop方法通过context取消和等待处理完成，确保了消息不会在关闭过程中丢失。
3.健壮性提升 ：通过在goroutine中添加defer recover机制，我们防止了单个消息处理panic导致整个消费者崩溃。
4.状态管理 ：增加了running标志和互斥锁，避免了重复启动或停止操作的问题。
5.符合Go的设计惯例 ：这种设计更符合Go中"显式控制"的哲学，使得消费者的行为更加可预测和可控。
*/

// JetStreamBus 事件流总线
type JetStreamBus struct {
	StreamSubscriber
	Subscribers
	usecase.GracefulClose
	usecase.Startable
	js             jetstream.JetStream
	serviceName    string // 服务名，同时作为流名称
	logger         *logger.Logger
	handlers       map[string]DomainEventHandler[*entity.BaseEvent]
	filterSubjects []string
	consumer       jetstream.Consumer // JetStream消费者实例
	consumerName   string
	consumeCtx     context.Context    // 用于控制消费的上下文
	cancel         context.CancelFunc // 取消消费的函数
	wg             sync.WaitGroup     // 等待所有处理完成的WaitGroup
	running        bool               // 运行状态标志
	mu             sync.Mutex         // 保护running状态的互斥锁
}

func NewJetStreamBus(
	conn *nats.Conn,
	serviceName string,
	logger *logger.Logger,
) StreamSubscriber {
	js, err := jetstream.New(conn)
	if err != nil {
		logger.Fatal("获取JetStream客户端失败", "error", err)
		panic(err)
	}

	return &JetStreamBus{
		js:           js,
		serviceName:  serviceName,
		consumerName: fmt.Sprintf("%s-CONSUMER", strings.ToUpper(serviceName)),
		handlers:     make(map[string]DomainEventHandler[*entity.BaseEvent]),
		logger:       logger,
		running:      false,
	}
}

// Start 启动订阅（实现StreamSubscriber接口）
func (s *JetStreamBus) Start(ctx context.Context) error {
	s.mu.Lock()
	// 检查是否已经在运行
	if s.running {
		s.mu.Unlock()
		s.logger.Warn("订阅器已经在运行中", "consumer", fmt.Sprintf("%s", s.serviceName))
		return nil
	}
	s.running = true
	s.mu.Unlock()

	// 创建用于控制消费的上下文，支持取消
	consumeCtx, cancel := context.WithCancel(context.Background())
	s.consumeCtx = consumeCtx
	s.cancel = cancel

	// 1. 配置消费者
	cfg := jetstream.ConsumerConfig{
		Durable:        s.consumerName,              // 持久化消费者（重启不丢失进度）
		AckPolicy:      jetstream.AckExplicitPolicy, // 保持显式确认以确保可靠处理
		FilterSubjects: s.filterSubjects,
		DeliverPolicy:  jetstream.DeliverAllPolicy,    // 从最早的消息开始投递，支持事件重放
		ReplayPolicy:   jetstream.ReplayInstantPolicy, // 立即重放所有消息
	}

	// 2. 获取流并创建消费者
	stream, err := s.js.Stream(ctx, s.serviceName)
	if err != nil {
		s.logger.Fatal("获取流失败", "consumer", s.consumerName, "stream", s.serviceName, "error", err)
		s.cleanup()
		return fmt.Errorf("获取流失败: %w", err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		s.logger.Fatal("创建消费者失败", "consumer", s.consumerName, "stream", s.serviceName, "error", err)
		s.cleanup()
		return fmt.Errorf("创建消费者失败: %w", err)
	}
	s.consumer = consumer

	// 3. 异步启动消费
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runConsumer(consumeCtx, consumer)
	}()

	s.logger.Info("订阅器启动成功", "consumer", s.consumerName)
	return nil
}

// runConsumer 在goroutine中运行消费者
func (s *JetStreamBus) runConsumer(ctx context.Context, consumer jetstream.Consumer) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("消费者处理发生panic", "consumer", s.consumerName, "panic", r)
		}
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	// 开始消费，传入ctx用于控制
	_, err := consumer.Consume(func(msg jetstream.Msg) {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			// 如果上下文已取消，简单确认消息并返回
			if err := msg.Ack(); err != nil {
				s.logger.Warn("取消上下文后确认消息失败", "consumer", s.consumerName, "error", err)
			}
			return
		default:
			// 正常处理消息
			s.handleMessage(msg)
		}
	})

	if err != nil && err != context.Canceled {
		s.logger.Error("消费者运行失败", "consumer", s.consumerName, "error", err)
	}

}

// Stop 停止订阅（实现StreamSubscriber接口）- 支持优雅关闭
func (s *JetStreamBus) Stop(ctx context.Context) error {
	s.mu.Lock()
	// 检查是否已经停止
	if !s.running {
		s.mu.Unlock()
		s.logger.Warn("订阅器已经停止", "consumer", s.consumerName)
		return nil
	}
	s.mu.Unlock()

	s.logger.Info("开始停止订阅器", "consumer", s.consumerName)

	// 1. 取消消费上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 2. 等待所有处理完成，但支持超时
	doneChan := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(doneChan)
	}()

	// 3. 等待处理完成或超时
	select {
	case <-doneChan:
		// 处理完成
	case <-ctx.Done():
		// 超时
		s.logger.Warn("停止订阅器超时，强制退出", "consumer", s.consumerName)
		return ctx.Err()
	}

	// 4. 清理资源
	s.cleanup()

	s.logger.Info("订阅器已优雅停止", "consumer", s.consumerName)
	return nil
}

// Close 实现GracefulClose接口，支持优雅关闭
func (s *JetStreamBus) Close(ctx context.Context) error {
	return s.Stop(ctx)
}

// cleanup 清理资源
func (s *JetStreamBus) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	s.consumer = nil
	s.consumeCtx = nil
	s.cancel = nil
}

// handleMessage 处理单条消息（内部方法，继承结构体泛型）
func (s *JetStreamBus) handleMessage(msg jetstream.Msg) {
	// 检查消费上下文是否已取消
	if s.consumeCtx != nil && s.consumeCtx.Err() != nil {
		// 如果消费已取消，简单确认消息
		if err := msg.Ack(); err != nil {
			s.logger.Warn("消费上下文已取消时确认消息失败", "subject", msg.Subject(), "error", err)
		}
		return
	}

	// 使用消费上下文，支持取消
	ctx := s.consumeCtx
	if ctx == nil {
		ctx = context.Background()
	}

	detailSubject := msg.Subject()
	aggType := strings.Split(detailSubject, ".")[0]
	eventType := strings.Split(detailSubject, ".")[2]
	subjectPattern := fmt.Sprintf("%s.*.%s", aggType, eventType)

	handler := s.handlers[subjectPattern]
	if handler == nil {
		s.logger.Error("未找到处理程序", "consumer", s.consumerName, "subject", msg.Subject())
		msg.Nak() // 标记处理失败
		return
	}

	var baseEvent entity.BaseEvent

	if err := json.Unmarshal(msg.Data(), &baseEvent); err != nil {
		s.logger.Error("消息反序列化失败", "consumer", s.consumerName, "error", err, "subject", msg.Subject())
		msg.Nak() // 标记处理失败
		return
	}

	// 2. 调用类型安全的处理器
	if err := handler(ctx, &baseEvent); err != nil {
		s.logger.Error("事件处理失败", "consumer", s.consumerName, "error", err, "eventID", baseEvent.GetEventID())
		msg.Nak() // 失败重试
		return
	}

	// 3. 确认处理完成
	if err := msg.Ack(); err != nil {
		s.logger.Warn("消息确认失败", "consumer", s.consumerName, "error", err)
	}
}

func (s *JetStreamBus) AddHandler(aggType, eventType string, handler DomainEventHandler[*entity.BaseEvent]) {
	subject := fmt.Sprintf("%s.*.%s", aggType, eventType)
	s.handlers[subject] = handler
}
