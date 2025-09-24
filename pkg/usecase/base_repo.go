package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
)

// QueryCondition 查询条件
type QueryCondition struct {
	Field    string
	Operator string // EQ, NEQ, GT, GTE, LT, LTE, LIKE, IN, NOT_IN, IS_NULL, IS_NOT_NULL
	Value    interface{}
}

// SortField 排序字段
type SortField struct {
	Field     string
	Ascending bool
}

// QueryOptions 查询选项
type QueryOptions struct {
	Conditions []QueryCondition
	SortFields []SortField
	Limit      int
	Offset     int
}

// BaseRepository 基础仓储实现
type BaseRepository[T entity.Entity] struct {
	entityType string
}

// NewBaseRepository 创建基础仓储
func NewBaseRepository[T entity.Entity]() *BaseRepository[T] {
	var zero T
	return &BaseRepository[T]{
		entityType: fmt.Sprintf("%T", zero),
	}
}

// Save 保存实体
func (r *BaseRepository[T]) Save(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "save",
			Message:    "entity ID cannot be empty",
		}
	}
	return errors.New("not implemented: Save method needs to be implemented by specific repository")
}

// FindByID 根据ID查找
func (r *BaseRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	if id == "" {
		var zero T
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}
	var zero T
	return zero, errors.New("not implemented: FindByID method needs to be implemented by specific repository")
}

// FindAll 查找所有
func (r *BaseRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	return nil, errors.New("not implemented: FindAll method needs to be implemented by specific repository")
}

// Update 更新实体
func (r *BaseRepository[T]) Update(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			Message:    "entity ID cannot be empty",
		}
	}
	entity.SetUpdatedAt(time.Now())
	return errors.New("not implemented: Update method needs to be implemented by specific repository")
}

// Delete 删除实体
func (r *BaseRepository[T]) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}
	return errors.New("not implemented: Delete method needs to be implemented by specific repository")
}

// SaveBatch 批量保存
func (r *BaseRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}
	return errors.New("not implemented: SaveBatch method needs to be implemented by specific repository")
}

// FindByIDs 批量查找
func (r *BaseRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}
	return nil, errors.New("not implemented: FindByIDs method needs to be implemented by specific repository")
}

// DeleteBatch 批量删除
func (r *BaseRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return errors.New("not implemented: DeleteBatch method needs to be implemented by specific repository")
}

// FindWithPagination 分页查询
func (r *BaseRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	return nil, errors.New("not implemented")
}

// Count 统计总数
func (r *BaseRepository[T]) Count(ctx context.Context) (int64, error) {
	return 0, errors.New("not implemented: Count method needs to be implemented by specific repository")
}

// FindByField 按字段查找（保留原有方法以保持向后兼容）
func (r *BaseRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}
	return nil, errors.New("not implemented: FindByField method needs to be implemented by specific repository")
}

// FindByFieldWithPagination 按字段查找并支持分页（新增方法）
func (r *BaseRepository[T]) FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field_with_pagination",
			Message:    "field name cannot be empty",
		}
	}
	return nil, errors.New("not implemented")
}

// CountByField 按字段统计数量（新增方法）
func (r *BaseRepository[T]) CountByField(ctx context.Context, field string, value interface{}) (int64, error) {
	if field == "" {
		return 0, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "count_by_field",
			Message:    "field name cannot be empty",
		}
	}
	return 0, errors.New("not implemented: CountByField method needs to be implemented by specific repository")
}

// Exists 检查是否存在
func (r *BaseRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	return false, errors.New("not implemented: Exists method needs to be implemented by specific repository")
}

// 支持多条件查询的方法

// FindWithConditions 根据条件查询
func (r *BaseRepository[T]) FindWithConditions(ctx context.Context, options QueryOptions) ([]T, error) {
	return nil, errors.New("not implemented: FindWithConditions method needs to be implemented by specific repository")
}

// CountWithConditions 根据条件统计
func (r *BaseRepository[T]) CountWithConditions(ctx context.Context, conditions []QueryCondition) (int64, error) {
	return 0, errors.New("not implemented: CountWithConditions method needs to be implemented by specific repository")
}

// 创建查询条件构建器
func NewQueryCondition(field, operator string, value interface{}) QueryCondition {
	return QueryCondition{
		Field:    field,
		Operator: operator,
		Value:    value,
	}
}

// 创建排序字段
func NewSortField(field string, ascending bool) SortField {
	return SortField{
		Field:     field,
		Ascending: ascending,
	}
}
