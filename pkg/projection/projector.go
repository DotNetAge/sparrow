package projection

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// Projector 定义投影逻辑接口：将事件流转换为视图
type Projector interface {
	// Project 重放聚合根的事件流，返回最终视图
	// ctx：上下文，用于取消操作
	// aggregateType：聚合根类型（如"order"、"user"）
	// events：待投影的事件流（按版本排序）
	// 返回值：投影后的视图（如订单详情、用户信息等）
	Project(ctx context.Context, aggregateType string, events []entity.DomainEvent) (entity.Entity, error)
}
