package usecase

import (
	"context"
)

// Repository 泛型仓储接口
type Repository[T any] interface {
	// Save 保存实体
	Save(ctx context.Context, entity T) error
	// FindByID 根据实体的ID查询
	FindByID(ctx context.Context, id string) (T, error)
	// FindAll 返回所有实体
	FindAll(ctx context.Context) ([]T, error)
	// Update 更新实体数据
	Update(ctx context.Context, entity T) error
	// Delete 删除指定ID的实体
	Delete(ctx context.Context, id string) error
	// SaveBatch 批量操作
	SaveBatch(ctx context.Context, entities []T) error
	// FindByIDs 根据ID列表批量查询
	FindByIDs(ctx context.Context, ids []string) ([]T, error)
	// DeleteBatch 批量删除
	DeleteBatch(ctx context.Context, ids []string) error
	// FindWithPagination 分页查询
	FindWithPagination(ctx context.Context, limit, offset int) ([]T, error)
	// Count 统计实体数量
	Count(ctx context.Context) (int64, error)
	// FindByField 根据指定字段的值查询
	FindByField(ctx context.Context, field string, value interface{}) ([]T, error)
	// Exists 检查指定ID的实体是否存在
	Exists(ctx context.Context, id string) (bool, error)
	// FindByFieldWithPagination 按字段查找并支持分页
	FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error)
	// CountByField 按字段统计数量
	CountByField(ctx context.Context, field string, value interface{}) (int64, error)
	// FindWithConditions 根据条件查询
	FindWithConditions(ctx context.Context, options QueryOptions) ([]T, error)
	// CountWithConditions 根据条件统计
	CountWithConditions(ctx context.Context, conditions []QueryCondition) (int64, error)
}
