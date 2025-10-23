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
	"github.com/DotNetAge/sparrow/pkg/utils"

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

// JetStreamSubscriber 泛型订阅器（绑定单一事件类型T）
// 结构体带泛型参数，方法继承泛型约束
//
// 只适用于对外部事件单一事件的订阅，并且仅对无严格事件序列要求的场合。
// 因为如果在同一进程用启用多个Subscriber，就会进入并行运行状态，
// 每个Subscriber都有自己的消费者实例，各自独立消费消息。
// 这可能会导致事件处理的无序性和不一致性，
// 特别是当多个Subscriber处理相同聚合类型的事件时。
type JetStreamSubscriber[T DomainEventConstraint] struct {
	StreamSubscriber
	usecase.GracefulClose
	js          jetstream.JetStream
	serviceName string // 服务名，同时作为流名称
	aggType     string // 聚合类型
	logger      *logger.Logger
	eventType   string                // 事件类型
	handler     DomainEventHandler[T] // 该类型事件的处理器
	consumer    jetstream.Consumer    // JetStream消费者实例
	consumeCtx  context.Context       // 用于控制消费的上下文
	cancel      context.CancelFunc    // 取消消费的函数
	wg          sync.WaitGroup        // 等待所有处理完成的WaitGroup
	running     bool                  // 运行状态标志
	mu          sync.Mutex            // 保护running状态的互斥锁
}

func NewJetStreamNamedEventSubscriber[T DomainEventConstraint](
	conn *nats.Conn,
	serviceName string,
	aggType string,
	eventType string,
	logger *logger.Logger,
	handler DomainEventHandler[T],
) StreamSubscriber {
	js, err := jetstream.New(conn)
	if err != nil {
		logger.Fatal("获取JetStream客户端失败", "error", err)
		panic(err)
	}

	return &JetStreamSubscriber[T]{
		js:          js,
		serviceName: serviceName,
		aggType:     aggType,
		logger:      logger,
		handler:     handler,
		eventType:   eventType,
		running:     false,
	}
}

// NewJetStreamSubscriber 创建订阅器（指定事件类型T和处理器）
func NewJetStreamSubscriber[T DomainEventConstraint](
	conn *nats.Conn,
	serviceName string,
	aggType string,
	logger *logger.Logger,
	handler DomainEventHandler[T],
) StreamSubscriber {
	et := utils.GetTypeName[T]()
	names := strings.Split(et, ".")
	eventType := names[len(names)-1]

	return NewJetStreamNamedEventSubscriber(conn, serviceName, aggType, eventType, logger, handler)
}

// Start 启动订阅（实现StreamSubscriber接口）
func (s *JetStreamSubscriber[T]) Start(ctx context.Context) error {
	s.mu.Lock()
	// 检查是否已经在运行
	if s.running {
		s.mu.Unlock()
		s.logger.Warn("订阅器已经在运行中", "consumer", fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType))
		return nil
	}
	s.running = true
	s.mu.Unlock()

	// 创建用于控制消费的上下文，支持取消
	consumeCtx, cancel := context.WithCancel(context.Background())
	s.consumeCtx = consumeCtx
	s.cancel = cancel

	// 1. 配置消费者
	consumerName := fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,                                   // 持久化消费者（重启不丢失进度）
		AckPolicy:     jetstream.AckExplicitPolicy,                    // 保持显式确认以确保可靠处理
		FilterSubject: fmt.Sprintf("%s.*.%s", s.aggType, s.eventType), // 只订阅当前事件类型
		DeliverPolicy: jetstream.DeliverAllPolicy,                     // 从最早的消息开始投递，支持事件重放
		ReplayPolicy:  jetstream.ReplayInstantPolicy,                  // 立即重放所有消息
	}

	// 2. 获取流并创建消费者
	stream, err := s.js.Stream(ctx, s.serviceName)
	if err != nil {
		s.logger.Fatal("获取流失败", "consumer", consumerName, "stream", s.serviceName, "error", err)
		s.cleanup()
		return fmt.Errorf("获取流失败: %w", err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		s.logger.Fatal("创建消费者失败", "consumer", consumerName, "stream", s.serviceName, "error", err)
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

	s.logger.Info("订阅器启动成功", "consumer", consumerName)
	return nil
}

// runConsumer 在goroutine中运行消费者
func (s *JetStreamSubscriber[T]) runConsumer(ctx context.Context, consumer jetstream.Consumer) {
	consumerName := fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType)
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("消费者处理发生panic", "consumer", consumerName, "panic", r)
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
				s.logger.Warn("取消上下文后确认消息失败", "consumer", consumerName, "error", err)
			}
			return
		default:
			// 正常处理消息
			s.handleMessage(msg)
		}
	})

	if err != nil && err != context.Canceled {
		s.logger.Error("消费者运行失败", "consumer", consumerName, "error", err)
	}

}

// Stop 停止订阅（实现StreamSubscriber接口）- 支持优雅关闭
func (s *JetStreamSubscriber[T]) Stop(ctx context.Context) error {
	consumerName := fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType)

	s.mu.Lock()
	// 检查是否已经停止
	if !s.running {
		s.mu.Unlock()
		s.logger.Warn("订阅器已经停止", "consumer", consumerName)
		return nil
	}
	s.mu.Unlock()

	s.logger.Info("开始停止订阅器", "consumer", consumerName)

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
		s.logger.Warn("停止订阅器超时，强制退出", "consumer", consumerName)
		return ctx.Err()
	}

	// 4. 清理资源
	s.cleanup()

	s.logger.Info("订阅器已优雅停止", "consumer", consumerName)
	return nil
}

// Close 实现GracefulClose接口，支持优雅关闭
func (s *JetStreamSubscriber[T]) Close(ctx context.Context) error {
	return s.Stop(ctx)
}

// cleanup 清理资源
func (s *JetStreamSubscriber[T]) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	s.consumer = nil
	s.consumeCtx = nil
	s.cancel = nil
}

// handleMessage 处理单条消息（内部方法，继承结构体泛型）
func (s *JetStreamSubscriber[T]) handleMessage(msg jetstream.Msg) {
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

	// 1. 反序列化为T类型事件
	// NOTES: 这是一个间接的过程，为了让每个类都能保持强类型化与弱类型化事件的兼容，存在流中的是弱类型的通用事件BaseEvent，
	// 而在BaseEvent.Playload 中才是强类型的序列化后的JSON字节流
	var event T
	var baseEvent entity.BaseEvent

	if err := json.Unmarshal(msg.Data(), &baseEvent); err != nil {
		consumerName := fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType)
		s.logger.Error("消息反序列化失败", "consumer", consumerName, "error", err, "subject", msg.Subject())
		msg.Nak() // 标记处理失败
		return
	}

	event, err := entity.UnmarshalEvent[T](&baseEvent)
	if err != nil {
		consumerName := fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType)
		s.logger.Error("事件负载反序列化失败", "consumer", consumerName, "error", err, "eventID", baseEvent.Id)
		msg.Nak() // 标记处理失败
		return
	}

	// 2. 调用类型安全的处理器
	if err := s.handler(ctx, event); err != nil {
		consumerName := fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType)
		s.logger.Error("事件处理失败", "consumer", consumerName, "error", err, "eventID", event.GetEventID())
		msg.Nak() // 失败重试
		return
	}

	// 3. 确认处理完成
	if err := msg.Ack(); err != nil {
		consumerName := fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType)
		s.logger.Warn("消息确认失败", "consumer", consumerName, "error", err)
	}
}
