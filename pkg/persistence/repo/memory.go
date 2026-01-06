package repo

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// MemoryRepository 内存仓储实现
// 基于BaseRepository[T]的完整内存实现
// 支持泛型实体类型，提供完整的CRUD操作
// 适用于测试环境或不需要持久化的场景

type MemoryRepository[T entity.Entity] struct {
	usecase.BaseRepository[T]
	mu         sync.RWMutex
	entities   map[string]T
	entityType string
}

var _ usecase.Repository[entity.Entity] = (*MemoryRepository[entity.Entity])(nil)

// NewMemoryRepository 创建内存仓储实例
// 返回: 初始化的内存仓储实例
func NewMemoryRepository[T entity.Entity]() usecase.Repository[T] {
	var zero T
	entityType := fmt.Sprintf("%T", zero)

	return &MemoryRepository[T]{
		BaseRepository: usecase.BaseRepository[T]{},
		entities:       make(map[string]T),
		entityType:     entityType,
	}
}

// Save 保存实体（插入或更新）
// 如果实体ID已存在则执行更新，否则执行插入
func (r *MemoryRepository[T]) Save(ctx context.Context, entity T) error {
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
func (r *MemoryRepository[T]) insert(ctx context.Context, entity T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 再次检查，防止并发问题
	if _, exists := r.entities[entity.GetID()]; exists {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "insert",
			ID:         entity.GetID(),
			Message:    "entity already exists",
		}
	}

	// 设置创建和更新时间
	t := time.Now()
	env := reflect.ValueOf(entity)
	if env.Kind() == reflect.Ptr {
		env = env.Elem()
	}

	// 尝试设置CreatedAt字段
	createdAtField := env.FieldByName("CreatedAt")
	if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
		createdAtField.Set(reflect.ValueOf(t))
	}

	// 设置UpdatedAt字段
	updatedAtField := env.FieldByName("UpdatedAt")
	if updatedAtField.IsValid() && updatedAtField.CanSet() && updatedAtField.Type() == reflect.TypeOf(time.Time{}) {
		updatedAtField.Set(reflect.ValueOf(t))
	}

	// 保存实体
	r.entities[entity.GetID()] = entity

	return nil
}

// Update 更新实体
func (r *MemoryRepository[T]) Update(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			Message:    "entity ID cannot be empty",
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查实体是否存在
	if _, exists := r.entities[entity.GetID()]; !exists {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			ID:         entity.GetID(),
			Message:    "entity not found",
		}
	}

	// 设置更新时间
	t := time.Now()
	env := reflect.ValueOf(entity)
	if env.Kind() == reflect.Ptr {
		env = env.Elem()
	}

	updatedAtField := env.FieldByName("UpdatedAt")
	if updatedAtField.IsValid() && updatedAtField.CanSet() && updatedAtField.Type() == reflect.TypeOf(time.Time{}) {
		updatedAtField.Set(reflect.ValueOf(t))
	}

	// 更新实体
	r.entities[entity.GetID()] = entity

	return nil
}

// FindByID 根据ID查找实体
func (r *MemoryRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	if id == "" {
		var zero T
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, exists := r.entities[id]
	if !exists {
		var zero T
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "entity not found",
		}
	}

	return entity, nil
}

// FindAll 查找所有实体
func (r *MemoryRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entities := make([]T, 0, len(r.entities))
	for _, entity := range r.entities {
		entities = append(entities, entity)
	}

	return entities, nil
}

// Delete 删除实体
func (r *MemoryRepository[T]) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entities[id]; !exists {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "entity not found",
		}
	}

	delete(r.entities, id)

	return nil
}

// SaveBatch 批量保存实体
func (r *MemoryRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	t := time.Now()
	for _, entity := range entities {
		if entity.GetID() == "" {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "save_batch",
				Message:    "entity ID cannot be empty",
			}
		}

		// 设置时间字段
		env := reflect.ValueOf(entity)
		if env.Kind() == reflect.Ptr {
			env = env.Elem()
		}

		// 检查是否存在
		exists := false
		if _, ok := r.entities[entity.GetID()]; ok {
			exists = true
		}

		// 设置创建时间（如果是新实体）
		if !exists {
			createdAtField := env.FieldByName("CreatedAt")
			if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
				createdAtField.Set(reflect.ValueOf(t))
			}
		}

		// 设置更新时间
		updatedAtField := env.FieldByName("UpdatedAt")
		if updatedAtField.IsValid() && updatedAtField.CanSet() && updatedAtField.Type() == reflect.TypeOf(time.Time{}) {
			updatedAtField.Set(reflect.ValueOf(t))
		}

		r.entities[entity.GetID()] = entity
	}

	return nil
}

// FindByIDs 批量查找实体
func (r *MemoryRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	entities := make([]T, 0, len(ids))
	for _, id := range ids {
		if entity, exists := r.entities[id]; exists {
			entities = append(entities, entity)
		}
	}

	return entities, nil
}

// DeleteBatch 批量删除实体
func (r *MemoryRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range ids {
		delete(r.entities, id)
	}

	return nil
}

// FindWithPagination 分页查询实体
func (r *MemoryRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 准备所有实体的切片
	allEntities := make([]T, 0, len(r.entities))
	for _, entity := range r.entities {
		allEntities = append(allEntities, entity)
	}

	// 处理分页参数
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// 检查边界
	if offset >= len(allEntities) {
		return []T{}, nil
	}

	// 计算结束索引
	end := offset + limit
	if end > len(allEntities) {
		end = len(allEntities)
	}

	// 返回分页结果
	return allEntities[offset:end], nil
}

// Count 统计实体数量
func (r *MemoryRepository[T]) Count(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return int64(len(r.entities)), nil
}

// FindByField 按字段查找实体
func (r *MemoryRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []T

	for _, entity := range r.entities {
		// 使用反射获取字段值
		env := reflect.ValueOf(entity)
		if env.Kind() == reflect.Ptr {
			env = env.Elem()
		}

		fieldValue := env.FieldByName(field)
		if !fieldValue.IsValid() {
			continue
		}

		// 处理指针类型
		if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
			fieldValue = fieldValue.Elem()
		}

		// 比较值
		valueValue := reflect.ValueOf(value)

		// 如果类型不一致，尝试转换
		if fieldValue.Type() != valueValue.Type() {
			// 简单类型转换支持
			if fieldValue.Kind() == reflect.Int && valueValue.Kind() == reflect.Float64 {
				if int(fieldValue.Int()) == int(valueValue.Float()) {
					result = append(result, entity)
				}
			} else if fieldValue.Kind() == reflect.Float64 && valueValue.Kind() == reflect.Int {
				if fieldValue.Float() == float64(valueValue.Int()) {
					result = append(result, entity)
				}
			} else if fieldValue.Kind() == reflect.String && valueValue.Kind() == reflect.String {
				if fieldValue.String() == valueValue.String() {
					result = append(result, entity)
				}
			} else if fieldValue.Kind() == reflect.Bool && valueValue.Kind() == reflect.Bool {
				if fieldValue.Bool() == valueValue.Bool() {
					result = append(result, entity)
				}
			}
		} else if reflect.DeepEqual(fieldValue.Interface(), value) {
			result = append(result, entity)
		}
	}

	return result, nil
}

// Exists 检查实体是否存在
func (r *MemoryRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "exists",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.entities[id]
	return exists, nil
}

// Clear 清空所有实体（测试用）
func (r *MemoryRepository[T]) Clear(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entities = make(map[string]T)

	return nil
}

// FindWithConditions 根据条件查询实体
func (r *MemoryRepository[T]) FindWithConditions(ctx context.Context, options usecase.QueryOptions) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []T

	// 遍历所有实体
	for _, entity := range r.entities {
		// 检查是否满足所有条件
		match := true
		for _, condition := range options.Conditions {
			if !r.matchCondition(entity, condition) {
				match = false
				break
			}
		}

		// 如果满足所有条件，则添加到结果集
		if match {
			result = append(result, entity)
		}
	}

	return result, nil
}

// CountWithConditions 根据条件统计实体数量
func (r *MemoryRepository[T]) CountWithConditions(ctx context.Context, conditions []usecase.QueryCondition) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64 = 0

	// 遍历所有实体
	for _, entity := range r.entities {
		// 检查是否满足所有条件
		match := true
		for _, condition := range conditions {
			if !r.matchCondition(entity, condition) {
				match = false
				break
			}
		}

		// 如果满足所有条件，则增加计数
		if match {
			count++
		}
	}

	return count, nil
}

// Random 返回随机实体
func (r *MemoryRepository[T]) Random(ctx context.Context, take int) ([]T, error) {
	if take <= 0 {
		take = 1
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 获取所有实体的ID
	ids := make([]string, 0, len(r.entities))
	for id := range r.entities {
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return []T{}, nil
	}

	// 随机打乱ID列表
	for i := len(ids) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		ids[i], ids[j] = ids[j], ids[i]
	}

	// 选择前take个ID
	if len(ids) > take {
		ids = ids[:take]
	}

	// 根据ID获取实体
	entities := make([]T, 0, len(ids))
	for _, id := range ids {
		if entity, exists := r.entities[id]; exists {
			entities = append(entities, entity)
		}
	}

	return entities, nil
}

// matchCondition 检查实体是否满足单个查询条件
func (r *MemoryRepository[T]) matchCondition(entity T, condition usecase.QueryCondition) bool {
	// 使用反射获取实体字段值
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	fieldValue := entityValue.FieldByName(condition.Field)
	if !fieldValue.IsValid() || !fieldValue.CanInterface() {
		return false
	}

	// 获取字段的具体值
	fieldInterface := fieldValue.Interface()

	// 根据操作符进行比较
	switch condition.Operator {
	case "EQ", "eq":
		// 相等比较
		return reflect.DeepEqual(fieldInterface, condition.Value)
	case "NE", "ne":
		// 不等比较
		return !reflect.DeepEqual(fieldInterface, condition.Value)
	case "GT", "gt":
		// 大于比较
		return r.compareValues(fieldInterface, condition.Value, func(a, b interface{}) bool {
			aValue := reflect.ValueOf(a)
			bValue := reflect.ValueOf(b)

			switch aValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return aValue.Int() > bValue.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return aValue.Uint() > bValue.Uint()
			case reflect.Float32, reflect.Float64:
				return aValue.Float() > bValue.Float()
			case reflect.String:
				return aValue.String() > bValue.String()
			case reflect.Struct:
				// 检查是否是time.Time类型
				aType := aValue.Type()
				if aType == reflect.TypeOf(time.Time{}) {
					aTime := aValue.Interface().(time.Time)
					bTime := bValue.Interface().(time.Time)
					return aTime.After(bTime)
				}
				return false
			default:
				return false
			}
		})
	case "LT", "lt":
		// 小于比较
		return r.compareValues(fieldInterface, condition.Value, func(a, b interface{}) bool {
			aValue := reflect.ValueOf(a)
			bValue := reflect.ValueOf(b)

			switch aValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return aValue.Int() < bValue.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return aValue.Uint() < bValue.Uint()
			case reflect.Float32, reflect.Float64:
				return aValue.Float() < bValue.Float()
			case reflect.String:
				return aValue.String() < bValue.String()
			case reflect.Struct:
				// 检查是否是time.Time类型
				aType := aValue.Type()
				if aType == reflect.TypeOf(time.Time{}) {
					aTime := aValue.Interface().(time.Time)
					bTime := bValue.Interface().(time.Time)
					return aTime.Before(bTime)
				}
				return false
			default:
				return false
			}
		})
	case "GTE", "gte":
		// 大于等于比较
		return r.compareValues(fieldInterface, condition.Value, func(a, b interface{}) bool {
			aValue := reflect.ValueOf(a)
			bValue := reflect.ValueOf(b)

			switch aValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return aValue.Int() >= bValue.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return aValue.Uint() >= bValue.Uint()
			case reflect.Float32, reflect.Float64:
				return aValue.Float() >= bValue.Float()
			case reflect.String:
				return aValue.String() >= bValue.String()
			case reflect.Struct:
				// 检查是否是time.Time类型
				aType := aValue.Type()
				if aType == reflect.TypeOf(time.Time{}) {
					aTime := aValue.Interface().(time.Time)
					bTime := bValue.Interface().(time.Time)
					return aTime.After(bTime) || aTime.Equal(bTime)
				}
				return false
			default:
				return false
			}
		})
	case "LTE", "lte":
		// 小于等于比较
		return r.compareValues(fieldInterface, condition.Value, func(a, b interface{}) bool {
			aValue := reflect.ValueOf(a)
			bValue := reflect.ValueOf(b)

			switch aValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return aValue.Int() <= bValue.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return aValue.Uint() <= bValue.Uint()
			case reflect.Float32, reflect.Float64:
				return aValue.Float() <= bValue.Float()
			case reflect.String:
				return aValue.String() <= bValue.String()
			case reflect.Struct:
				// 检查是否是time.Time类型
				aType := aValue.Type()
				if aType == reflect.TypeOf(time.Time{}) {
					aTime := aValue.Interface().(time.Time)
					bTime := bValue.Interface().(time.Time)
					return aTime.Before(bTime) || aTime.Equal(bTime)
				}
				return false
			default:
				return false
			}
		})
	default:
		// 不支持的操作符
		return false
	}
}

// compareValues 比较两个值，使用提供的比较函数
func (r *MemoryRepository[T]) compareValues(a, b interface{}, compareFunc func(a, b interface{}) bool) bool {
	// 检查类型是否相同
	aType := reflect.TypeOf(a)
	bType := reflect.TypeOf(b)

	// 如果类型不同，尝试进行简单转换
	if aType != bType {
		// 对于一些常见的数值类型转换
		switch aType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if bType.Kind() == reflect.Float64 {
				// int 转 float64 比较
				aInt := reflect.ValueOf(a).Int()
				bFloat := reflect.ValueOf(b).Float()
				return float64(aInt) > bFloat
			}
		case reflect.Float64, reflect.Float32:
			if bType.Kind() == reflect.Int {
				// float 转 int 比较
				aFloat := reflect.ValueOf(a).Float()
				bInt := reflect.ValueOf(b).Int()
				return aFloat > float64(bInt)
			}
		default:
			// 不支持的类型转换
			return false
		}
	}

	// 类型相同，直接比较
	return compareFunc(a, b)
}
