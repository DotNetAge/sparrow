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
	js jetstream.JetStream // JetStream客户端
	// 流名称（存储所有事件的流，需提前创建，主题覆盖所有聚合根事件）
	streamName string
	logger     *logger.Logger
}

// NewJetStreamIndexer 创建JetStream索引器
func NewJetStreamIndexer(conn *nats.Conn, streamName string, logger *logger.Logger) AggregateIndexer {
	js, err := jetstream.New(conn)
	if err != nil {
		logger.Fatal("[聚合根索引器]创建JetStream客户端失败", "stream", streamName, "error", err)
		panic(fmt.Errorf("创建JetStream客户端失败: %w", err))
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

	// // 1. 获取流的元数据（包含所有已接收的主题）
	// stream, err := j.js.Stream(ctx, j.streamName)
	// if err != nil {
	// 	j.logger.Fatal("[聚合根索引器]获取流失败", "stream", j.streamName, "error", err)
	// 	return nil, fmt.Errorf("获取流失败: %w", err)
	// }
	// streamInfo, err := stream.Info(ctx)
	// if err != nil {
	// 	j.logger.Fatal("[聚合根索引器]获取流信息失败", "stream", j.streamName, "error", err)
	// 	return nil, fmt.Errorf("获取流信息失败: %w", err)
	// }

	// // 2. 过滤出当前聚合类型的主题（格式：{aggregateType}.{id}.{eventType}）
	// // 例如：aggregateType=order → 匹配 "order.123.created"、"order.456.paid" 等
	// prefix := fmt.Sprintf("%s.", aggregateType)
	// aggregateIDs := make(map[string]struct{}) // 用map去重

	// for _, subject := range streamInfo.Config.Subjects {
	// 	// 检查主题是否以 "聚合类型." 开头（避免误匹配）
	// 	if !strings.HasPrefix(subject, prefix) {
	// 		continue
	// 	}

	// 	// 解析主题，提取聚合根ID（第二层级）
	// 	// 主题格式：{aggregateType}.{id}.{eventType} → 分割后长度至少为3
	// 	parts := strings.Split(subject, ".")
	// 	if len(parts) < 3 {
	// 		continue // 跳过格式错误的主题
	// 	}
	// 	aggregateID := parts[1] // 第二层级为聚合根ID
	// 	aggregateIDs[aggregateID] = struct{}{}
	// }

	// // 3. 转换map为切片返回
	// ids := make([]string, 0, len(aggregateIDs))
	// for id := range aggregateIDs {
	// 	ids = append(ids, id)
	// }
	// return ids, nil

	// 设置上下文超时
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
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

	consumer, err := j.getConsumer(ctxWithTimeout, cfg)
	if err != nil {
		return nil, err
	}

	// 收集事件
	ids := make(map[string]struct{})

	// 获取消息
	batch, err := consumer.FetchNoWait(1000)
	if err != nil {
		j.logger.Fatal("[事件流读取器]获取消息失败", "stream", j.streamName, "error", err)
		return nil, fmt.Errorf("[事件流读取器]获取消息失败: %w", err)
	}

	for msg := range batch.Messages() {
		// 反序列化事件
		var event entity.BaseEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			j.logger.Error("[事件流读取器]反序列化事件失败", "stream", j.streamName, "error", err)
			continue
		}

		if event.GetAggregateType() != aggregateType {
			continue
		}
		id := event.GetAggregateID()
		// 如果ID不存在于map中，就增加一个以id为键的空结构
		if _, exists := ids[id]; !exists {
			ids[id] = struct{}{}
		}
	}

	// 存储所有 key 的切片
	var allKeys []string
	for k := range ids {
		allKeys = append(allKeys, k)
	}

	return allKeys, nil
}

func (r *JetStreamIndexer) getConsumer(ctx context.Context, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	stream, err := r.js.Stream(ctx, r.streamName)
	if err != nil {
		if err == jetstream.ErrStreamNotFound {
			r.logger.Warn("[事件流读取器]获取流失败,聚合根不存在可能还未创建,不进行任何读取操作。", "stream", r.streamName, "error", err)
			return nil, nil
		}

		r.logger.Fatal("[事件流读取器]获取流失败", "stream", r.streamName, "error", err)
		return nil, fmt.Errorf("[事件流读取器]获取流失败 %w", err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		r.logger.Fatal("[事件流读取器]创建消费者失败", "stream", r.streamName, "error", err)
		return nil, fmt.Errorf("[事件流读取器]创建消费者失败: %w", err)
	}

	return consumer, nil
}
