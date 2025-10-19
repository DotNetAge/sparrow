package messaging

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

type JetStreamReader struct {
	StreamReader
	js          jetstream.JetStream
	agType      string
	logger      *logger.Logger
	serviceName string // 同时用作流名称
}

// NewJetStreamEventReader 创建新的JetStream事件读取器
func NewJetStreamReader(
	js jetstream.JetStream,
	agType string,
	serviceName string, // 同时用作流名称
	logger *logger.Logger,
) StreamReader {
	return &JetStreamReader{
		js:          js,
		agType:      agType,
		logger:      logger,
		serviceName: serviceName,
	}
}

// 获取消费者的通用方法
func (r *JetStreamReader) getConsumer(ctx context.Context, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	stream, err := r.js.Stream(ctx, r.serviceName)
	if err != nil {
		return nil, fmt.Errorf("获取流失败: %w", err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("创建消费者失败: %w", err)
	}

	return consumer, nil
}

// 获取事件的通用方法，支持过滤函数
func (r *JetStreamReader) getEvents(ctx context.Context, aggregateID string, filterFunc func(*entity.BaseEvent) bool) ([]entity.DomainEvent, error) {
	// 设置上下文超时
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 构建消费者配置
	consumerName := fmt.Sprintf("%s-%s-%s", r.serviceName, r.agType, aggregateID)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("%s.%s.*", r.agType, aggregateID),
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	consumer, err := r.getConsumer(ctxWithTimeout, cfg)
	if err != nil {
		return nil, err
	}

	// 收集事件
	var events []entity.DomainEvent

	// 获取消息
	batch, err := consumer.FetchNoWait(1000)
	if err != nil {
		return nil, fmt.Errorf("获取消息失败: %w", err)
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

		// 应用过滤函数
		if filterFunc == nil || filterFunc(eventPtr) {
			events = append(events, eventPtr)
		}

		msg.Ack()
	}

	return events, nil
}

// 应用事件到聚合根的通用方法
func (r *JetStreamReader) applyEventsToAggregate(events []entity.DomainEvent, aggregateID string, aggregate entity.AggregateRoot) error {
	// 按版本升序排序
	sort.Slice(events, func(i, j int) bool {
		return events[i].GetVersion() < events[j].GetVersion()
	})

	// 应用事件到聚合根
	if len(events) > 0 {
		if err := aggregate.LoadFromEvents(events); err != nil {
			if r.logger != nil {
				r.logger.Error("从事件加载聚合根失败", "aggregate_id", aggregateID, "events_count", len(events), "error", err)
			}
			return fmt.Errorf("从事件加载聚合根失败: %w", err)
		}
	}

	return nil
}

// GetEvents 获取聚合根的所有事件（实现EventReader接口）
func (r *JetStreamReader) GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	// 过滤函数：只保留目标聚合ID的事件
	filterFunc := func(event *entity.BaseEvent) bool {
		return event.GetAggregateID() == aggregateID
	}

	return r.getEvents(ctx, aggregateID, filterFunc)

}

// Replay 将事件重放至聚合根（实现EventReader接口）
func (r *JetStreamReader) Replay(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error {
	// 过滤函数：只保留目标聚合ID的事件
	filterFunc := func(event *entity.BaseEvent) bool {
		return event.GetAggregateID() == aggregateID
	}

	events, err := r.getEvents(ctx, aggregateID, filterFunc)
	if err != nil {
		return err
	}

	return r.applyEventsToAggregate(events, aggregateID, aggregate)

}

// ReplayFromVersion 从指定版本开始重放事件（实现EventReader接口）
func (r *JetStreamReader) ReplayFromVersion(ctx context.Context, aggregateID string, fromVersion int, aggregate entity.AggregateRoot) error {
	// 过滤函数：只保留目标聚合ID且版本大于等于fromVersion的事件
	filterFunc := func(event *entity.BaseEvent) bool {
		return event.GetAggregateID() == aggregateID && int(event.GetVersion()) >= fromVersion
	}

	events, err := r.getEvents(ctx, aggregateID, filterFunc)
	if err != nil {
		return err
	}

	return r.applyEventsToAggregate(events, aggregateID, aggregate)

}

// ReplayFromOffset 按事件流的物理偏移量重放（实现StreamEventReader接口）
func (r *JetStreamReader) ReplayFromOffset(ctx context.Context, aggregateID string, offset uint64, aggregate entity.AggregateRoot) error {
	// 设置上下文超时
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 构建消费者配置，从指定偏移量开始
	consumerName := fmt.Sprintf("%s-%s-%s-offset", r.serviceName, r.agType, aggregateID)
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("%s.%s.*", r.agType, aggregateID),
		DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
		OptStartSeq:   offset,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	consumer, err := r.getConsumer(ctxWithTimeout, cfg)
	if err != nil {
		return err
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

	// 按版本升序排序
	sort.Slice(events, func(i, j int) bool {
		return events[i].GetVersion() < events[j].GetVersion()
	})

	// 加载事件到聚合根
	return aggregate.LoadFromEvents(events)
}
