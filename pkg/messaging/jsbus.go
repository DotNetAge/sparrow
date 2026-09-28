package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger" // 替换为实际日志包路径
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"gorm.io/gorm/utils"

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
	name           string // 本服务名，用作消费者的唯一名
	serviceName    string // 服务名，同时作为流名称
	logger         *logger.Logger
	handlers       map[string]DomainEventHandler[*entity.BaseEvent]
	filterSubjects []string
	consumer       jetstream.Consumer // JetStream消费者实例
	consumerName   string
	consumeCtx     context.Context     // 用于控制消费的上下文
	cancel         context.CancelFunc  // 取消消费的函数
	wg             sync.WaitGroup      // 等待所有处理完成的WaitGroup
	running        bool                // 运行状态标志
	mu             sync.Mutex          // 保护running状态的互斥锁
	dedupeMu       sync.Mutex          // 保护已处理事件位图的互斥锁
	seenEvents     map[string]struct{} // 已成功处理的事件位图，键：{aggregateID}:{eventID}

	// 死信队列（P2）：handler 连续失败达阈值后转入死信流，可审计可重放
	dlqWriter  *DLQWriter
	dlqTracker *dlqTracker
}

// BusOption 订阅总线选项
type BusOption func(*JetStreamBus)

// WithMaxDeliver 设置死信队列的最大投递次数（handler 连续失败超过该值转入死信）
func WithMaxDeliver(maxDeliver int) BusOption {
	return func(b *JetStreamBus) {
		if b.dlqTracker != nil {
			b.dlqTracker.maxDeliver = maxDeliver
		}
	}
}

// WithDLQDisabled 禁用死信队列（失败时无限重试，保持旧语义）
func WithDLQDisabled() BusOption {
	return func(b *JetStreamBus) {
		b.dlqTracker = nil
		b.dlqWriter = nil
	}
}

func NewJetStreamBus(
	conn *nats.Conn,
	name string,
	serviceName string,
	logger *logger.Logger,
	options ...BusOption,
) StreamSubscriber {
	js, err := jetstream.New(conn)
	if err != nil {
		logger.Panic("获取JetStream客户端失败", "error", err)
	}

	// RULES: 同一个流之下，所有消费者名称必须唯一，否则后创建的消费者就会覆盖前一个的配置！
	// 消费者名称格式：{服务名}.{服务实例名}.Consumer
	consumerName := fmt.Sprintf("%s-%s-Consumer", name, serviceName)

	bus := &JetStreamBus{
		js:           js,
		name:         name,
		serviceName:  serviceName,
		consumerName: consumerName,
		handlers:     make(map[string]DomainEventHandler[*entity.BaseEvent]),
		seenEvents:   make(map[string]struct{}),
		logger:       logger,
		running:      false,
	}

	// 默认启用死信队列：handler 连续失败 MaxDeliver(3) 次后转入死信，避免无限重试阻塞消费
	bus.dlqTracker = newDLQTracker(3)
	for _, opt := range options {
		opt(bus)
	}
	if bus.dlqTracker != nil {
		bus.dlqWriter = NewDLQWriter(js, serviceName, logger)
	}

	return bus
}

// Start 启动订阅（实现StreamSubscriber接口）
func (s *JetStreamBus) Start(ctx context.Context) error {
	s.mu.Lock()
	// 检查是否已经在运行
	if s.running {
		s.mu.Unlock()
		s.logger.Warn("订阅器已经在运行中", "serviceName", s.serviceName, "consumerName", s.consumerName)
		return nil
	}
	s.running = true
	s.mu.Unlock()

	// 创建用于控制消费的上下文，支持取消
	consumeCtx, cancel := context.WithCancel(context.Background())
	s.consumeCtx = consumeCtx
	s.cancel = cancel

	// 确保死信流存在（幂等）
	if s.dlqWriter != nil {
		if err := s.dlqWriter.EnsureStream(ctx); err != nil {
			s.logger.Error("死信流初始化失败，本次启动禁用死信队列", "consumer", s.consumerName, "error", err)
			s.dlqWriter = nil
		}
	}

	// 1. 配置消费者
	cfg := jetstream.ConsumerConfig{
		Durable:        s.consumerName,              // 持久化消费者（重启不丢失进度）
		AckPolicy:      jetstream.AckExplicitPolicy, // 保持显式确认以确保可靠处理
		FilterSubjects: s.filterSubjects,
		// DeliverPolicy:  jetstream.DeliverLastPolicy, // 首次启动从最新消息开始（避免重复处理历史）
		MaxAckPending: 1000, // 流控阈值（避免未确认消息堆积）
		// RateLimit:     500,  // 速率限制（每秒 500 条，保护投影处理）		ReplayPolicy:   jetstream.ReplayInstantPolicy, // 立即重放所有消息
		// 核心新增：跨进程消费者共享的分组名称（所有消费者实例必须相同）
		DeliverGroup: fmt.Sprintf("%s.Consumer.Group", s.serviceName), // 启用负载均衡，确保消息均匀分布。投递组是竞争模式，确保每个消息只被一个消费者处理。
		// 可选优化：确保连接稳定
		// FlowControl:   true,             // 启用流量控制，避免消息过载
		// IdleHeartbeat: 15 * time.Second, // 15秒心跳，检测连接状态
		// MaxAckPending: 1000,             // 最大未确认消息数（根据业务调整）
	}

	// 2. 获取流并创建消费者
	stream, err := s.js.Stream(ctx, s.serviceName)
	if err != nil {
		s.logger.Error("获取流失败", "consumer", s.consumerName, "stream", s.serviceName, "error", err)
		s.cleanup()
		return fmt.Errorf("获取流失败: %w", err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		s.logger.Error("创建消费者失败", "consumer", s.consumerName, "stream", s.serviceName, "error", err)
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

	s.logger.Info("订阅器启动成功", "consumer", s.consumerName, "stream", s.serviceName, "subjects", cfg.FilterSubjects)
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
	parts := strings.Split(detailSubject, ".")
	if len(parts) < 3 {
		s.logger.Error("事件主题格式无效，跳过", "consumer", s.consumerName, "subject", detailSubject)
		msg.Ack() // 格式无效的消息直接确认丢弃，避免无限重试
		return
	}
	aggType := parts[0]
	eventType := parts[2]
	subjectPattern := fmt.Sprintf("%s.*.%s", aggType, eventType)

	handler := s.handlers[subjectPattern]
	if handler == nil {
		s.logger.Warn("未找到处理程序，跳过", "consumer", s.consumerName, "subject", msg.Subject())
		msg.Ack() // 本服务未注册该事件类型，确认丢弃，避免 Nak 无限重试阻塞消费
		return
	}

	var baseEvent entity.BaseEvent

	if err := json.Unmarshal(msg.Data(), &baseEvent); err != nil {
		s.logger.Error("消息反序列化失败", "consumer", s.consumerName, "error", err, "subject", msg.Subject())
		msg.Nak() // 标记处理失败
		return
	}

	// 2. 幂等去重：同一 (聚合ID, 事件ID) 已成功处理过则跳过 handler，直接确认
	//    事件ID为空（存量事件）时不做去重，避免误杀
	eventKey := baseEvent.GetAggregateID() + ":" + baseEvent.GetEventID()
	if baseEvent.GetEventID() != "" {
		s.dedupeMu.Lock()
		_, seen := s.seenEvents[eventKey]
		s.dedupeMu.Unlock()
		if seen {
			s.logger.Info("重复事件已成功处理过，跳过", "consumer", s.consumerName, "eventID", baseEvent.GetEventID())
			msg.Ack()
			return
		}
	}

	// 3. 调用类型安全的处理器
	if err := handler(ctx, &baseEvent); err != nil {
		s.logger.Error("事件处理失败", "consumer", s.consumerName, "error", err, "eventID", baseEvent.GetEventID())
		// 死信兜底（P2）：连续失败达阈值后转入死信队列并确认原消息，避免无限重试阻塞消费
		if s.dlqWriter != nil && s.dlqTracker.record(eventKey) {
			if werr := s.dlqWriter.Write(ctx, detailSubject, &baseEvent, err.Error()); werr != nil {
				msg.Nak() // 死信写入失败则继续重试
				return
			}
			if ackErr := msg.Ack(); ackErr != nil {
				s.logger.Warn("死信消息确认失败", "consumer", s.consumerName, "error", ackErr)
			}
			return
		}
		msg.NakWithDelay(1 * time.Second) // 失败重试（不记录已处理，保证重试可再次执行；短延迟便于快速收敛）
		return
	}

	// 4. 处理成功后记录已处理，供后续重复投递去重
	if baseEvent.GetEventID() != "" {
		s.dedupeMu.Lock()
		s.seenEvents[eventKey] = struct{}{}
		s.dedupeMu.Unlock()
	}

	// 5. 确认处理完成
	if err := msg.Ack(); err != nil {
		s.logger.Warn("消息确认失败", "consumer", s.consumerName, "error", err)
	}
}

func (s *JetStreamBus) AddHandler(aggType, eventType string, handler DomainEventHandler[*entity.BaseEvent]) {
	subject := fmt.Sprintf("%s.*.%s", aggType, eventType)
	s.handlers[subject] = handler

	if s.filterSubjects == nil {
		s.filterSubjects = []string{subject}
	} else {
		if !utils.Contains(s.filterSubjects, subject) {
			s.filterSubjects = append(s.filterSubjects, subject)
		}
	}
}

// DLQCount 查询当前死信队列中的条目数量（管理接口/测试）
func (s *JetStreamBus) DLQCount(ctx context.Context) (int, error) {
	if s.dlqWriter == nil {
		return 0, nil
	}
	stream, err := s.js.Stream(ctx, s.dlqWriter.streamName)
	if err != nil {
		return 0, fmt.Errorf("获取死信流失败: %w", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return 0, fmt.Errorf("获取死信流信息失败: %w", err)
	}
	return int(info.State.Msgs), nil
}

// ReplayDLQ 重放死信队列中的全部条目（按原主题重新发布，供 handler 重新消费）
// 返回成功重放的条目数；重放成功的死信条目将被确认消费，避免重复重放
func (s *JetStreamBus) ReplayDLQ(ctx context.Context) (int, error) {
	if s.dlqWriter == nil {
		return 0, fmt.Errorf("死信队列已禁用")
	}
	stream, err := s.js.Stream(ctx, s.dlqWriter.streamName)
	if err != nil {
		return 0, fmt.Errorf("获取死信流失败: %w", err)
	}
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       s.dlqWriter.streamName + "-Replay", // 消费者名同样不允许点号
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
		FilterSubject: s.dlqWriter.subject,
	})
	if err != nil {
		return 0, fmt.Errorf("创建死信重放消费者失败: %w", err)
	}

	replayed := 0
	for {
		batch, err := consumer.Fetch(100, jetstream.FetchMaxWait(1*time.Second))
		if err != nil {
			return replayed, fmt.Errorf("读取死信消息失败: %w", err)
		}
		fetched := 0
		for msg := range batch.Messages() {
			fetched++
			var entry DLQEntry
			if err := json.Unmarshal(msg.Data(), &entry); err != nil {
				s.logger.Error("死信条目解析失败，跳过重放", "consumer", s.consumerName, "error", err, "subject", msg.Subject())
				msg.Term() // 无法解析的条目终止投递，避免循环
				continue
			}
			// 死信条目中保存的是通用基础事件，直接序列化即还原为流中的原始消息体
			data, err := json.Marshal(entry.Event)
			if err != nil {
				s.logger.Error("死信事件序列化失败，跳过重放", "consumer", s.consumerName, "error", err, "subject", entry.Subject)
				msg.Term()
				continue
			}
			if _, err := s.js.Publish(ctx, entry.Subject, data); err != nil {
				return replayed, fmt.Errorf("重放事件发布失败: %w", err)
			}
			replayed++
			if err := msg.Ack(); err != nil {
				s.logger.Warn("死信条目确认失败", "consumer", s.consumerName, "error", err)
			}
		}
		if err := batch.Error(); err != nil {
			return replayed, fmt.Errorf("读取死信消息失败: %w", err)
		}
		if fetched == 0 {
			break
		}
	}
	s.logger.Info("死信队列重放完成", "stream", s.dlqWriter.streamName, "replayed", replayed)
	return replayed, nil
}
