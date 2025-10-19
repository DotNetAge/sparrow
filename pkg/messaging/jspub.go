package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/logger" // 替换为实际日志包路径

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type JetStreamPublisherOption func(*JetStreamPublisher)

// JetStreamPublisher 实现StreamPublisher接口
type JetStreamPublisher struct {
	StreamPublisher
	js          jetstream.JetStream
	serviceName string   // 服务名，同时作为流名称
	aggTypes    []string // 聚合根类型列表，每个类型对应一个主题
	logger      *logger.Logger
	maxAge      time.Duration // 最大消息年龄（秒）
	maxSize     int64         // 最大流大小（字节）
	maxMsgSize  int32         // 最大消息大小（字节）
	debug       bool
}

func WithTopics(aggTypes ...string) JetStreamPublisherOption {
	return func(p *JetStreamPublisher) {
		p.aggTypes = aggTypes
	}
}

// WithMaxAge 设置最大消息年龄
func WithMaxAge(maxAge time.Duration) JetStreamPublisherOption {
	return func(p *JetStreamPublisher) {
		p.maxAge = maxAge
	}
}

// WithMaxSize 设置最大流大小
func WithMaxSize(maxSize int64) JetStreamPublisherOption {
	return func(p *JetStreamPublisher) {
		p.maxSize = maxSize
	}
}

// WithMaxMsgSize 设置最大消息大小
func WithMaxMsgSize(maxMsgSize int32) JetStreamPublisherOption {
	return func(p *JetStreamPublisher) {
		p.maxMsgSize = maxMsgSize
	}
}

// Debug 开启调试模式
func Debug() JetStreamPublisherOption {
	return func(p *JetStreamPublisher) {
		p.debug = true
	}
}

// NewJetStreamPublisher 创建JetStream发布器
// 使用规则:
// 一个服务内只应该有一个发布器实例，每个发布器应负责一个流，一个流内就将该服务的所有聚合根作为主题包含其中。
// 否则很容易出现流与主题不匹配的情况，导致事件无法正确发布到对应的流中。经常出现发布无响应又或主题重叠的情况
// 1. 服务名称作为流的名称
// 2. aggTypes 必须包含当前服务内全部的聚合根类型，否则会导致事件无法正确发布到对应的流中
// 3. 一个服务应该只有一个发布器实例，每个服务只有一个流，流内的主题就是该服务的所有聚合根类型
func NewJetStreamPublisher(
	conn *nats.Conn,
	serviceName string,
	aggTypes []string,
	logger *logger.Logger,
	options ...JetStreamPublisherOption,
) StreamPublisher {
	pub := &JetStreamPublisher{
		serviceName: serviceName,
		aggTypes:    aggTypes,
		logger:      logger,
		maxAge:      0, // 不限制消息的过期时间
		maxSize:     0, // 不限制流的总字节数
		maxMsgSize:  0, // 不限制消息的最大大小
	}

	for _, opt := range options {
		opt(pub)
	}

	// 获取JetStream客户端（使用正确的包和类型）
	js, err := jetstream.New(conn)
	if err != nil {
		pub.logger.Fatal("事件流发布器获取JetStream客户端失败", "stream", pub.serviceName, "error", err)
		panic(err)
	}

	pub.js = js

	// 确保流存在
	if err := pub.ensureStream(context.Background()); err != nil {
		pub.logger.Fatal("事件流发布器无法创建事件流", "stream", pub.serviceName, "error", err)
		panic(err)
	}

	return pub
}

// ensureStream 确保流存在，如果不存在则创建
func (p *JetStreamPublisher) ensureStream(ctx context.Context) error {
	// 使用serviceName作为流名称，简化设计
	streamName := p.serviceName

	// 检查流是否存在
	_, err := p.js.Stream(ctx, streamName)
	if err == nil {
		// 流已存在，直接返回
		return nil
	}

	// 构建主题列表：每个聚合类型对应一个主题，主题模式为：聚合类型.>
	topics := make([]string, 0, len(p.aggTypes))
	for _, aggType := range p.aggTypes {
		topics = append(topics, fmt.Sprintf("%s.>", aggType))
	}

	// 流不存在，创建流
	cfg := jetstream.StreamConfig{
		Name:        streamName,
		Description: fmt.Sprintf("%s service domain events stream", streamName),
		Subjects:    topics,
		Retention:   jetstream.LimitsPolicy, // 使用限制策略而非工作队列策略，适合事件存储
		MaxAge:      p.maxAge,               // 不限制消息的过期时间
		MaxBytes:    p.maxSize,              // 不限制流的总字节数
		MaxMsgSize:  p.maxMsgSize,           // 不限制消息的最大大小
	}

	if p.debug {
		cfg.Storage = jetstream.MemoryStorage
	}

	_, err = p.js.CreateStream(ctx, cfg)
	if err != nil {
		p.logger.Error("[事件流发布器] 创建流失败", "stream", streamName, "error", err)
		return fmt.Errorf("create stream: %w", err)
	}

	p.logger.Info("[事件流发布器] 流创建成功", "stream", streamName)
	return nil
}

// Publish 发布事件（实现StreamPublisher接口）
func (p *JetStreamPublisher) Publish(ctx context.Context, event entity.DomainEvent) error {

	// 构建主题：{聚合类型}.{聚合ID}.{事件类型}
	subject := fmt.Sprintf("%s.%s.%s", event.GetAggregateType(), event.GetAggregateID(), event.GetEventType())

	// 序列化事件
	data, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("[事件流发布器]序列化事件失败", "subject", subject, "aggregateType", event.GetAggregateType(), "eventType", event.GetEventType(), "error", err)
		return fmt.Errorf("marshal event: %w", err)
	}

	// 发布到JetStream
	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		p.logger.Error("[事件流发布器]发布事件失败", "stream", p.serviceName, "subject", subject, "aggregateType", event.GetAggregateType(), "eventType", event.GetEventType(), "error", err)
		return fmt.Errorf("publish event: %w", err)
	}
	return nil
}

func (p *JetStreamPublisher) PublishEvents(ctx context.Context, events []entity.DomainEvent) error {
	for _, event := range events {
		if err := p.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
