package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type JetStreamEventReader struct {
	EventReader
	js         jetstream.JetStream // JetStream客户端
	streamName string              // 流名称（存储所有事件的流，需提前创建，主题覆盖所有聚合根事件）
	logger     *logger.Logger
}

func NewJetStreamEventReader(
	conn *nats.Conn,
	streamName string,
	logger *logger.Logger,
) EventReader {
	js, err := jetstream.New(conn)
	if err != nil {
		logger.Panic("[事件流读取器]创建JetStream客户端失败", "error", err)
	}

	return &JetStreamEventReader{
		js:         js,
		streamName: streamName,
		logger:     logger,
	}
}

func (j *JetStreamEventReader) GetEvents(aggregateType string, eventType string) ([]entity.DomainEvent, error) {
	ctx := context.Background()

	// 设置上下文超时
	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 构建消费者配置
	consumerName := fmt.Sprintf("TMP_EVENT_READER_%s_%s", j.streamName, aggregateType)

	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("%s.*.*", aggregateType),
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	stream, err := j.js.Stream(ctx, j.streamName)
	if err != nil {
		if err == jetstream.ErrStreamNotFound {
			j.logger.Warn("[事件流读取器]获取流失败,聚合根不存在可能还未创建,不进行任何读取操作。", "stream", j.streamName, "error", err)
			return nil, nil
		}

		j.logger.Error("[事件流读取器]获取流失败", "stream", j.streamName, "error", err)
		return nil, fmt.Errorf("[事件流读取器]获取流失败 %w", err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		j.logger.Error("[事件流读取器]创建消费者失败", "stream", j.streamName, "error", err)
		return nil, fmt.Errorf("[事件流读取器]创建消费者失败: %w", err)
	}
	// 使用map存储每个AggregateID对应的最高版本事件
	highestVersionEvents := make(map[string]entity.DomainEvent)

	// 分页获取所有消息
	batchSize := 1000
	for {
		// 获取消息批次
		batch, err := consumer.FetchNoWait(batchSize)
		if err != nil {
			j.logger.Error("[事件流读取器]获取消息失败", "stream", j.streamName, "error", err)
			return nil, fmt.Errorf("[事件流读取器]获取消息失败: %w", err)
		}

		// 处理批次中的所有消息
		messageCount := 0
		for msg := range batch.Messages() {
			messageCount++
			// 反序列化事件
			var event entity.BaseEvent
			data := msg.Data()
			if err := json.Unmarshal(data, &event); err != nil {
				j.logger.Error("[事件流读取器]反序列化事件失败", "stream", j.streamName, "error", err)
				// 确认消息，防止消息重复处理
				_ = msg.Ack()
				continue
			}
			event.Payload = data

			if event.GetAggregateType() != aggregateType || event.GetEventType() != eventType {
				// 确认消息，防止消息重复处理
				_ = msg.Ack()
				continue
			}

			// 获取当前事件的AggregateID和Version
			aggID := event.GetAggregateID()
			aggVersion := event.GetVersion()

			// 如果该AggregateID不存在于map中，或者当前事件的Version高于已存储的事件，则更新map
			if existingEvent, exists := highestVersionEvents[aggID]; !exists || aggVersion > existingEvent.GetVersion() {
				highestVersionEvents[aggID] = &event
			}

			// 确认消息，防止消息重复处理
			_ = msg.Ack()
		}

		// 如果当前批次没有消息，说明已经读取完所有消息，退出循环
		if messageCount == 0 {
			break
		}
	}

	// 删除临时消费者，清理资源
	if err := stream.DeleteConsumer(ctx, consumerName); err != nil {
		j.logger.Error("[事件流读取器]删除临时消费者失败", "stream", j.streamName, "error", err)
		// 不返回错误，因为主要功能已完成
	}

	// 将map中的事件转换为切片
	events := make([]entity.DomainEvent, 0, len(highestVersionEvents))
	for _, event := range highestVersionEvents {
		events = append(events, event)
	}

	return events, nil
}
