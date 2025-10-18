package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/nats-io/nats.go/jetstream"
)

// JetStreamEventReader 基于NATS JetStream实现的事件读取器
// 实现StreamEventReader接口，提供事件流的读取、重放和订阅功能

type JetStreamEventReader struct {
	js          jetstream.JetStream
	agType      string
	logger      *logger.Logger
	serviceName string // 同时用作流名称
}

// NewJetStreamEventReader 创建新的JetStream事件读取器
func NewJetStreamEventReader(
	js jetstream.JetStream,
	agType string,
	serviceName string, // 同时用作流名称
	logger *logger.Logger,
) *JetStreamEventReader {
	return &JetStreamEventReader{
		js:          js,
		agType:      agType,
		logger:      logger,
		serviceName: serviceName,
	}
}

// GetEvents 获取聚合根的所有事件（实现EventReader接口）
func (r *JetStreamEventReader) GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	// 构建消费者配置，只订阅特定聚合ID的事件
	consumerName := fmt.Sprintf("%s-%s-%s", r.serviceName, r.agType, aggregateID)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("%s.%s.*", r.agType, aggregateID), // 订阅所有该聚合类型的事件
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	// 获取流
	stream, err := r.js.Stream(ctx, r.serviceName)
	if err != nil {
		return nil, fmt.Errorf("获取流失败: %w", err)
	}

	// 创建或更新消费者
	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("创建消费者失败: %w", err)
	}

	// 收集事件
	var events []entity.DomainEvent

	// 设置上下文超时
	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 使用FetchAPI获取所有消息
	batch, err := consumer.FetchNoWait(1000)
	if err != nil {
		return nil, fmt.Errorf("获取消息失败: %w", err)
	}

	for msg := range batch.Messages() {
		// 反序列化事件
		var event entity.BaseEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			r.logger.Error("事件反序列化失败", "error", err)
			msg.Ack() // 即使失败也确认，避免阻塞
			continue
		}
		eventPtr := &event

		// 检查是否是目标聚合ID的事件
		if eventPtr.GetAggregateID() == aggregateID {
			events = append(events, eventPtr)
		}

		msg.Ack() // 确认处理完成
	}

	return events, nil
}

// Replay 将事件重放至聚合根（实现EventReader接口）
func (r *JetStreamEventReader) Replay(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error {
	// 直接从JetStream读取特定聚合ID的事件
	consumerName := fmt.Sprintf("%s-%s-%s-replay", r.serviceName, r.agType, aggregateID)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("%s.%s.*", r.agType, aggregateID), // 订阅所有该聚合类型的事件
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	// 获取流
	stream, err := r.js.Stream(ctx, r.serviceName)
	if err != nil {
		return fmt.Errorf("获取流失败: %w", err)
	}

	// 创建或更新消费者
	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("创建消费者失败: %w", err)
	}

	// 收集事件
	var events []entity.DomainEvent

	// 获取消息
	batch, err := consumer.FetchNoWait(1000)
	if err != nil {
		return fmt.Errorf("获取消息失败: %w", err)
	}

	for msg := range batch.Messages() {
		// 反序列化事件
		var event entity.BaseEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			r.logger.Error("事件反序列化失败", "error", err)
			msg.Ack()
			continue
		}
		eventPtr := &event

		// 只保留目标聚合ID的事件
		if eventPtr.GetAggregateID() == aggregateID {
			events = append(events, eventPtr)
		}

		msg.Ack()
	}

	// 按版本升序排序
	sort.Slice(events, func(i, j int) bool {
		return events[i].GetVersion() < events[j].GetVersion()
	})

	// 直接应用事件
	for _, event := range events {
		if err := aggregate.ApplyEvent(event); err != nil {
			return fmt.Errorf("应用事件失败: %w", err)
		}
	}

	return nil
}

// ReplayFromVersion 从指定版本开始重放事件（实现EventReader接口）
func (r *JetStreamEventReader) ReplayFromVersion(ctx context.Context, aggregateID string, fromVersion int, aggregate entity.AggregateRoot) error {
	// 直接从JetStream读取特定聚合ID的事件
	consumerName := fmt.Sprintf("%s-%s-%s-replay-v%d", r.serviceName, r.agType, aggregateID, fromVersion)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("%s.%s.*", r.agType, aggregateID),
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	// 获取流
	stream, err := r.js.Stream(ctx, r.serviceName)
	if err != nil {
		return fmt.Errorf("获取流失败: %w", err)
	}

	// 创建或更新消费者
	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("创建消费者失败: %w", err)
	}

	// 收集事件
	var eventsToReplay []entity.DomainEvent

	// 获取消息
	batch, err := consumer.FetchNoWait(1000)
	if err != nil {
		return fmt.Errorf("获取消息失败: %w", err)
	}

	for msg := range batch.Messages() {
		// 反序列化事件
		var event entity.BaseEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			r.logger.Error("事件反序列化失败", "error", err)
			msg.Ack()
			continue
		}
		eventPtr := &event

		// 只处理目标聚合ID且版本大于等于fromVersion的事件
		if eventPtr.GetAggregateID() == aggregateID && int(eventPtr.GetVersion()) >= fromVersion {
			eventsToReplay = append(eventsToReplay, eventPtr)
		}

		msg.Ack()
	}

	// 按版本升序排序
	sort.Slice(eventsToReplay, func(i, j int) bool {
		return eventsToReplay[i].GetVersion() < eventsToReplay[j].GetVersion()
	})

	// 直接应用事件
	for _, event := range eventsToReplay {
		if err := aggregate.ApplyEvent(event); err != nil {
			return fmt.Errorf("应用事件失败: %w", err)
		}
	}

	return nil
}

// ReplayFromOffset 按事件流的物理偏移量重放（实现StreamEventReader接口）
func (r *JetStreamEventReader) ReplayFromOffset(ctx context.Context, aggregateID string, offset uint64, aggregate entity.AggregateRoot) error {
	// 构建消费者配置，从指定偏移量开始
	consumerName := fmt.Sprintf("%s-%s-%s-offset", r.serviceName, r.agType, aggregateID)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("%s.%s.*", r.agType, aggregateID),
		DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
		OptStartSeq:   offset, // 保持uint64类型
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	// 获取流
	stream, err := r.js.Stream(ctx, r.serviceName)
	if err != nil {
		return fmt.Errorf("获取流失败: %w", err)
	}

	// 创建或更新消费者
	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("创建消费者失败: %w", err)
	}

	// 收集事件
	var events []entity.DomainEvent

	// 获取消息
	batch, err := consumer.FetchNoWait(1000)
	if err != nil {
		return fmt.Errorf("获取消息失败: %w", err)
	}

	for msg := range batch.Messages() {
		// 反序列化事件
		var event entity.BaseEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			r.logger.Error("事件反序列化失败", "error", err)
			msg.Ack()
			continue
		}
		eventPtr := &event

		// 检查是否是目标聚合ID的事件
		if eventPtr.GetAggregateID() == aggregateID {
			events = append(events, eventPtr)
		}

		msg.Ack()
	}

	// 加载事件到聚合根
	return aggregate.LoadFromEvents(events)
}

// Subscribe 订阅聚合根的实时事件（实现StreamEventReader接口）
func (r *JetStreamEventReader) Subscribe(ctx context.Context, aggregateID string, handler func(entity.DomainEvent) error) (func(), error) {
	// 构建消费者配置，用于实时订阅
	consumerName := fmt.Sprintf("%s-%s-%s-live", r.serviceName, r.agType, aggregateID)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("%s.%s.*", r.agType, aggregateID),
		DeliverPolicy: jetstream.DeliverNewPolicy, // 只接收新消息
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	// 获取流
	stream, err := r.js.Stream(ctx, r.serviceName)
	if err != nil {
		return nil, fmt.Errorf("获取流失败: %w", err)
	}

	// 创建或更新消费者
	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("创建消费者失败: %w", err)
	}

	// 创建取消函数
	_, cancel := context.WithCancel(ctx)

	// 启动消费
	consumeChan, err := consumer.Consume(func(msg jetstream.Msg) {
		// 反序列化事件
		var event entity.BaseEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			r.logger.Error("事件反序列化失败", "error", err)
			msg.Nak()
			return
		}
		eventPtr := &event

		// 检查是否是目标聚合ID的事件
		if eventPtr.GetAggregateID() != aggregateID {
			msg.Ack()
			return
		}

		// 调用处理器
		if err = handler(eventPtr); err != nil {
			r.logger.Error("事件处理失败", "error", err, "eventID", eventPtr.GetEventID())
			msg.Nak()
			return
		}

		msg.Ack()
	})

	if err != nil {
		cancel()
		return nil, fmt.Errorf("启动消费失败: %w", err)
	}

	// 返回取消函数
	return func() {
		cancel()
		consumeChan.Stop()
	}, nil
}
