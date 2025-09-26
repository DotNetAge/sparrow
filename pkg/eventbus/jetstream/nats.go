package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// jetStreamBus 实现了eventbus.EventBus接口
// 它使用NATS JetStream来处理事件，提供持久化和可靠的消息传递
type jetStreamBus struct {
	conn       *nats.Conn
	js         jetstream.JetStream
	name       string
	streamName string
	subs       map[string]*nats.Subscription
	subID      int64
	closed     bool
	mu         sync.RWMutex
	logger     *logger.Logger
}

// 确保jetStreamBus实现了EventBus接口
var _ eventbus.EventBus = (*jetStreamBus)(nil)

// NewJetStreamEventBus 创建事件总线实例
func NewJetStreamEventBus(cfg *config.NATsConfig) (eventbus.EventBus, error) {
	// 连接到NATS服务器，配置客户端名称以区分不同连接
	nc, err := nats.Connect(cfg.NATSURL, 
		nats.Name(cfg.StreamName),
		nats.MaxReconnects(-1), // 无限重连
		nats.ReconnectWait(time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	// 创建日志实例
	logConfig := &config.LogConfig{Level: "info", Format: "console"}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	// 创建JetStream上下文
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	// 配置JetStream流 - 使用LimitsPolicy以支持多订阅者模式
	// 每条消息可以被多个消费者消费
	streamConfig := jetstream.StreamConfig{
		Name:      cfg.StreamName,
		Subjects:  []string{"*"}, // 订阅所有主题
		Retention: jetstream.LimitsPolicy, // 使用LimitsPolicy允许多消费者接收相同消息
		Discard:   jetstream.DiscardOld,
		MaxMsgs:   -1, // 不限制消息数量
	}

	// 检查流是否存在，如果不存在则创建
	_, err = js.Stream(context.Background(), cfg.StreamName)
	if err != nil {
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			// 流不存在，创建新流
			_, err = js.CreateStream(context.Background(), streamConfig)
			if err != nil {
				return nil, fmt.Errorf("create stream: %w", err)
			}
		} else {
			// 其他错误
			return nil, fmt.Errorf("check stream: %w", err)
		}
	}

	// 构建实例名称
	instanceName := cfg.StreamName
	if cfg.DurableName != "" {
		instanceName = fmt.Sprintf("%s-%s", instanceName, cfg.DurableName)
	}

	return &jetStreamBus{
	conn:       nc,
	js:         js,
	name:       instanceName,
	streamName: cfg.StreamName,
	subID:      0,
	closed:     false,
	logger:     logger,
	subs:       make(map[string]*nats.Subscription),
	},
	nil
}

// Pub 发布事件到NATS JetStream
func (b *jetStreamBus) Pub(ctx context.Context, evt entity.Event) error {
	b.mu.RLock()
	closed := b.closed
	js := b.js
	b.mu.RUnlock()

	if closed {
		return fmt.Errorf("event bus is closed")
	}

	subject := evt.GetEventType()
	b.logger.Info("Publishing event to JetStream", "subject", subject, "instance", b.name)

	// 序列化事件数据
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// 使用JetStream发布消息，确保消息被持久化
	_, err = js.Publish(ctx, subject, data)
	if err != nil {
		b.logger.Error("Failed to publish event", "error", err, "subject", subject)
		return fmt.Errorf("publish event: %w", err)
	}

	return nil
}

// Sub 订阅指定类型的事件
func (b *jetStreamBus) Sub(eventType string, handler eventbus.EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	subject := eventType
	b.logger.Info("Attempting to subscribe to NATS", "subject", subject, "instance", b.name)

	// 生成唯一的订阅ID
	b.subID++
	subID := fmt.Sprintf("%s-%d", subject, b.subID)

	// 使用标准的NATS订阅而不是JetStream消费者
	sub, err := b.conn.Subscribe(subject, func(msg *nats.Msg) {
		b.logger.Info("Received message from NATS", "subject", subject, "instance", b.name)

		// 解析事件数据
		event := &entity.GenericEvent{}
		if err := json.Unmarshal(msg.Data, event); err != nil {
			b.logger.Error("Failed to unmarshal event data", "error", err)
			return
		}

		// 调用处理器
		if err := handler(context.Background(), event); err != nil {
			b.logger.Error("Failed to handle event", "error", err, "event_type", event.GetEventType())
		} else {
			b.logger.Info("Successfully handled event", "subject", subject, "instance", b.name)
		}
	})

	if err != nil {
		b.logger.Error("Failed to create subscription", "error", err, "subject", subject)
		return fmt.Errorf("create subscription: %w", err)
	}

	// 存储订阅引用
	b.subs[subID] = sub
	b.logger.Info("Successfully subscribed to NATS", "subject", subject, "subID", subID, "instance", b.name)

	return nil
}

// Unsub 取消订阅指定类型的事件
func (b *jetStreamBus) Unsub(eventType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var toDelete []string
	for id := range b.subs {
		// 查找与事件类型相关的所有订阅
		if strings.HasPrefix(id, eventType) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		if sub, ok := b.subs[id]; ok {
			sub.Unsubscribe()
			delete(b.subs, id)
			b.logger.Info("Unsubscribed", "id", id, "subject", eventType)
		}
	}

	return nil
}

// Close 关闭事件总线
func (b *jetStreamBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil // 已经关闭，避免重复操作
	}

	b.closed = true

	// 取消所有订阅
	for _, sub := range b.subs {
		sub.Unsubscribe()
	}
	b.subs = make(map[string]*nats.Subscription)

	// 关闭连接
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
		b.logger.Info("Event bus closed", "instance", b.name)
	}

	return nil
}
