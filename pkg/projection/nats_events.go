package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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

// GetEvents 获取指定聚合类型下的特定事件类型的事件
// Deprecated: 该实现只返回每个聚合根的最高版本事件，与事件溯源相悖，请改用 GetAggregateEvents
func (j *JetStreamEventReader) GetEvents(aggregateType string, eventType string) ([]entity.DomainEvent, error) {
	ctx := context.Background()

	// 设置上下文超时
	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 构建消费者配置
	// consumerName := fmt.Sprintf("TMP_EVENT_READER_%s_%s", j.streamName, aggregateType)

	cfg := jetstream.ConsumerConfig{
		// Durable:       consumerName,
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

	consumer, err := stream.CreateConsumer(ctx, cfg)
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
	// if err := stream.DeleteConsumer(ctx, consumerName); err != nil {
	// 	j.logger.Error("[事件流读取器]删除临时消费者失败", "stream", j.streamName, "error", err)
	// 	// 不返回错误，因为主要功能已完成
	// }

	// 将map中的事件转换为切片
	events := make([]entity.DomainEvent, 0, len(highestVersionEvents))
	for _, event := range highestVersionEvents {
		events = append(events, event)
	}

	return events, nil
}

// GetAggregateEvents 按聚合ID获取完整事件流（按版本升序），用于聚合重放与全量投影重建
func (j *JetStreamEventReader) GetAggregateEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 构建消费者配置：过滤主题通配所有聚合类型下该聚合ID的事件
	cfg := jetstream.ConsumerConfig{
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: fmt.Sprintf("*.%s.*", aggregateID),
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	}

	stream, err := j.js.Stream(ctxWithTimeout, j.streamName)
	if err != nil {
		if err == jetstream.ErrStreamNotFound {
			j.logger.Warn("[事件流读取器]获取流失败,聚合根不存在可能还未创建,不进行任何读取操作。", "stream", j.streamName, "error", err)
			return []entity.DomainEvent{}, nil
		}

		j.logger.Error("[事件流读取器]获取流失败", "stream", j.streamName, "error", err)
		return nil, fmt.Errorf("[事件流读取器]获取流失败 %w", err)
	}

	consumer, err := stream.CreateConsumer(ctxWithTimeout, cfg)
	if err != nil {
		j.logger.Error("[事件流读取器]创建消费者失败", "stream", j.streamName, "error", err)
		return nil, fmt.Errorf("[事件流读取器]创建消费者失败: %w", err)
	}

	// 分页拉取该聚合ID的全部事件
	var events []entity.DomainEvent
	batchSize := 1000
	for {
		batch, err := consumer.FetchNoWait(batchSize)
		if err != nil {
			j.logger.Error("[事件流读取器]获取消息失败", "stream", j.streamName, "error", err)
			return nil, fmt.Errorf("[事件流读取器]获取消息失败: %w", err)
		}

		messageCount := 0
		for msg := range batch.Messages() {
			messageCount++
			// 反序列化事件
			var event entity.BaseEvent
			data := msg.Data()
			if err := json.Unmarshal(data, &event); err != nil {
				j.logger.Error("[事件流读取器]反序列化事件失败", "stream", j.streamName, "error", err)
				_ = msg.Ack()
				continue
			}
			event.Payload = data

			if event.GetAggregateID() != aggregateID {
				_ = msg.Ack()
				continue
			}

			events = append(events, &event)
			_ = msg.Ack()
		}

		if messageCount == 0 {
			break
		}
	}

	// 按版本升序排列，保证重放与投影的版本连续性
	sort.Slice(events, func(i, j int) bool {
		return events[i].GetVersion() < events[j].GetVersion()
	})

	return events, nil
}
