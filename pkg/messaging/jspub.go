package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/logger" // 替换为实际日志包路径

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/nats-io/nats.go/jetstream"
)

type JetStreamPublisherOption func(*JetStreamPublisher)

// JetStreamPublisher 实现StreamPublisher接口
type JetStreamPublisher struct {
	js          jetstream.JetStream
	serviceName string // 服务名，同时作为流名称
	aggType     string
	logger      *logger.Logger
	maxAge      time.Duration // 最大消息年龄（秒）
	maxSize     int64         // 最大流大小（字节）
	maxMsgSize  int32         // 最大消息大小（字节）
	debug       bool
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
func NewJetStreamPublisher(
	js jetstream.JetStream,
	serviceName string,
	aggType string,
	logger *logger.Logger,
	options ...JetStreamPublisherOption,
) *JetStreamPublisher {
	pub := &JetStreamPublisher{
		js:          js,
		serviceName: serviceName,
		aggType:     aggType,
		logger:      logger,
		maxAge:      0, // 不限制消息的过期时间
		maxSize:     0, // 不限制流的总字节数
		maxMsgSize:  0, // 不限制消息的最大大小
	}

	for _, opt := range options {
		opt(pub)
	}

	// 确保流存在
	if err := pub.ensureStream(context.Background()); err != nil {
		pub.logger.Fatal("确保流存在失败", "stream", pub.serviceName, "error", err)
		panic(err)
	}

	return pub
}

// func (p *JetStreamPublisher) Use(options ...JetStreamPublisherOption) {
// 	for _, opt := range options {
// 		opt(p)
// 	}
// }

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

	// 流不存在，创建流
	cfg := jetstream.StreamConfig{
		Name:        streamName,
		Description: fmt.Sprintf("%s service domain events stream", streamName),
		Subjects:    []string{fmt.Sprintf("%s.>", p.aggType)},
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
		p.logger.Error("创建流失败", "stream", streamName, "error", err)
		return fmt.Errorf("create stream: %w", err)
	}

	p.logger.Info("流创建成功", "stream", streamName)
	return nil
}

// Publish 发布事件（实现StreamPublisher接口）
func (p *JetStreamPublisher) Publish(ctx context.Context, event entity.DomainEvent) error {

	// 构建主题：聚合类型.聚合ID.事件类型
	subject := fmt.Sprintf("%s.%s.%s", p.aggType, event.GetAggregateID(), event.GetEventType())

	// 序列化事件
	data, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("事件序列化失败", "error", err)
		return fmt.Errorf("marshal event: %w", err)
	}

	// 发布到JetStream
	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		p.logger.Error("发布事件失败", "subject", subject, "error", err)
		return fmt.Errorf("publish event: %w", err)
	}
	return nil
}
