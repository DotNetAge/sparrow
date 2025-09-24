package repo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"purchase-service/pkg/entity"
	"purchase-service/pkg/errs"
	"purchase-service/pkg/usecase"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// GormRepository GORM仓储实现
// 基于BaseRepository[T]的完整GORM实现
// 支持泛型实体类型，提供完整的CRUD操作
// 可用于各种关系型数据库（MySQL, PostgreSQL, SQLite等）
type GormRepository[T entity.Entity] struct {
	usecase.BaseRepository[T]
	db         *gorm.DB
	tableName  string
	entityType string
	model      T
}

// NewGormRepository 创建GORM仓储实例
// 参数:
//   - db: GORM数据库连接
//
// 返回: 初始化的GORM仓储实例
func NewGormRepository[T entity.Entity](db *gorm.DB) *GormRepository[T] {
	var model T
	entityType := fmt.Sprintf("%T", model)

	// 获取表名
	var tableName string
	if _, ok := any(model).(*gorm.Model); ok && db != nil {
		// 如果实体实现了gorm.Model接口
		schema, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
		if err == nil {
			tableName = schema.Table
		}
	} else if db != nil {
		// 尝试通过GORM获取表名
		tableName = db.NamingStrategy.TableName(fmt.Sprintf("%T", model))
	}

	// 确保有表名
	if tableName == "" {
		tableName = strings.ToLower(strings.ReplaceAll(entityType, ".", "_"))
	}

	return &GormRepository[T]{
		BaseRepository: usecase.BaseRepository[T]{},
		db:             db,
		tableName:      tableName,
		entityType:     entityType,
		model:          model,
	}
}

// Save 保存实体（插入或更新）
// 如果实体ID已存在则执行更新，否则执行插入
func (r *GormRepository[T]) Save(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "save",
			Message:    "entity ID cannot be empty",
		}
	}

	// 检查是否存在
	exists, err := r.Exists(ctx, entity.GetID())
	if err != nil {
		return fmt.Errorf("failed to check existence: %w", err)
	}

	tx := r.db.WithContext(ctx)

	if exists {
		entity.SetUpdatedAt(time.Now())
		return tx.Save(entity).Error
	}

	// 新实体设置创建时间
	// 使用反射设置CreatedAt字段
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	createdAtField := entityValue.FieldByName("CreatedAt")
	if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
		createdAtField.Set(reflect.ValueOf(time.Now()))
	}

	entity.SetUpdatedAt(time.Now())
	return tx.Create(entity).Error
}

// FindByID 根据ID查找实体
func (r *GormRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	var entity T

	if id == "" {
		return entity, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	tx := r.db.WithContext(ctx)
	err := tx.Where("id = ?", id).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity, &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "find_by_id",
				ID:         id,
				Message:    "entity not found",
			}
		}
		return entity, err
	}

	return entity, nil
}

// FindAll 查找所有实体
func (r *GormRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	var entities []T
	tx := r.db.WithContext(ctx)
	err := tx.Find(&entities).Error
	return entities, err
}

// Update 更新实体
func (r *GormRepository[T]) Update(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			Message:    "entity ID cannot be empty",
		}
	}

	// 检查实体是否存在
	exists, err := r.Exists(ctx, entity.GetID())
	if err != nil {
		return err
	}

	if !exists {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			ID:         entity.GetID(),
			Message:    "entity not found",
		}
	}

	entity.SetUpdatedAt(time.Now())
	tx := r.db.WithContext(ctx)
	return tx.Where("id = ?", entity.GetID()).Updates(entity).Error
}

// Delete 删除实体（软删除，如果实体支持）
func (r *GormRepository[T]) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	// 检查实体是否存在
	exists, err := r.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "entity not found",
		}
	}

	tx := r.db.WithContext(ctx)
	// 尝试软删除
	var entity T
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return err
	}

	// 检查实体是否有DeletedAt字段
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	deletedAtField := entityValue.FieldByName("DeletedAt")
	if deletedAtField.IsValid() && deletedAtField.CanSet() {
		// 有DeletedAt字段，执行软删除
		return tx.Where("id = ?", id).Update("deleted_at", time.Now()).Error
	}

	// 没有DeletedAt字段，执行硬删除
	return tx.Where("id = ?", id).Delete(&entity).Error
}

// SaveBatch 批量保存实体
func (r *GormRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	// 检查是否所有实体都有ID
	for i, entity := range entities {
		if entity.GetID() == "" {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "save_batch",
				Message:    fmt.Sprintf("entity ID cannot be empty at index %d", i),
			}
		}
	}

	tx := r.db.WithContext(ctx)
	// 开始事务
	return tx.Transaction(func(tx *gorm.DB) error {
		for _, entity := range entities {
			// 检查是否存在
			exists, err := r.existsInTransaction(tx, entity.GetID())
			if err != nil {
				return err
			}

			if exists {
				entity.SetUpdatedAt(time.Now())
				if err := tx.Where("id = ?", entity.GetID()).Updates(entity).Error; err != nil {
					return err
				}
			} else {
				// 设置创建时间
				entityValue := reflect.ValueOf(entity)
				if entityValue.Kind() == reflect.Ptr {
					entityValue = entityValue.Elem()
				}

				createdAtField := entityValue.FieldByName("CreatedAt")
				if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
					createdAtField.Set(reflect.ValueOf(time.Now()))
				}

				entity.SetUpdatedAt(time.Now())
				if err := tx.Create(entity).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// FindByIDs 根据多个ID查找实体
func (r *GormRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	var entities []T

	if len(ids) == 0 {
		return entities, nil
	}

	tx := r.db.WithContext(ctx)
	err := tx.Where("id IN ?", ids).Find(&entities).Error
	return entities, err
}

// DeleteBatch 批量删除实体
func (r *GormRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// 检查所有ID是否有效
	for _, id := range ids {
		if id == "" {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "delete_batch",
				Message:    "id cannot be empty",
			}
		}
	}

	tx := r.db.WithContext(ctx)
	// 尝试软删除
	var entity T
	tx.First(&entity) // 只需要一个实例来检查字段

	// 检查实体是否有DeletedAt字段
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	deletedAtField := entityValue.FieldByName("DeletedAt")
	if deletedAtField.IsValid() && deletedAtField.CanSet() {
		// 有DeletedAt字段，执行软删除
		return tx.Where("id IN ?", ids).Update("deleted_at", time.Now()).Error
	}

	// 没有DeletedAt字段，执行硬删除
	return tx.Where("id IN ?", ids).Delete(&entity).Error
}

// FindWithPagination 分页查询实体
func (r *GormRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	var entities []T
	tx := r.db.WithContext(ctx)
	err := tx.Limit(limit).Offset(offset).Find(&entities).Error
	return entities, err
}

// Count 统计实体总数
func (r *GormRepository[T]) Count(ctx context.Context) (int64, error) {
	var count int64
	tx := r.db.WithContext(ctx)
	err := tx.Model(&r.model).Count(&count).Error
	return count, err
}

// FindByField 按字段查找实体
func (r *GormRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}

	var entities []T
	tx := r.db.WithContext(ctx)
	err := tx.Where(fmt.Sprintf("%s = ?", field), value).Find(&entities).Error
	return entities, err
}

// FindByFieldWithPagination 按字段查找并支持分页
func (r *GormRepository[T]) FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field_with_pagination",
			Message:    "field name cannot be empty",
		}
	}
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	var entities []T
	tx := r.db.WithContext(ctx)
	err := tx.Where(fmt.Sprintf("%s = ?", field), value).Limit(limit).Offset(offset).Find(&entities).Error
	return entities, err
}

// CountByField 按字段统计数量
func (r *GormRepository[T]) CountByField(ctx context.Context, field string, value interface{}) (int64, error) {
	if field == "" {
		return 0, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "count_by_field",
			Message:    "field name cannot be empty",
		}
	}

	var count int64
	tx := r.db.WithContext(ctx)
	err := tx.Model(&r.model).Where(fmt.Sprintf("%s = ?", field), value).Count(&count).Error
	return count, err
}

// Exists 检查实体是否存在
func (r *GormRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}

	var count int64
	tx := r.db.WithContext(ctx)
	err := tx.Model(&r.model).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}

// FindWithConditions 根据条件查询
func (r *GormRepository[T]) FindWithConditions(ctx context.Context, options usecase.QueryOptions) ([]T, error) {
	if options.Limit <= 0 {
		options.Limit = 10
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	var entities []T
	tx := r.db.WithContext(ctx).Model(&r.model)

	// 应用条件
	if len(options.Conditions) > 0 {
		for _, condition := range options.Conditions {
			tx = r.applyCondition(tx, condition)
		}
	}

	// 应用排序
	if len(options.SortFields) > 0 {
		for _, sortField := range options.SortFields {
			order := "ASC"
			if !sortField.Ascending {
				order = "DESC"
			}
			tx = tx.Order(fmt.Sprintf("%s %s", sortField.Field, order))
		}
	} else {
		// 默认排序
		tx = tx.Order("created_at DESC")
	}

	// 应用分页
	tx = tx.Limit(options.Limit).Offset(options.Offset)

	err := tx.Find(&entities).Error
	return entities, err
}

// CountWithConditions 根据条件统计
func (r *GormRepository[T]) CountWithConditions(ctx context.Context, conditions []usecase.QueryCondition) (int64, error) {
	var count int64
	tx := r.db.WithContext(ctx).Model(&r.model)

	// 应用条件
	if len(conditions) > 0 {
		for _, condition := range conditions {
			tx = r.applyCondition(tx, condition)
		}
	}

	err := tx.Count(&count).Error
	return count, err
}

// 辅助方法

// existsInTransaction 在事务中检查实体是否存在
func (r *GormRepository[T]) existsInTransaction(tx *gorm.DB, id string) (bool, error) {
	var count int64
	err := tx.Model(&r.model).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}

// applyCondition 应用单个查询条件
func (r *GormRepository[T]) applyCondition(tx *gorm.DB, condition usecase.QueryCondition) *gorm.DB {
	field := condition.Field
	op := condition.Operator
	value := condition.Value

	switch op {
	case "EQ":
		return tx.Where(fmt.Sprintf("%s = ?", field), value)
	case "NEQ":
		return tx.Where(fmt.Sprintf("%s <> ?", field), value)
	case "GT":
		return tx.Where(fmt.Sprintf("%s > ?", field), value)
	case "GTE":
		return tx.Where(fmt.Sprintf("%s >= ?", field), value)
	case "LT":
		return tx.Where(fmt.Sprintf("%s < ?", field), value)
	case "LTE":
		return tx.Where(fmt.Sprintf("%s <= ?", field), value)
	case "LIKE":
		return tx.Where(fmt.Sprintf("%s LIKE ?", field), fmt.Sprintf("%%%v%%", value))
	case "IN":
		return tx.Where(fmt.Sprintf("%s IN ?", field), value)
	case "IS_NULL":
		return tx.Where(fmt.Sprintf("%s IS NULL", field))
	case "IS_NOT_NULL":
		return tx.Where(fmt.Sprintf("%s IS NOT NULL", field))
	default:
		// 默认使用等于
		return tx.Where(fmt.Sprintf("%s = ?", field), value)
	}
}

// GORM特有的方法

// HardDelete 硬删除实体（忽略软删除）
func (r *GormRepository[T]) HardDelete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "hard_delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	tx := r.db.WithContext(ctx).Unscoped() // Unscoped忽略软删除
	return tx.Where("id = ?", id).Delete(&r.model).Error
}

// FindDeleted 查找已删除的实体（软删除的记录）
func (r *GormRepository[T]) FindDeleted(ctx context.Context) ([]T, error) {
	var entities []T
	tx := r.db.WithContext(ctx).Unscoped() // Unscoped包含已软删除的记录
	err := tx.Where("deleted_at IS NOT NULL").Find(&entities).Error
	return entities, err
}

// Restore 恢复已删除的实体（取消软删除）
func (r *GormRepository[T]) Restore(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "restore",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	tx := r.db.WithContext(ctx).Unscoped() // Unscoped包含已软删除的记录
	err := tx.Where("id = ?", id).Update("deleted_at", nil).Error
	return err
}

// WithTx 设置事务
func (r *GormRepository[T]) WithTx(tx *gorm.DB) *GormRepository[T] {
	r.db = tx
	return r
}

// GetDB 获取GORM数据库连接
func (r *GormRepository[T]) GetDB() *gorm.DB {
	return r.db
}

// TableName 获取表名
func (r *GormRepository[T]) TableName() string {
	return r.tableName
}
