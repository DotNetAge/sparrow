package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// kafkaBus 实现了eventbus.EventBus接口
// 使用Kafka的发布/订阅功能处理事件
// 2024-10-12: 初始版本
type kafkaBus struct {
	producer      sarama.SyncProducer
	consumers     map[string]sarama.Consumer
	handlers      map[string]eventbus.EventHandler
	config        *config.KafkaConfig
	closed        bool
	mu            sync.RWMutex
	logger        *logger.Logger
}

// 确保kafkaBus实现了EventBus接口
var _ eventbus.EventBus = (*kafkaBus)(nil)

// NewKafkaEventBus 创建基于Kafka的事件总线实例
func NewKafkaEventBus(cfg *config.KafkaConfig) (eventbus.EventBus, error) {
	// 创建日志实例
	logConfig := &config.LogConfig{Level: "info", Format: "console"}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		return nil, fmt.Errorf("创建日志实例失败: %w", err)
	}

	// 配置Kafka生产者
	producerConfig := sarama.NewConfig()
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.RequiredAcks = sarama.WaitForLocal
	producerConfig.Producer.Retry.Max = 5
	producerConfig.Producer.Flush.Frequency = 100 * time.Millisecond

	// 创建同步生产者
	producer, err := sarama.NewSyncProducer(cfg.Brokers, producerConfig)
	if err != nil {
		return nil, fmt.Errorf("创建Kafka生产者失败: %w", err)
	}

	// 使用默认主题名
	topic := cfg.Topic
	if topic == "" {
		topic = "events"
	}

	bus := &kafkaBus{
		producer:  producer,
		consumers: make(map[string]sarama.Consumer),
		handlers:  make(map[string]eventbus.EventHandler),
		config:    cfg,
		closed:    false,
		logger:    logger,
	}

	bus.logger.Info("Kafka事件总线初始化成功", "brokers", cfg.Brokers, "topic", topic)

	return bus, nil
}

// Pub 发布事件到Kafka
func (b *kafkaBus) Pub(ctx context.Context, evt entity.Event) error {
	b.mu.RLock()
	closed := b.closed
	producer := b.producer
	topic := b.config.Topic
	if topic == "" {
		topic = "events"
	}
	b.mu.RUnlock()

	if closed {
		return fmt.Errorf("事件总线已关闭")
	}

	subject := evt.GetEventType()
	messageID := evt.GetEventID()
	createdAt := evt.GetCreatedAt()

	b.logger.Info("正在将事件发布到Kafka", "subject", subject, "message_id", messageID, "topic", topic)

	// 序列化事件数据
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	// 创建消息
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(messageID),
		Value: sarama.StringEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("event_type"), Value: []byte(subject)},
			{Key: []byte("created_at"), Value: []byte(createdAt.Format(time.RFC3339))},
		},
	}

	// 发布消息
	partition, offset, err := producer.SendMessage(msg)
	if err != nil {
		b.logger.Error("发布事件失败", "error", err, "subject", subject)
		return fmt.Errorf("发布事件到Kafka失败: %w", err)
	}

	b.logger.Info("成功发布事件到Kafka", "subject", subject, "partition", partition, "offset", offset)

	return nil
}

// Sub 订阅指定类型的事件
func (b *kafkaBus) Sub(eventType string, handler eventbus.EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("事件总线已关闭")
	}

	subject := eventType
	b.logger.Info("尝试订阅Kafka事件", "subject", subject)

	// 存储处理器
	b.handlers[subject] = handler

	// 配置Kafka消费者
	consumerConfig := sarama.NewConfig()
	consumerConfig.Version = sarama.V2_8_0_0
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetNewest

	// 创建消费者
	consumer, err := sarama.NewConsumer(b.config.Brokers, consumerConfig)
	if err != nil {
		b.logger.Error("创建消费者失败", "error", err, "subject", subject)
		return fmt.Errorf("创建Kafka消费者失败: %w", err)
	}

	// 存储消费者引用
	b.consumers[subject] = consumer

	// 启动异步消息处理
	go b.handleMessages(subject, handler)

	b.logger.Info("成功订阅Kafka事件", "subject", subject)

	return nil
}

// handleMessages 处理从Kafka接收的消息
func (b *kafkaBus) handleMessages(subject string, handler eventbus.EventHandler) {
	b.mu.RLock()
	consumer := b.consumers[subject]
	topic := b.config.Topic
	if topic == "" {
		topic = "events"
	}
	b.mu.RUnlock()

	// 获取分区列表
	partitions, err := consumer.Partitions(topic)
	if err != nil {
		b.logger.Error("获取分区列表失败", "error", err, "topic", topic)
		return
	}

	// 为每个分区创建一个消费者
	for _, partition := range partitions {
		pc, err := consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
		if err != nil {
			b.logger.Error("创建分区消费者失败", "error", err, "topic", topic, "partition", partition)
			continue
		}

		go func(pc sarama.PartitionConsumer) {
			defer pc.Close()

			// 处理消息
			for {
				b.mu.RLock()
				if b.closed {
					b.mu.RUnlock()
					break
				}
				b.mu.RUnlock()

				select {
				case msg := <-pc.Messages():
					b.logger.Info("接收到Kafka消息", "subject", subject, "partition", msg.Partition, "offset", msg.Offset)

					// 解析事件类型
					eventType := subject
					for _, header := range msg.Headers {
						if string(header.Key) == "event_type" {
							eventType = string(header.Value)
							break
						}
					}

					// 检查消息是否匹配订阅的事件类型
					if eventType != subject {
						continue
					}

					// 反序列化事件数据
					var genericEvent map[string]interface{}
					if err := json.Unmarshal(msg.Value, &genericEvent); err != nil {
						b.logger.Error("反序列化事件数据失败", "error", err, "subject", subject)
						continue
					}

					// 创建上下文并调用处理器
					ctx := context.Background()
					// 使用GenericEvent作为事件对象
					event := &entity.GenericEvent{
						Id:        string(msg.Key),
						EventType: eventType,
						Timestamp: msg.Timestamp,
						Payload:   genericEvent,
					}

					// 调用处理器
					if err := handler(ctx, event); err != nil {
						b.logger.Error("处理事件失败", "error", err, "subject", subject)
					}

				case err := <-pc.Errors():
					b.logger.Error("Kafka消费错误", "error", err)

				case <-time.After(1 * time.Second):
					// 防止goroutine死锁
				}
			}
		}(pc)
	}
}

// Unsub 取消订阅指定类型的事件
func (b *kafkaBus) Unsub(eventType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	subject := eventType
	b.logger.Info("正在取消Kafka订阅", "subject", subject)

	// 关闭并移除消费者
	if consumer, exists := b.consumers[subject]; exists {
		consumer.Close()
		delete(b.consumers, subject)
	}

	// 移除处理器
	if _, exists := b.handlers[subject]; exists {
		delete(b.handlers, subject)
	}

	b.logger.Info("成功取消Kafka订阅", "subject", subject)

	return nil
}

// Close 关闭事件总线，释放资源
func (b *kafkaBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.logger.Info("正在关闭Kafka事件总线")

	// 关闭所有消费者
	for _, consumer := range b.consumers {
		consumer.Close()
	}

	// 关闭生产者
	if b.producer != nil {
		b.producer.Close()
	}

	// 清理状态
	b.closed = true
	b.consumers = make(map[string]sarama.Consumer)
	b.handlers = make(map[string]eventbus.EventHandler)

	b.logger.Info("Kafka事件总线已关闭")

	return nil
}