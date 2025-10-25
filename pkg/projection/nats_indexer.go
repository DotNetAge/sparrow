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

// JetStreamIndexer 基于JetStream的聚合根索引实现
type JetStreamIndexer struct {
	AggregateIndexer
	js         jetstream.JetStream // JetStream客户端
	streamName string              // 流名称（存储所有事件的流，需提前创建，主题覆盖所有聚合根事件）
	logger     *logger.Logger
}

// NewJetStreamIndexer 创建JetStream索引器
func NewJetStreamIndexer(conn *nats.Conn, streamName string, logger *logger.Logger) AggregateIndexer {
	js, err := jetstream.New(conn)
	if err != nil {
		logger.Panic("[聚合根索引器]创建JetStream客户端失败", "stream", streamName, "error", err)
	}

	return &JetStreamIndexer{
		js:         js,
		streamName: streamName,
		logger:     logger,
	}
}

// GetAllAggregateIDs 获取指定聚合类型的所有聚合根ID
func (j *JetStreamIndexer) GetAllAggregateIDs(aggregateType string) ([]string, error) {
	ctx := context.Background()

	// 设置上下文超时
	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 构建消费者配置
	consumerName := fmt.Sprintf("TMP_INDEXER_%s_%s", j.streamName, aggregateType)

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
	// 收集事件
	ids := make(map[string]struct{})

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
			if err := json.Unmarshal(msg.Data(), &event); err != nil {
				j.logger.Error("[事件流读取器]反序列化事件失败", "stream", j.streamName, "error", err)
				// 确认消息，防止重复处理
				_ = msg.Ack()
				continue
			}

			if event.GetAggregateType() != aggregateType {
				// 确认消息，防止重复处理
				_ = msg.Ack()
				continue
			}
			id := event.GetAggregateID()
			// 如果ID不存在于map中，就增加一个以id为键的空结构
			if _, exists := ids[id]; !exists {
				ids[id] = struct{}{}
			}
			// 确认消息，防止重复处理
			_ = msg.Ack()
		}

		// 如果当前批次没有消息，说明已经读取完所有消息，退出循环
		if messageCount == 0 {
			break
		}
	}

	// 存储所有 key 的切片
	var allKeys []string
	for k := range ids {
		allKeys = append(allKeys, k)
	}
	// 删除临时消费者
	if err := stream.DeleteConsumer(ctx, consumerName); err != nil {
		j.logger.Error("[聚合根索引器]删除临时消费者失败", "stream", j.streamName, "error", err)
		return nil, fmt.Errorf("[聚合根索引器]删除临时消费者失败: %w", err)
	}
	return allKeys, nil
}
