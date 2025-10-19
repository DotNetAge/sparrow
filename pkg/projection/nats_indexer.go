package projection

import (
	"context"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

// JetStreamIndexer 基于JetStream的聚合根索引实现
type JetStreamIndexer struct {
	js jetstream.JetStream // JetStream客户端
	// 流名称（存储所有事件的流，需提前创建，主题覆盖所有聚合根事件）
	streamName string
}

// NewJetStreamIndexer 创建JetStream索引器
func NewJetStreamIndexer(js jetstream.JetStream, streamName string) *JetStreamIndexer {
	return &JetStreamIndexer{
		js:         js,
		streamName: streamName,
	}
}

// GetAllAggregateIDs 获取指定聚合类型的所有聚合根ID
func (j *JetStreamIndexer) GetAllAggregateIDs(aggregateType string) ([]string, error) {
	ctx := context.Background()

	// 1. 获取流的元数据（包含所有已接收的主题）
	stream, err := j.js.Stream(ctx, j.streamName)
	if err != nil {
		return nil, fmt.Errorf("获取流失败: %w", err)
	}
	streamInfo, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取流信息失败: %w", err)
	}

	// 2. 过滤出当前聚合类型的主题（格式：{aggregateType}.{id}.{eventType}）
	// 例如：aggregateType=order → 匹配 "order.123.created"、"order.456.paid" 等
	prefix := fmt.Sprintf("%s.", aggregateType)
	aggregateIDs := make(map[string]struct{}) // 用map去重

	for _, subject := range streamInfo.Config.Subjects {
		// 检查主题是否以 "聚合类型." 开头（避免误匹配）
		if !strings.HasPrefix(subject, prefix) {
			continue
		}

		// 解析主题，提取聚合根ID（第二层级）
		// 主题格式：{aggregateType}.{id}.{eventType} → 分割后长度至少为3
		parts := strings.Split(subject, ".")
		if len(parts) < 3 {
			continue // 跳过格式错误的主题
		}
		aggregateID := parts[1] // 第二层级为聚合根ID
		aggregateIDs[aggregateID] = struct{}{}
	}

	// 3. 转换map为切片返回
	ids := make([]string, 0, len(aggregateIDs))
	for id := range aggregateIDs {
		ids = append(ids, id)
	}
	return ids, nil
}

// 辅助函数：检查字符串是否以prefix开头
// func startsWithPrefix(s, prefix string) bool {
// 	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
// }
