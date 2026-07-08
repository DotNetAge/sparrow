package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"

	bolt "go.etcd.io/bbolt"
)

// BoltRepository bbolt 仓储实现
// 基于 BaseRepository[T] 的完整 bbolt 实现
// 支持泛型实体类型，提供完整的 CRUD 操作
type BoltRepository[T entity.Entity] struct {
	usecase.BaseRepository[T]
	db         *bolt.DB
	bucketName []byte
	entityType string
}

var _ usecase.Repository[entity.Entity] = (*BoltRepository[entity.Entity])(nil)

// NewBoltRepository 创建 bbolt 仓储实例
// 参数:
//   - db: bbolt 实例
//   - bucket: 存储桶名称，用于区分不同类型的实体
//
// 返回: 初始化的 bbolt 仓储实例
func NewBoltRepository[T entity.Entity](db *bolt.DB, bucket string) usecase.Repository[T] {
	var zero T
	entityType := fmt.Sprintf("%T", zero)

	if bucket == "" {
		bucket = strings.ReplaceAll(entityType, "*", "")
		bucket = strings.ReplaceAll(bucket, ".", "_")
	}

	return &BoltRepository[T]{
		BaseRepository: usecase.BaseRepository[T]{},
		db:             db,
		bucketName:     []byte(bucket),
		entityType:     entityType,
	}
}

// ensureBucket 确保 bucket 存在
func (r *BoltRepository[T]) ensureBucket() error {
	return r.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(r.bucketName)
		return err
	})
}

// Save 保存实体（插入或更新）
// 如果实体 ID 已存在则执行更新，否则执行插入
func (r *BoltRepository[T]) Save(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "save",
			Message:    "entity ID cannot be empty",
		}
	}

	if err := r.ensureBucket(); err != nil {
		return fmt.Errorf("failed to ensure bucket: %w", err)
	}

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
func (r *BoltRepository[T]) insert(ctx context.Context, entity T) error {
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}
	createdAtField := entityValue.FieldByName("CreatedAt")
	if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
		if createdAtField.IsZero() {
			createdAtField.Set(reflect.ValueOf(time.Now()))
		}
	}
	entity.SetUpdatedAt(time.Now())

	data, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	key := []byte(entity.GetID())

	return r.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return fmt.Errorf("bucket %s not found", r.bucketName)
		}
		return b.Put(key, data)
	})
}

// Update 更新实体
func (r *BoltRepository[T]) Update(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			Message:    "entity ID cannot be empty",
		}
	}

	if err := r.ensureBucket(); err != nil {
		return fmt.Errorf("failed to ensure bucket: %w", err)
	}

	entity.SetUpdatedAt(time.Now())

	data, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	key := []byte(entity.GetID())

	return r.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return fmt.Errorf("bucket %s not found", r.bucketName)
		}

		if b.Get(key) == nil {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "update",
				ID:         entity.GetID(),
				Message:    "entity not found",
			}
		}

		return b.Put(key, data)
	})
}

// FindByID 根据 ID 查找实体
func (r *BoltRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	if id == "" {
		var zero T
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	if err := r.ensureBucket(); err != nil {
		var zero T
		return zero, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var result T
	key := []byte(id)

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "find_by_id",
				ID:         id,
				Message:    "entity not found",
			}
		}

		data := b.Get(key)
		if data == nil {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "find_by_id",
				ID:         id,
				Message:    "entity not found",
			}
		}

		return json.Unmarshal(data, &result)
	})

	return result, err
}

// FindAll 查找所有实体
func (r *BoltRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	if err := r.ensureBucket(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var entities []T

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var entity T
			if err := json.Unmarshal(v, &entity); err != nil {
				return fmt.Errorf("failed to unmarshal entity: %w", err)
			}
			entities = append(entities, entity)
			return nil
		})
	})

	return entities, err
}

// Delete 删除实体（硬删除）
func (r *BoltRepository[T]) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	if err := r.ensureBucket(); err != nil {
		return fmt.Errorf("failed to ensure bucket: %w", err)
	}

	key := []byte(id)

	return r.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "delete",
				ID:         id,
				Message:    "entity not found",
			}
		}

		if b.Get(key) == nil {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "delete",
				ID:         id,
				Message:    "entity not found",
			}
		}

		return b.Delete(key)
	})
}

// SaveBatch 批量保存实体
func (r *BoltRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	if err := r.ensureBucket(); err != nil {
		return fmt.Errorf("failed to ensure bucket: %w", err)
	}

	return r.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return fmt.Errorf("bucket %s not found", r.bucketName)
		}

		for _, entity := range entities {
			if entity.GetID() == "" {
				return &errs.RepositoryError{
					EntityType: r.entityType,
					Operation:  "save_batch",
					Message:    "entity ID cannot be empty",
				}
			}

			entityValue := reflect.ValueOf(entity)
			if entityValue.Kind() == reflect.Ptr {
				entityValue = entityValue.Elem()
			}
			createdAtField := entityValue.FieldByName("CreatedAt")
			if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
				if createdAtField.IsZero() {
					createdAtField.Set(reflect.ValueOf(time.Now()))
				}
			}
			entity.SetUpdatedAt(time.Now())

			data, err := json.Marshal(entity)
			if err != nil {
				return fmt.Errorf("failed to marshal entity: %w", err)
			}

			if err := b.Put([]byte(entity.GetID()), data); err != nil {
				return err
			}
		}

		return nil
	})
}

// FindByIDs 根据多个 ID 查找实体
func (r *BoltRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	if err := r.ensureBucket(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var entities []T

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		for _, id := range ids {
			data := b.Get([]byte(id))
			if data == nil {
				continue
			}

			var entity T
			if err := json.Unmarshal(data, &entity); err != nil {
				return fmt.Errorf("failed to unmarshal entity: %w", err)
			}
			entities = append(entities, entity)
		}

		return nil
	})

	return entities, err
}

// DeleteBatch 批量删除实体
func (r *BoltRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	if err := r.ensureBucket(); err != nil {
		return fmt.Errorf("failed to ensure bucket: %w", err)
	}

	return r.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		for _, id := range ids {
			if err := b.Delete([]byte(id)); err != nil {
				return err
			}
		}

		return nil
	})
}

// FindWithPagination 分页查询实体
func (r *BoltRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	if err := r.ensureBucket(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var entities []T

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		c := b.Cursor()
		count := 0
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if count < offset {
				count++
				continue
			}

			if len(entities) >= limit {
				break
			}

			var entity T
			if err := json.Unmarshal(v, &entity); err != nil {
				return fmt.Errorf("failed to unmarshal entity: %w", err)
			}
			entities = append(entities, entity)
		}

		return nil
	})

	return entities, err
}

// Count 统计实体总数
func (r *BoltRepository[T]) Count(ctx context.Context) (int64, error) {
	if err := r.ensureBucket(); err != nil {
		return 0, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var count int64

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
	})

	return count, err
}

// FindByField 按字段查找实体
// 注意：bbolt 不支持索引，这是全表扫描操作
func (r *BoltRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}

	if err := r.ensureBucket(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var entities []T

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var entity T
			if err := json.Unmarshal(v, &entity); err != nil {
				return fmt.Errorf("failed to unmarshal entity: %w", err)
			}

			entityValue := reflect.ValueOf(entity)
			if entityValue.Kind() == reflect.Ptr {
				entityValue = entityValue.Elem()
			}

			fieldValue := entityValue.FieldByName(field)
			if !fieldValue.IsValid() {
				return nil
			}

			if reflect.DeepEqual(fieldValue.Interface(), value) {
				entities = append(entities, entity)
			}

			return nil
		})
	})

	return entities, err
}

// Exists 检查实体是否存在
func (r *BoltRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}

	if err := r.ensureBucket(); err != nil {
		return false, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	exists := false

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		exists = b.Get([]byte(id)) != nil
		return nil
	})

	return exists, err
}

// FindByFieldWithPagination 按字段查找并支持分页
func (r *BoltRepository[T]) FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error) {
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

	allMatches, err := r.FindByField(ctx, field, value)
	if err != nil {
		return nil, err
	}

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
func (r *BoltRepository[T]) CountByField(ctx context.Context, field string, value interface{}) (int64, error) {
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
func (r *BoltRepository[T]) Random(ctx context.Context, take int) ([]T, error) {
	if take <= 0 {
		take = 1
	}

	if err := r.ensureBucket(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var ids []string

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			ids = append(ids, string(k))
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return []T{}, nil
	}

	utils.ShuffleStrings(ids)
	if len(ids) > take {
		ids = ids[:take]
	}

	return r.FindByIDs(ctx, ids)
}

// FindWithConditions 根据条件查询
func (r *BoltRepository[T]) FindWithConditions(ctx context.Context, options usecase.QueryOptions) ([]T, error) {
	if options.Limit <= 0 {
		options.Limit = 10
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	if err := r.ensureBucket(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var entities []T

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var entity T
			if err := json.Unmarshal(v, &entity); err != nil {
				return fmt.Errorf("failed to unmarshal entity: %w", err)
			}

			if r.matchConditions(entity, options.Conditions) {
				entities = append(entities, entity)
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	r.sortEntities(entities, options.SortFields)

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
func (r *BoltRepository[T]) CountWithConditions(ctx context.Context, conditions []usecase.QueryCondition) (int64, error) {
	if err := r.ensureBucket(); err != nil {
		return 0, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	var count int64

	err := r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucketName)
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var entity T
			if err := json.Unmarshal(v, &entity); err != nil {
				return fmt.Errorf("failed to unmarshal entity: %w", err)
			}

			if r.matchConditions(entity, conditions) {
				count++
			}

			return nil
		})
	})

	return count, err
}

// matchConditions 检查实体是否符合所有条件
func (r *BoltRepository[T]) matchConditions(entity T, conditions []usecase.QueryCondition) bool {
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
func (r *BoltRepository[T]) compareField(fieldValue interface{}, operator string, conditionValue interface{}) bool {
	fieldValueStr := fmt.Sprintf("%v", fieldValue)

	switch operator {
	case "EQ":
		return fieldValueStr == fmt.Sprintf("%v", conditionValue)
	case "NEQ":
		return fieldValueStr != fmt.Sprintf("%v", conditionValue)
	case "LIKE":
		return strings.Contains(fieldValueStr, fmt.Sprintf("%v", conditionValue))
	case "IN":
		fieldReflectValue := reflect.ValueOf(fieldValue)
		if fieldReflectValue.Kind() == reflect.Slice || fieldReflectValue.Kind() == reflect.Array {
			for i := 0; i < fieldReflectValue.Len(); i++ {
				fieldElement := fmt.Sprintf("%v", fieldReflectValue.Index(i).Interface())
				if r.isValueInSlice(fieldElement, conditionValue) {
					return true
				}
			}
			return false
		}
		return r.isValueInSlice(fieldValueStr, conditionValue)
	case "GT":
		return r.compareNumeric(fieldValue, conditionValue, func(f, c float64) bool { return f > c })
	case "GTE":
		return r.compareNumeric(fieldValue, conditionValue, func(f, c float64) bool { return f >= c })
	case "LT":
		return r.compareNumeric(fieldValue, conditionValue, func(f, c float64) bool { return f < c })
	case "LTE":
		return r.compareNumeric(fieldValue, conditionValue, func(f, c float64) bool { return f <= c })
	default:
		return false
	}
}

// compareNumeric 比较数值类型的值
func (r *BoltRepository[T]) compareNumeric(fieldValue interface{}, conditionValue interface{}, compareFunc func(float64, float64) bool) bool {
	fieldFloat, err := r.toFloat64(fieldValue)
	if err != nil {
		return false
	}

	conditionFloat, err := r.toFloat64(conditionValue)
	if err != nil {
		return false
	}

	return compareFunc(fieldFloat, conditionFloat)
}

// toFloat64 将值转换为 float64
func (r *BoltRepository[T]) toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		strValue := fmt.Sprintf("%v", value)
		return strconv.ParseFloat(strValue, 64)
	}
}

// isValueInSlice 检查值是否在切片中
func (r *BoltRepository[T]) isValueInSlice(value string, slice interface{}) bool {
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
func (r *BoltRepository[T]) sortEntities(entities []T, sortFields []usecase.SortField) {
	utils.SortEntities(entities, sortFields, false)
}

// Close 关闭数据库连接
func (r *BoltRepository[T]) Close() error {
	return r.db.Close()
}
