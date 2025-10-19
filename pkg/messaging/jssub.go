package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/sparrow/pkg/logger" // 替换为实际日志包路径
	"github.com/DotNetAge/sparrow/pkg/utils"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// JetStreamSubscriber 泛型订阅器（绑定单一事件类型T）
// 结构体带泛型参数，方法继承泛型约束
type JetStreamSubscriber[T DomainEventConstraint] struct {
	StreamSubscriber
	js          jetstream.JetStream
	serviceName string // 服务名，同时作为流名称
	aggType     string // 聚合类型
	logger      *logger.Logger
	eventType   string                // 事件类型
	handler     DomainEventHandler[T] // 该类型事件的处理器
	consumer    jetstream.Consumer    // JetStream消费者实例
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
	}
}

// Start 启动订阅（实现StreamSubscriber接口）
func (s *JetStreamSubscriber[T]) Start(ctx context.Context) error {
	// 1. 获取事件类型（通过T的空实例）
	// var event T
	// eventType := event.GetEventType() // 使用正确的方式获取事件类型
	// 构建主题：聚合类型.聚合ID.事件类型
	// 2. 配置消费者 - 适配LimitsPolicy的事件存储策略
	consumerName := fmt.Sprintf("%s-%s-%s", s.serviceName, s.aggType, s.eventType)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,                                   // 持久化消费者（重启不丢失进度）
		AckPolicy:     jetstream.AckExplicitPolicy,                    // 保持显式确认以确保可靠处理
		FilterSubject: fmt.Sprintf("%s.*.%s", s.aggType, s.eventType), // 只订阅当前事件类型
		DeliverPolicy: jetstream.DeliverAllPolicy,                     // 从最早的消息开始投递，支持事件重放
		ReplayPolicy:  jetstream.ReplayInstantPolicy,                  // 立即重放所有消息
	}

	// 3. 获取流并创建消费者 - 使用serviceName作为流名称
	stream, err := s.js.Stream(ctx, s.serviceName)
	if err != nil {
		s.logger.Fatal("获取流失败", "consumer", consumerName, "stream", s.serviceName, "error", err)
		return fmt.Errorf("获取流失败: %w", err)
	}
	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		s.logger.Fatal("创建消费者失败", "consumer", consumerName, "stream", s.serviceName, "error", err)
		return fmt.Errorf("创建消费者失败: %w", err)
	}
	s.consumer = consumer

	// 4. 开始消费消息
	_, err = consumer.Consume(s.handleMessage) // 绑定消息处理函数，不需要传递ctx
	if err != nil {
		s.logger.Fatal("启动消费者失败", "consumer", consumerName, "stream", s.serviceName, "error", err)
		return fmt.Errorf("启动消费者失败: %w", err)
	}

	s.logger.Info("订阅器启动成功", "consumer", consumerName)
	return nil
}

// Stop 停止订阅（实现StreamSubscriber接口）
func (s *JetStreamSubscriber[T]) Stop(ctx context.Context) error {
	if s.consumer != nil {
		// NATS JetStream消费者不需要显式删除
		s.logger.Info("消费者已停止", "consumer", s.consumer)
	}
	s.logger.Info("订阅器已停止")
	return nil
}

// handleMessage 处理单条消息（内部方法，继承结构体泛型）
func (s *JetStreamSubscriber[T]) handleMessage(msg jetstream.Msg) {
	ctx := context.Background()

	// 1. 反序列化为T类型事件
	var event T
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		s.logger.Error("消息反序列化失败", "consumer", s.consumer, "error", err, "subject", msg.Subject())
		msg.Nak() // 标记处理失败
		return
	}

	// 2. 调用类型安全的处理器
	if err := s.handler(ctx, event); err != nil {
		s.logger.Error("事件处理失败", "consumer", s.consumer, "error", err, "eventID", event.GetEventID())
		msg.Nak() // 失败重试
		return
	}

	// 3. 确认处理完成
	if err := msg.Ack(); err != nil {
		s.logger.Warn("消息确认失败", "consumer", s.consumer, "error", err)
	}
}
