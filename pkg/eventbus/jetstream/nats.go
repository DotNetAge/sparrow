package jetstream

import (
	"context"
	"encoding/json"
	"fmt"
	"purchase-service/pkg/adapter/event/handlers"
	"purchase-service/pkg/config"
	"purchase-service/pkg/entity"
	"purchase-service/pkg/eventbus"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// jetStreamBus 基于NATS JetStream的事件总线
// 提供可靠的事件发布和订阅机制
// 支持持久化订阅和消费者组
type jetStreamBus struct {
	js     nats.JetStreamContext
	name   string
	subs   map[string]*nats.Subscription
	mu     sync.RWMutex
	logger *zap.Logger
}

// 确保jetStreamBus实现了EventBus接口
var _ eventbus.EventBus = (*jetStreamBus)(nil)

// NewEventBus 创建事件总线实例
func NewJetStreamEventBus(cfg *config.NATsConfig) (eventbus.EventBus, error) {
	// 连接到NATS服务器
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	// 获取JetStream上下文
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("get jetstream context: %w", err)
	}

	// 创建zap logger
	logger, _ := zap.NewProduction()

	return &jetStreamBus{
		js:     js,
		name:   cfg.StreamName,
		subs:   make(map[string]*nats.Subscription),
		logger: logger,
	}, nil
}

// Pub 发布事件到JetStream
// 根据用户的设计要求，EventBus只负责发布一般性事件，不需要知道领域事件的细节
// 事件的主题由调用方（如EventPublisher）通过evt.GetEventType()提供
func (b *jetStreamBus) Pub(ctx context.Context, evt entity.Event) error {
	// 根据EventBus的设计职责，我们直接使用事件提供的EventType作为主题
	// 不再尝试判断事件类型或修改主题格式
	subject := evt.GetEventType()

	// 使用标准JSON序列化
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
	}

	_, err = b.js.PublishMsg(msg)
	return err
}

// Subscribe 订阅事件
// handler: 事件处理器函数
func (b *jetStreamBus) Sub(eventType string, handler handlers.EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	subject := fmt.Sprintf("%s.*.%s", b.name, eventType)
	consumerName := fmt.Sprintf("%s-%s", b.name, eventType)

	// 创建订阅
	sub, err := b.js.Subscribe(subject, func(msg *nats.Msg) {
		// 由于我们无法直接反序列化到接口类型，这里创建一个通用的map来存储事件数据
		var eventData map[string]interface{}
		if err := json.Unmarshal(msg.Data, &eventData); err != nil {
			if err := msg.Nak(); err != nil {
				b.logger.Error("Failed to Nak message", zap.Error(err))
			}
			return
		}

		// 从事件数据中提取必要信息
		// 注意：这里我们不尝试重建原始的事件对象，因为这需要知道具体的事件类型
		// 在实际使用中，事件处理器应该知道如何处理这种通用的事件数据
		ctx := context.Background()
		if err := handler(ctx, &entity.GenericEvent{
			Id:        eventData["id"].(string),
			EventType: eventData["event_type"].(string),
			Timestamp: time.Now(), // 在实际实现中，应该从eventData中解析时间
			Payload:   eventData,
		}); err != nil {
			if err := msg.Nak(); err != nil {
				b.logger.Error("Failed to Nak message", zap.Error(err))
			}
			return
		}

		if err := msg.Ack(); err != nil {
			b.logger.Error("Failed to Ack message", zap.Error(err))
		}
	}, nats.Durable(consumerName))

	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	b.subs[consumerName] = sub
	return nil
}

// Unsub 取消订阅指定类型的事件
func (b *jetStreamBus) Unsub(eventType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 构建与Subscribe时相同的consumerName
	consumerName := fmt.Sprintf("%s-%s", b.name, eventType)

	// 查找并取消订阅
	if sub, exists := b.subs[consumerName]; exists {
		if err := sub.Unsubscribe(); err != nil {
			return fmt.Errorf("unsubscribe failed: %w", err)
		}
		delete(b.subs, consumerName)
	}

	return nil
}

// Close 关闭所有订阅
func (b *jetStreamBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var errs []error
	for name, sub := range b.subs {
		if err := sub.Unsubscribe(); err != nil {
			errs = append(errs, fmt.Errorf("failed to unsubscribe from %s: %w", name, err))
		}
		delete(b.subs, name)
	}
	if len(errs) > 0 {
		return fmt.Errorf("close event bus: %v", errs)
	}
	return nil
}
