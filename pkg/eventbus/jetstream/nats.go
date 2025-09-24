package jetstream

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/adapter/event/handlers"
	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"

	"github.com/nats-io/nats.go"
)

// jetStreamBus 基于NATS JetStream的事件总线
// 提供可靠的事件发布和订阅机制
// 支持持久化订阅和消费者组
type jetStreamBus struct {
	js     nats.JetStreamContext
	name   string
	subs   map[string]*nats.Subscription
	mu     sync.RWMutex
	logger *logger.Logger
}

// 确保jetStreamBus实现了EventBus接口
var _ eventbus.EventBus = (*jetStreamBus)(nil)

// NewJetStreamEventBus 创建事件总线实例
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

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "info",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

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
				b.logger.Error("Failed to Nak message", "error", err)
			}
			return
		}

		// 从事件数据中提取必要信息
		// 安全地进行类型断言
		id, ok := eventData["id"].(string)
		if !ok {
			b.logger.Error("Invalid event ID format")
			if err := msg.Nak(); err != nil {
				b.logger.Error("Failed to Nak message", "error", err)
			}
			return
		}

		eventType, ok := eventData["event_type"].(string)
		if !ok {
			b.logger.Error("Invalid event type format")
			if err := msg.Nak(); err != nil {
				b.logger.Error("Failed to Nak message", "error", err)
			}
			return
		}

		// 从事件数据中解析时间戳
		timestamp := time.Now()
		if tsStr, ok := eventData["timestamp"].(string); ok {
			if ts, err := time.Parse(time.RFC3339, tsStr); err == nil {
				timestamp = ts
			} else if ts, ok := eventData["timestamp"].(float64); ok {
				timestamp = time.Unix(int64(ts), 0)
			}
		}

		// 创建GenericEvent对象
		event := &entity.GenericEvent{
			Id:        id,
			EventType: eventType,
			Timestamp: timestamp,
			Payload:   eventData,
		}

		// 处理事件
		ctx := context.Background()
		if err := handler(ctx, event); err != nil {
			b.logger.Error("Failed to handle event", "error", err, "event_type", eventType)
			if err := msg.Nak(); err != nil {
				b.logger.Error("Failed to Nak message", "error", err)
			}
			return
		}

		// 确认消息已处理
		if err := msg.Ack(); err != nil {
			b.logger.Error("Failed to Ack message", "error", err)
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

	var lastErr error
	for name, sub := range b.subs {
		if err := sub.Unsubscribe(); err != nil {
			b.logger.Error("Failed to unsubscribe", "name", name, "error", err)
			lastErr = err // 记录最后一个错误，但继续尝试关闭其他订阅
		}
		delete(b.subs, name)
	}
	return lastErr
}
