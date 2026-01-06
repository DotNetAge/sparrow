package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"

	"github.com/dgraph-io/badger/v4"
)

// BadgerRepository BadgerDB仓储实现
// 基于BaseRepository[T]的完整BadgerDB实现
// 支持泛型实体类型，提供完整的CRUD操作

type BadgerRepository[T entity.Entity] struct {
	usecase.BaseRepository[T]
	db         *badger.DB
	prefix     string
	entityType string
}

var _ usecase.Repository[entity.Entity] = (*BadgerRepository[entity.Entity])(nil)

// NewBadgerRepository 创建BadgerDB仓储实例
// 参数:
//   - db: BadgerDB实例
//   - prefix: 键前缀，用于区分不同类型的实体
//
// 返回: 初始化的BadgerDB仓储实例
func NewBadgerRepository[T entity.Entity](db *badger.DB, prefix string) usecase.Repository[T] {
	var zero T
	entityType := fmt.Sprintf("%T", zero)

	// 确保前缀以冒号结尾
	if prefix != "" && !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}

	return &BadgerRepository[T]{
		BaseRepository: usecase.BaseRepository[T]{},
		db:             db,
		prefix:         prefix,
		entityType:     entityType,
	}
}

// Save 保存实体（插入或更新）
// 如果实体ID已存在则执行更新，否则执行插入
func (r *BadgerRepository[T]) Save(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "save",
			Message:    "entity ID cannot be empty",
		}
	}

	// 检查实体是否存在
	exists, err := r.Exists(ctx, entity.GetID())
	if err != nil {
		return fmt.Errorf("failed to check existence: %w", err)
	}

	if exists {
		return r.Update(ctx, entity)
	}

	return r.insert(ctx, entity)
}

// insert 插入新实体
func (r *BadgerRepository[T]) insert(ctx context.Context, entity T) error {
	// 使用反射设置CreatedAt字段
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}
	createdAtField := entityValue.FieldByName("CreatedAt")
	if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
		if createdAtField.IsZero() { // 没有值的时候才更新
			createdAtField.Set(reflect.ValueOf(time.Now()))
		}
	}
	entity.SetUpdatedAt(time.Now())

	data, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	key := r.buildKey(entity.GetID())

	err = r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})

	return err
}

// Update 更新实体
func (r *BadgerRepository[T]) Update(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			Message:    "entity ID cannot be empty",
		}
	}

	entity.SetUpdatedAt(time.Now())

	data, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	key := r.buildKey(entity.GetID())

	err = r.db.Update(func(txn *badger.Txn) error {
		// 检查实体是否存在
		_, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return &errs.RepositoryError{
					EntityType: r.entityType,
					Operation:  "update",
					ID:         entity.GetID(),
					Message:    "entity not found",
				}
			}
			return err
		}

		return txn.Set(key, data)
	})

	return err
}

// FindByID 根据ID查找实体
func (r *BadgerRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	if id == "" {
		var zero T
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	key := r.buildKey(id)
	var entity T

	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return &errs.RepositoryError{
					EntityType: r.entityType,
					Operation:  "find_by_id",
					ID:         id,
					Message:    "entity not found",
				}
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &entity)
		})
	})

	return entity, err
}

// FindAll 查找所有实体
func (r *BadgerRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	var entities []T

	err := r.db.View(func(txn *badger.Txn) error {
		prefix := []byte(r.prefix)

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var entity T
				if err := json.Unmarshal(val, &entity); err != nil {
					return fmt.Errorf("failed to unmarshal entity: %w", err)
				}
				entities = append(entities, entity)
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return entities, err
}

// Delete 删除实体（硬删除，BadgerDB不支持软删除）
func (r *BadgerRepository[T]) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	key := r.buildKey(id)

	err := r.db.Update(func(txn *badger.Txn) error {
		// 检查实体是否存在
		_, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return &errs.RepositoryError{
					EntityType: r.entityType,
					Operation:  "delete",
					ID:         id,
					Message:    "entity not found",
				}
			}
			return err
		}

		return txn.Delete(key)
	})

	return err
}

// SaveBatch 批量保存实体
func (r *BadgerRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	err := r.db.Update(func(txn *badger.Txn) error {
		for _, entity := range entities {
			if entity.GetID() == "" {
				return &errs.RepositoryError{
					EntityType: r.entityType,
					Operation:  "save_batch",
					Message:    "entity ID cannot be empty",
				}
			}

			// 检查是否存在
			key := r.buildKey(entity.GetID())
			_, err := txn.Get(key)

			if err == badger.ErrKeyNotFound {
				// 插入
				// 使用反射设置CreatedAt字段
				entityValue := reflect.ValueOf(entity)
				if entityValue.Kind() == reflect.Ptr {
					entityValue = entityValue.Elem()
				}
				createdAtField := entityValue.FieldByName("CreatedAt")
				if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
					if createdAtField.IsZero() { // 没有值的时候才更新
						createdAtField.Set(reflect.ValueOf(time.Now()))
					}
				}
				entity.SetUpdatedAt(time.Now())
			} else if err != nil {
				return err
			} else {
				// 更新
				entity.SetUpdatedAt(time.Now())
			}

			data, err := json.Marshal(entity)
			if err != nil {
				return fmt.Errorf("failed to marshal entity: %w", err)
			}

			if err := txn.Set(key, data); err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

// FindByIDs 根据多个ID查找实体
func (r *BadgerRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	var entities []T

	err := r.db.View(func(txn *badger.Txn) error {
		for _, id := range ids {
			key := r.buildKey(id)

			item, err := txn.Get(key)
			if err != nil {
				if err == badger.ErrKeyNotFound {
					continue // 跳过不存在的实体
				}
				return err
			}

			err = item.Value(func(val []byte) error {
				var entity T
				if err = json.Unmarshal(val, &entity); err != nil {
					return fmt.Errorf("failed to unmarshal entity: %w", err)
				}
				entities = append(entities, entity)
				return nil
			})

			if err != nil {
				return err
			}
		}
		return nil
	})

	return entities, err
}

// DeleteBatch 批量删除实体
func (r *BadgerRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	err := r.db.Update(func(txn *badger.Txn) error {
		for _, id := range ids {
			key := r.buildKey(id)

			// 检查是否存在
			_, err := txn.Get(key)
			if err == badger.ErrKeyNotFound {
				continue // 跳过不存在的实体
			} else if err != nil {
				return err
			}

			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

// FindWithPagination 分页查询实体
func (r *BadgerRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	var entities []T
	var count int

	err := r.db.View(func(txn *badger.Txn) error {
		prefix := []byte(r.prefix)

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		// 跳过offset
		for it.Seek(prefix); it.ValidForPrefix(prefix) && count < offset; it.Next() {
			count++
		}

		// 获取limit个实体
		for it.ValidForPrefix(prefix) && len(entities) < limit {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var entity T
				if err := json.Unmarshal(val, &entity); err != nil {
					return fmt.Errorf("failed to unmarshal entity: %w", err)
				}
				entities = append(entities, entity)
				return nil
			})

			if err != nil {
				return err
			}

			it.Next()
		}

		return nil
	})

	return entities, err
}

// Count 统计实体总数
func (r *BadgerRepository[T]) Count(ctx context.Context) (int64, error) {
	var count int64

	err := r.db.View(func(txn *badger.Txn) error {
		prefix := []byte(r.prefix)

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}

		return nil
	})

	return count, err
}

// FindByField 按字段查找实体
// 注意：BadgerDB不支持索引，这是全表扫描操作
func (r *BadgerRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}

	var entities []T

	err := r.db.View(func(txn *badger.Txn) error {
		prefix := []byte(r.prefix)

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var entity T
				if err := json.Unmarshal(val, &entity); err != nil {
					return fmt.Errorf("failed to unmarshal entity: %w", err)
				}

				// 使用反射获取字段值
				entityValue := reflect.ValueOf(entity)
				if entityValue.Kind() == reflect.Ptr {
					entityValue = entityValue.Elem()
				}

				fieldValue := entityValue.FieldByName(field)
				if !fieldValue.IsValid() {
					return fmt.Errorf("field %s not found", field)
				}

				// 比较字段值
				if reflect.DeepEqual(fieldValue.Interface(), value) {
					entities = append(entities, entity)
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return entities, err
}

// Exists 检查实体是否存在
func (r *BadgerRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}

	key := r.buildKey(id)

	err := r.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		return err
	})

	if err == badger.ErrKeyNotFound {
		return false, nil
	}

	return err == nil, err
}

// buildKey 构建存储键
func (r *BadgerRepository[T]) buildKey(id string) []byte {
	return []byte(r.prefix + id)
}

// 注意：BadgerDB不支持以下PostgreSQL特有的方法
// - HardDelete（所有删除都是硬删除）
// - FindDeleted（没有软删除概念）
// - Restore（没有软删除概念）

// FindByFieldWithPagination 按字段查找并支持分页
func (r *BadgerRepository[T]) FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error) {
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

	// 先获取所有符合条件的实体
	allMatches, err := r.FindByField(ctx, field, value)
	if err != nil {
		return nil, err
	}

	// 应用分页
	start := offset
	end := offset + limit
	if start >= len(allMatches) {
		return []T{}, nil
	}
	if end > len(allMatches) {
		end = len(allMatches)
	}

	return allMatches[start:end], nil
}

// CountByField 按字段统计数量
func (r *BadgerRepository[T]) CountByField(ctx context.Context, field string, value interface{}) (int64, error) {
	if field == "" {
		return 0, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "count_by_field",
			Message:    "field name cannot be empty",
		}
	}

	entities, err := r.FindByField(ctx, field, value)
	if err != nil {
		return 0, err
	}

	return int64(len(entities)), nil
}

// Random 返回随机实体
func (r *BadgerRepository[T]) Random(ctx context.Context, take int) ([]T, error) {
	if take <= 0 {
		take = 1
	}

	// 先获取所有实体的键
	var keys []string

	err := r.db.View(func(txn *badger.Txn) error {
		prefix := []byte(r.prefix)

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := string(item.Key())
			// 移除前缀，只保留ID部分
			id := strings.TrimPrefix(key, r.prefix)
			keys = append(keys, id)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return []T{}, nil
	}

	// 随机选择take个键
	utils.ShuffleStrings(keys)
	if len(keys) > take {
		keys = keys[:take]
	}

	// 根据选择的键获取实体
	return r.FindByIDs(ctx, keys)
}

// FindWithConditions 根据条件查询
func (r *BadgerRepository[T]) FindWithConditions(ctx context.Context, options usecase.QueryOptions) ([]T, error) {
	if options.Limit <= 0 {
		options.Limit = 10
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	var entities []T

	err := r.db.View(func(txn *badger.Txn) error {
		prefix := []byte(r.prefix)

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var entity T
				if err := json.Unmarshal(val, &entity); err != nil {
					return fmt.Errorf("failed to unmarshal entity: %w", err)
				}

				// 检查是否符合所有条件
				if r.matchConditions(entity, options.Conditions) {
					entities = append(entities, entity)
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 应用排序
	r.sortEntities(entities, options.SortFields)

	// 应用分页
	start := options.Offset
	end := options.Offset + options.Limit
	if start >= len(entities) {
		return []T{}, nil
	}
	if end > len(entities) {
		end = len(entities)
	}

	return entities[start:end], nil
}

// CountWithConditions 根据条件统计
func (r *BadgerRepository[T]) CountWithConditions(ctx context.Context, conditions []usecase.QueryCondition) (int64, error) {
	var count int64

	err := r.db.View(func(txn *badger.Txn) error {
		prefix := []byte(r.prefix)

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var entity T
				if err := json.Unmarshal(val, &entity); err != nil {
					return fmt.Errorf("failed to unmarshal entity: %w", err)
				}

				// 检查是否符合所有条件
				if r.matchConditions(entity, conditions) {
					count++
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return count, err
}

// matchConditions 检查实体是否符合所有条件
func (r *BadgerRepository[T]) matchConditions(entity T, conditions []usecase.QueryCondition) bool {
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	for _, condition := range conditions {
		fieldValue := entityValue.FieldByName(condition.Field)
		if !fieldValue.IsValid() {
			return false
		}

		if !r.compareField(fieldValue.Interface(), condition.Operator, condition.Value) {
			return false
		}
	}

	return true
}

// compareField 比较字段值和条件值
func (r *BadgerRepository[T]) compareField(fieldValue interface{}, operator string, conditionValue interface{}) bool {
	// 简化实现，支持基本类型的比较

	fieldValueStr := fmt.Sprintf("%v", fieldValue)

	switch operator {
	case "EQ":
		return fieldValueStr == fmt.Sprintf("%v", conditionValue)
	case "NEQ":
		return fieldValueStr != fmt.Sprintf("%v", conditionValue)
	case "LIKE":
		// 简化的LIKE实现，支持简单的包含
		return strings.Contains(fieldValueStr, fmt.Sprintf("%v", conditionValue))
	case "IN":
		// 检查字段值是否在条件值列表中
		// 首先检查字段值是否是数组/切片类型
		fieldReflectValue := reflect.ValueOf(fieldValue)
		if fieldReflectValue.Kind() == reflect.Slice || fieldReflectValue.Kind() == reflect.Array {
			// 如果字段值是数组，检查数组中是否有元素在条件值列表中
			for i := 0; i < fieldReflectValue.Len(); i++ {
				fieldElement := fmt.Sprintf("%v", fieldReflectValue.Index(i).Interface())
				if r.isValueInSlice(fieldElement, conditionValue) {
					return true
				}
			}
			return false
		} else {
			// 如果字段值不是数组，直接检查字段值是否在条件值列表中
			return r.isValueInSlice(fieldValueStr, conditionValue)
		}
	default:
		// 其他操作符可以根据需要实现
		return false
	}
}

// isValueInSlice 检查值是否在切片中
func (r *BadgerRepository[T]) isValueInSlice(value string, slice interface{}) bool {
	sliceValue := reflect.ValueOf(slice)
	if sliceValue.Kind() != reflect.Slice {
		return false
	}

	for i := 0; i < sliceValue.Len(); i++ {
		item := fmt.Sprintf("%v", sliceValue.Index(i).Interface())
		if item == value {
			return true
		}
	}
	return false
}

// sortEntities 对实体列表进行排序
// 使用通用的排序工具函数，简化排序逻辑
func (r *BadgerRepository[T]) sortEntities(entities []T, sortFields []usecase.SortField) {
	utils.SortEntities(entities, sortFields, false)
}

// getFieldValue 使用反射获取字段值
func (r *BadgerRepository[T]) getFieldValue(entity T, fieldName string) interface{} {
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	fieldValue := entityValue.FieldByName(fieldName)
	if !fieldValue.IsValid() {
		return nil
	}

	return fieldValue.Interface()
}

// Close 关闭数据库连接
func (r *BadgerRepository[T]) Close() error {
	return r.db.Close()
}
