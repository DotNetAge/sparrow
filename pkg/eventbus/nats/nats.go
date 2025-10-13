package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	n "github.com/nats-io/nats.go"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// 基于NATS实现标准事件总线，没有使用JetStream因此不支持持久化和可靠的消息传递
type natsBus struct {
	conn *n.Conn
	// js         jetstream.JetStream
	name       string
	streamName string
	subs       map[string]*n.Subscription
	subID      int64
	closed     bool
	mu         sync.RWMutex
	logger     *logger.Logger
}

// 确保natsBus实现了EventBus接口
var _ eventbus.EventBus = (*natsBus)(nil)

// NewNatsEventBus 创建事件总线实例
func NewNatsEventBus(cfg *config.NATsConfig) (eventbus.EventBus, error) {
	// 连接到NATS服务器，配置客户端名称以区分不同连接
	nc, err := n.Connect(cfg.NATSURL,
		n.Name(cfg.StreamName),
		n.MaxReconnects(-1), // 无限重连
		n.ReconnectWait(time.Second))
	if err != nil {
		return nil, fmt.Errorf("连接NATS失败: %w", err)
	}

	// 创建日志实例
	logConfig := &config.LogConfig{Level: "info", Format: "console"}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		return nil, fmt.Errorf("创建日志实例失败: %w", err)
	}

	// // 创建JetStream上下文
	// js, err := jetstream.New(nc)
	// if err != nil {
	// 	return nil, fmt.Errorf("创建JetStream上下文失败: %w", err)
	// }

	// // 检查流是否存在
	// _, err = js.Stream(context.Background(), cfg.StreamName)
	// if err != nil {
	// 	if errors.Is(err, jetstream.ErrStreamNotFound) {
	// 		// 流不存在，创建新流
	// 		// 配置JetStream流 - 使用LimitsPolicy以支持多订阅者模式
	// 		// 每条消息可以被多个消费者消费
	// 		streamConfig := jetstream.StreamConfig{
	// 			Name:      cfg.StreamName,
	// 			Subjects:  []string{"test.>", "event.>", "test-service.>"}, // 正确添加通配符'>'
	// 			Retention: jetstream.LimitsPolicy, // 使用LimitsPolicy允许多消费者接收相同消息
	// 			Discard:   jetstream.DiscardOld,
	// 			MaxMsgs:   -1, // 不限制消息数量
	// 		}

	// 		_, err = js.CreateStream(context.Background(), streamConfig)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("创建流失败: %w", err)
	// 		}
	// 	} else {
	// 		// 其他错误
	// 		return nil, fmt.Errorf("检查流失败: %w", err)
	// 	}
	// }

	// 构建实例名称
	instanceName := cfg.StreamName
	if cfg.DurableName != "" {
		instanceName = fmt.Sprintf("%s-%s", instanceName, cfg.DurableName)
	}

	return &natsBus{
			conn: nc,
			// js:         js,
			name:       instanceName,
			streamName: cfg.StreamName,
			subID:      0,
			closed:     false,
			logger:     logger,
			subs:       make(map[string]*n.Subscription),
		},
		nil
}

// Pub 发布事件到NATS JetStream
func (b *natsBus) Pub(ctx context.Context, evt eventbus.Event) error {
	b.mu.RLock()
	closed := b.closed
	conn := b.conn
	b.mu.RUnlock()

	if closed {
		return fmt.Errorf("事件总线已关闭")
	}

	subject := evt.EventType
	messageID := evt.Id

	b.logger.Info("发布事件到NATS", "subject", subject, "message_id", messageID, "instance", b.name)

	// 序列化整个eventbus.Event对象
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("序列化事件数据失败: %w", err)
	}

	// 使用标准NATS发布（为了测试兼容性）
	if err := conn.Publish(subject, data); err != nil {
		b.logger.Error("发布事件到NATS失败", "error", err, "subject", subject)
		return fmt.Errorf("发布事件到NATS失败: %w", err)
	}

	b.logger.Info("成功发布事件到NATS", "subject", subject, "instance", b.name)
	return nil
}

// Sub 订阅指定类型的事件
func (b *natsBus) Sub(eventType string, handler eventbus.EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("事件总线已关闭")
	}

	subject := eventType
	b.logger.Info("尝试订阅NATS", "subject", subject, "instance", b.name)

	// 生成唯一的订阅ID
	b.subID++
	subID := fmt.Sprintf("%s-%d", subject, b.subID)

	// 使用标准的NATS订阅而不是JetStream消费者
	sub, err := b.conn.Subscribe(subject, func(msg *n.Msg) {
		b.logger.Info("收到NATS消息", "subject", subject, "instance", b.name)

		// 解析事件数据
		var event eventbus.Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			b.logger.Error("解析事件数据失败", "error", err)
			return
		}

		// 调用处理器
		if err := handler(context.Background(), event); err != nil {
			b.logger.Error("处理事件失败", "error", err, "event_type", event.EventType)
		} else {
			b.logger.Info("成功处理事件", "subject", subject, "instance", b.name)
		}
	})

	if err != nil {
		b.logger.Error("创建订阅失败", "error", err, "subject", subject)
		return fmt.Errorf("创建订阅失败: %w", err)
	}

	// 存储订阅引用
	b.subs[subID] = sub
	b.logger.Info("成功订阅NATS", "subject", subject, "subID", subID, "instance", b.name)

	return nil
}

// Unsub 取消订阅指定类型的事件
func (b *natsBus) Unsub(eventType string) error {
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
			b.logger.Info("取消订阅NATS", "id", id, "subject", eventType)
		}
	}

	return nil
}

// Close 关闭事件总线
func (b *natsBus) Close() error {
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
	b.subs = make(map[string]*n.Subscription)

	// 关闭连接
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
		b.logger.Info("关闭NATS事件总线", "instance", b.name)
	}

	return nil
}
