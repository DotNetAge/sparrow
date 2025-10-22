package projection

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// AggregateIndexer 聚合根索引管理器接口
// 负责维护和查询所有有效聚合根的ID
//
//	使用场景：用于定时投影与全量投影时使用
type AggregateIndexer interface {
	// GetAllAggregateIDs 获取指定聚合类型的所有有效聚合根ID
	// aggregateType：聚合根类型（如"order"、"user"）
	// 返回值：该类型下的所有聚合根ID列表
	GetAllAggregateIDs(aggregateType string) ([]string, error)
}

// EventReader 事件读取器接口
// 负责从事件存储中读取聚合根的事件流
//
//	使用场景：用于投影时从事件存储中获取事件流
type EventReader interface {
	// GetEvents 获取指定聚合类型下的特定最新的事件
	// aggregateType：聚合根类型（如"order"、"user"）
	// eventType：事件类型（如"order_placed"、"user_registered"）
	// 返回值：该类型下的所有事件列表
	GetEvents(aggregateType string, eventType string) ([]entity.DomainEvent, error)
}

// Projector 定义投影逻辑接口：将事件流转换为视图
//
//	使用场景：用于将事件流转换为聚合根的当前状态视图（如订单详情、用户信息等）
type Projector interface {
	// Project 重放聚合根的事件流还原出聚合根的当前状态然后投影成为视图，返回最终视图
	// ctx：上下文，用于取消操作
	// aggregateType：聚合根类型（如"order"、"user"）
	// aggregateID：聚合根ID（如"order_123"、"user_456"）
	// events：待投影的事件流（按版本排序）
	// 返回值：投影后的视图（如订单详情、用户信息等）
	Project(ctx context.Context, aggregateType string, aggregateID string, events []entity.DomainEvent) (entity.Entity, error)
}
