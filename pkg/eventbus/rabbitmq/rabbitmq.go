package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// rabbitBus 实现了eventbus.EventBus接口
// 使用RabbitMQ的发布/订阅功能处理事件
// 2024-10-12: 初始版本
type rabbitBus struct {
	conn            *amqp.Connection
	channel         *amqp.Channel
	exchange        string
	name            string
	subs            map[string][]*amqp.Channel
	handlers        map[string]eventbus.EventHandler
	runningHandlers map[string]bool
	closed          bool
	mu              sync.RWMutex
	logger          *logger.Logger
}

// 确保rabbitBus实现了EventBus接口
var _ eventbus.EventBus = (*rabbitBus)(nil)

// NewRabbitMQEventBus 创建基于RabbitMQ的事件总线实例
func NewRabbitMQEventBus(cfg *config.RabbitMQConfig) (eventbus.EventBus, error) {
	// 构建RabbitMQ连接URL
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.VHost)

	// 创建连接
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("连接到RabbitMQ失败: %w", err)
	}

	// 创建通道
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建RabbitMQ通道失败: %w", err)
	}

	// 创建日志实例
	logConfig := &config.LogConfig{Level: "info", Format: "console"}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("创建日志实例失败: %w", err)
	}

	// 设置默认交换机名称
	exchangeName := cfg.Exchange
	if exchangeName == "" {
		exchangeName = "events"
	}

	// 声明交换机（如果不存在）
	err = ch.ExchangeDeclare(
		exchangeName, // 交换机名称
		"topic",      // 类型：topic支持通配符
		true,         // 持久化
		false,        // 自动删除
		false,        // 内部的
		false,        // 非阻塞
		nil,          // 参数
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("声明交换机失败: %w", err)
	}

	bus := &rabbitBus{
		conn:            conn,
		channel:         ch,
		exchange:        exchangeName,
		name:            "rabbitmq-event-bus",
		subs:            make(map[string][]*amqp.Channel),
		handlers:        make(map[string]eventbus.EventHandler),
		runningHandlers: make(map[string]bool),
		closed:          false,
		logger:          logger,
	}

	bus.logger.Info("RabbitMQ事件总线初始化成功", "exchange", exchangeName)

	return bus, nil
}

// Pub 发布事件到RabbitMQ
func (b *rabbitBus) Pub(ctx context.Context, evt eventbus.Event) error {
	b.mu.RLock()
	closed := b.closed
	channel := b.channel
	exchange := b.exchange
	b.mu.RUnlock()

	if closed {
		return fmt.Errorf("事件总线已关闭")
	}

	subject := evt.EventType
	messageID := evt.Id
	createdAt := evt.Timestamp

	b.logger.Info("正在将事件发布到RabbitMQ", "subject", subject, "message_id", messageID)

	// 序列化事件数据
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	// 发布消息
	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
		Timestamp:   time.Now(),
		MessageId:   messageID,
		Headers: amqp.Table{
			"event_type": subject,
			"created_at": createdAt.Format(time.RFC3339),
		},
	}

	if err := channel.PublishWithContext(ctx, exchange, subject, false, false, msg); err != nil {
		b.logger.Error("发布事件失败", "error", err, "subject", subject)
		return fmt.Errorf("发布事件到RabbitMQ失败: %w", err)
	}

	b.logger.Info("事件发布成功", "subject", subject)
	return nil
}

// Sub 订阅指定类型的事件
func (b *rabbitBus) Sub(eventType string, handler eventbus.EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("事件总线已关闭")
	}

	subject := eventType
	b.logger.Info("尝试订阅RabbitMQ事件", "subject", subject)

	// 存储处理器
	b.handlers[subject] = handler

	// 创建新的通道用于订阅
	subCh, err := b.conn.Channel()
	if err != nil {
		b.logger.Error("创建订阅通道失败", "error", err, "subject", subject)
		return fmt.Errorf("创建订阅通道失败: %w", err)
	}

	// 声明队列（临时队列）
	queue, err := subCh.QueueDeclare(
		"",    // 空名称表示由RabbitMQ生成唯一名称
		false, // 非持久化
		true,  // 自动删除
		false, // 排他性
		false, // 非阻塞
		nil,   // 参数
	)
	if err != nil {
		subCh.Close()
		b.logger.Error("声明队列失败", "error", err, "subject", subject)
		return fmt.Errorf("声明队列失败: %w", err)
	}

	// 将队列绑定到交换机
	if err := subCh.QueueBind(
		queue.Name, // 队列名称
		subject,    // 路由键
		b.exchange, // 交换机
		false,      // 非阻塞
		nil,        // 参数
	); err != nil {
		subCh.Close()
		b.logger.Error("绑定队列失败", "error", err, "subject", subject)
		return fmt.Errorf("绑定队列失败: %w", err)
	}

	// 消费消息
	msgs, err := subCh.Consume(
		queue.Name, // 队列名称
		"",         // 消费者标签
		true,       // 自动确认
		false,      // 非排他性
		false,      // 不本地
		false,      // 非阻塞
		nil,        // 参数
	)
	if err != nil {
		subCh.Close()
		b.logger.Error("创建消费者失败", "error", err, "subject", subject)
		return fmt.Errorf("创建消费者失败: %w", err)
	}

	// 存储订阅引用
	if _, exists := b.subs[subject]; !exists {
		b.subs[subject] = []*amqp.Channel{}
	}
	b.subs[subject] = append(b.subs[subject], subCh)

	// 启动异步消息处理
	go b.handleMessages(subject, handler, msgs)

	b.logger.Info("成功订阅RabbitMQ事件", "subject", subject)

	return nil
}

// handleMessages 处理从RabbitMQ接收的消息
func (b *rabbitBus) handleMessages(subject string, handler eventbus.EventHandler, msgs <-chan amqp.Delivery) {
	// 标记此处理器正在运行
	b.mu.Lock()
	processorKey := fmt.Sprintf("%s-handler", subject)
	b.runningHandlers[processorKey] = true
	b.mu.Unlock()

	// 确保在函数退出时标记为不运行
	defer func() {
		b.mu.Lock()
		delete(b.runningHandlers, processorKey)
		b.mu.Unlock()
	}()

	// 处理接收到的消息
	for d := range msgs {
		// 检查事件总线是否已关闭
		b.mu.RLock()
		if b.closed {
			b.mu.RUnlock()
			break
		}
		b.mu.RUnlock()

		b.logger.Info("接收到RabbitMQ消息", "subject", subject, "message_id", d.MessageId)

		// 解析事件类型
		eventType := subject
		if eventTypeFromHeader, ok := d.Headers["event_type"].(string); ok {
			eventType = eventTypeFromHeader
		}

		// 反序列化事件数据
		var payload map[string]interface{}
		if err := json.Unmarshal(d.Body, &payload); err != nil {
			b.logger.Error("反序列化事件数据失败", "error", err, "subject", subject)
			continue
		}

		// 创建上下文并调用处理器
		ctx := context.Background()
		// 使用 eventbus.Event 类型
		event := eventbus.Event{
			Id:        d.MessageId,
			EventType: eventType,
			Timestamp: d.Timestamp,
			Payload:   payload,
		}

		// 调用处理器
		if err := handler(ctx, event); err != nil {
			b.logger.Error("处理事件失败", "error", err, "subject", subject)
		} else {
			b.logger.Info("成功处理事件", "subject", subject)
		}
	}

	b.logger.Info("消息通道已关闭", "subject", subject)
}

// Unsub 取消订阅指定类型的事件
func (b *rabbitBus) Unsub(eventType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	subject := eventType
	b.logger.Info("正在取消RabbitMQ订阅", "subject", subject)

	// 关闭并移除所有相关订阅
	if channels, exists := b.subs[subject]; exists {
		for _, ch := range channels {
			ch.Close()
		}
		delete(b.subs, subject)
	}

	// 移除处理器
	if _, exists := b.handlers[subject]; exists {
		delete(b.handlers, subject)
	}

	b.logger.Info("成功取消RabbitMQ订阅", "subject", subject)

	return nil
}

// Close 关闭事件总线，释放资源
func (b *rabbitBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.logger.Info("正在关闭RabbitMQ事件总线")

	// 关闭所有订阅通道
	for _, channels := range b.subs {
		for _, ch := range channels {
			ch.Close()
		}
	}

	// 关闭主通道
	if b.channel != nil {
		b.channel.Close()
	}

	// 关闭连接
	if b.conn != nil {
		b.conn.Close()
	}

	// 清理状态
	b.closed = true
	b.subs = make(map[string][]*amqp.Channel)
	b.handlers = make(map[string]eventbus.EventHandler)
	b.runningHandlers = make(map[string]bool)

	b.logger.Info("RabbitMQ事件总线已关闭")

	return nil
}
