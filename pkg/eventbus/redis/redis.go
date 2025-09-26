package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/go-redis/redis/v8"
)

// redisBus 实现了eventbus.EventBus接口
// 使用Redis的发布/订阅功能处理事件
// 2024-09-26: 初始版本

type redisBus struct {
	client    *redis.Client
	name      string
	subs      map[string][]*redis.PubSub
	handlers  map[string]eventbus.EventHandler
	subID     int64
	closed    bool
	mu        sync.RWMutex
	logger    *logger.Logger
}

// 确保redisBus实现了EventBus接口
var _ eventbus.EventBus = (*redisBus)(nil)

// NewRedisEventBus 创建基于Redis的事件总线实例
func NewRedisEventBus(cfg *config.RedisConfig) (eventbus.EventBus, error) {
	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect to redis: %w", err)
	}

	// 创建日志实例
	logConfig := &config.LogConfig{Level: "info", Format: "console"}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("create logger: %w", err)
	}

	return &redisBus{
	client:   client,
	name:     "redis-event-bus",
	subs:     make(map[string][]*redis.PubSub),
	handlers: make(map[string]eventbus.EventHandler),
	subID:    0,
	closed:   false,
	logger:   logger,
	},
	nil
}

// Pub 发布事件到Redis频道
func (b *redisBus) Pub(ctx context.Context, evt entity.Event) error {
	b.mu.RLock()
	closed := b.closed
	client := b.client
	b.mu.RUnlock()

	if closed {
		return fmt.Errorf("event bus is closed")
	}

	subject := evt.GetEventType()
	b.logger.Info("正在将事件发布到Redis", "subject", subject, "instance", b.name)

	// 序列化事件数据
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// 发布到Redis频道
	if err := client.Publish(ctx, subject, data).Err(); err != nil {
		b.logger.Error("发布事件失败", "error", err, "subject", subject)
		return fmt.Errorf("publish event: %w", err)
	}

	return nil
}

// Sub 订阅指定类型的事件
func (b *redisBus) Sub(eventType string, handler eventbus.EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	subject := eventType
	b.logger.Info("尝试订阅Redis", "subject", subject, "instance", b.name)

	// 存储处理器
	b.handlers[subject] = handler

	// 创建新的PubSub客户端
	ps := b.client.Subscribe(context.Background(), subject)

	// 确保订阅成功
	if _, err := ps.ReceiveTimeout(context.Background(), 5*time.Second); err != nil {
		ps.Close()
		b.logger.Error("订阅失败", "error", err, "subject", subject)
		return fmt.Errorf("subscribe to redis: %w", err)
	}

	// 存储订阅引用
	if _, exists := b.subs[subject]; !exists {
		b.subs[subject] = []*redis.PubSub{}
	}
	b.subs[subject] = append(b.subs[subject], ps)

	// 启动异步消息处理
	go b.handleMessages(ps, subject, handler)

	b.logger.Info("成功订阅Redis", "subject", subject, "instance", b.name)

	return nil
}

// Unsub 取消订阅指定类型的事件
func (b *redisBus) Unsub(eventType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	subject := eventType
	b.logger.Info("正在取消Redis订阅", "subject", subject, "instance", b.name)

	// 关闭并移除所有相关订阅
	if pubs, exists := b.subs[subject]; exists {
		for _, pub := range pubs {
			pub.Close()
		}
		delete(b.subs, subject)
	}

	// 移除处理器
	delete(b.handlers, subject)

	return nil
}

// Close 关闭事件总线
func (b *redisBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil // 已经关闭，避免重复操作
	}

	b.closed = true

	// 关闭所有订阅
	for subject, pubs := range b.subs {
		for _, pub := range pubs {
			pub.Close()
		}
		delete(b.subs, subject)
		b.logger.Info("关闭过程中取消订阅", "subject", subject)
	}

	// 清空处理器
	b.handlers = make(map[string]eventbus.EventHandler)

	// 关闭Redis连接
	if b.client != nil {
		if err := b.client.Close(); err != nil {
			b.logger.Error("关闭Redis连接失败", "error", err)
			return err
		}
		b.client = nil
		b.logger.Info("事件总线已关闭", "instance", b.name)
	}

	return nil
}

// handleMessages 处理从Redis接收到的消息
func (b *redisBus) handleMessages(ps *redis.PubSub, subject string, handler eventbus.EventHandler) {
	ch := ps.Channel()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				// 通道已关闭，退出循环
				b.logger.Info("消息通道已关闭", "subject", subject)
				return
			}

			b.logger.Info("从Redis接收消息", "subject", subject, "instance", b.name)

			// 解析事件数据
			event := &entity.GenericEvent{}
			if err := json.Unmarshal([]byte(msg.Payload), event); err != nil {
				b.logger.Error("解析事件数据失败", "error", err, "subject", subject)
				continue
			}

			// 调用处理器
			if err := handler(context.Background(), event); err != nil {
				b.logger.Error("处理事件失败", "error", err, "event_type", event.GetEventType())
			} else {
				b.logger.Info("成功处理事件", "subject", subject, "instance", b.name)
			}

		case <-time.After(1 * time.Second):
			// 定期检查是否已关闭
			b.mu.RLock()
			closed := b.closed
			b.mu.RUnlock()
			if closed {
				return
			}
		}
	}
}