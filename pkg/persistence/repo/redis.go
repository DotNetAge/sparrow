package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"

	"github.com/redis/go-redis/v9"
)

// RedisRepository Redis仓储实现
// 基于BaseRepository[T]的完整Redis实现
// 支持泛型实体类型，提供完整的CRUD操作

type RedisRepository[T entity.Entity] struct {
	usecase.BaseRepository[T]
	client     *redis.Client
	prefix     string
	entityType string
	ttl        time.Duration // 过期时间，0表示永不过期
}

var _ usecase.Repository[entity.Entity] = (*RedisRepository[entity.Entity])(nil)

// NewRedisRepository 创建Redis仓储实例
// 参数:
//   - client: Redis客户端
//   - prefix: 键前缀，用于区分不同类型的实体
//   - ttl: 过期时间，0表示永不过期
//
// 返回: 初始化的Redis仓储实例
func NewRedisRepository[T entity.Entity](client *redis.Client, prefix string, ttl time.Duration) usecase.Repository[T] {
	var zero T
	entityType := fmt.Sprintf("%T", zero)

	// 确保前缀以冒号结尾
	if prefix != "" && !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}

	return &RedisRepository[T]{
		BaseRepository: usecase.BaseRepository[T]{},
		client:         client,
		prefix:         prefix,
		entityType:     entityType,
		ttl:            ttl,
	}
}

func NewCustomRedisRepository[T entity.Entity](client *redis.Client, prefix string, entityType string, ttl time.Duration) usecase.Repository[T] {
	// 确保前缀以冒号结尾
	if prefix != "" && !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}
	return &RedisRepository[T]{
		BaseRepository: usecase.BaseRepository[T]{},
		client:         client,
		prefix:         prefix,
		entityType:     entityType,
		ttl:            ttl,
	}
}

// Save 保存实体（插入或更新）
// 如果实体ID已存在则执行更新，否则执行插入
func (r *RedisRepository[T]) Save(ctx context.Context, entity T) error {
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
func (r *RedisRepository[T]) insert(ctx context.Context, entity T) error {
	// 使用反射设置CreatedAt字段
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	createdAtField := entityValue.FieldByName("CreatedAt")
	if createdAtField.IsValid() && createdAtField.CanSet() {
		createdAtField.Set(reflect.ValueOf(time.Now()))
	}

	entity.SetUpdatedAt(time.Now())

	data, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	key := r.buildKey(entity.GetID())

	if r.ttl > 0 {
		return r.client.Set(ctx, key, data, r.ttl).Err()
	}

	return r.client.Set(ctx, key, data, 0).Err()
}

// Update 更新实体
func (r *RedisRepository[T]) Update(ctx context.Context, entity T) error {
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

	// 检查实体是否存在
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}

	if exists == 0 {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			ID:         entity.GetID(),
			Message:    "entity not found",
		}
	}

	if r.ttl > 0 {
		return r.client.Set(ctx, key, data, r.ttl).Err()
	}

	return r.client.Set(ctx, key, data, 0).Err()
}

// FindByID 根据ID查找实体
func (r *RedisRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
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

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		var zero T
		if err == redis.Nil {
			return zero, &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "find_by_id",
				ID:         id,
				Message:    "entity not found",
			}
		}
		return zero, err
	}

	var entity T
	err = json.Unmarshal([]byte(data), &entity)
	return entity, err
}

// FindAll 查找所有实体
func (r *RedisRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	pattern := r.prefix + "*"

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return []T{}, nil
	}

	// 使用MGET批量获取
	dataList, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	var entities []T
	for _, data := range dataList {
		if data == nil {
			continue
		}

		var entity T
		strData, ok := data.(string)
		if !ok {
			continue
		}

		if err := json.Unmarshal([]byte(strData), &entity); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// Delete 删除实体
func (r *RedisRepository[T]) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	key := r.buildKey(id)

	// 检查实体是否存在
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}

	if exists == 0 {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "entity not found",
		}
	}

	return r.client.Del(ctx, key).Err()
}

// SaveBatch 批量保存实体
func (r *RedisRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	// 使用pipeline批量操作
	pipe := r.client.Pipeline()
	// Redis pipeline不需要Close，执行完后会自动释放

	for _, entity := range entities {
		if entity.GetID() == "" {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "save_batch",
				Message:    "entity ID cannot be empty",
			}
		}

		entity.SetUpdatedAt(time.Now())

		// 检查是否存在
		exists, err := r.Exists(ctx, entity.GetID())
		if err != nil {
			return err
		}

		if !exists {
			// 使用反射设置CreatedAt字段
			entityValue := reflect.ValueOf(entity)
			if entityValue.Kind() == reflect.Ptr {
				entityValue = entityValue.Elem()
			}

			createdAtField := entityValue.FieldByName("CreatedAt")
			if createdAtField.IsValid() && createdAtField.CanSet() {
				createdAtField.Set(reflect.ValueOf(time.Now()))
			}
		}

		data, err := json.Marshal(entity)
		if err != nil {
			return fmt.Errorf("failed to marshal entity: %w", err)
		}

		key := r.buildKey(entity.GetID())

		if r.ttl > 0 {
			pipe.Set(ctx, key, data, r.ttl)
		} else {
			pipe.Set(ctx, key, data, 0)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// FindByIDs 根据多个ID查找实体
func (r *RedisRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = r.buildKey(id)
	}

	dataList, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	var entities []T
	for _, data := range dataList {
		if data == nil {
			continue
		}

		var entity T
		strData, ok := data.(string)
		if !ok {
			continue
		}

		if err := json.Unmarshal([]byte(strData), &entity); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// DeleteBatch 批量删除实体
func (r *RedisRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = r.buildKey(id)
	}

	_, err := r.client.Del(ctx, keys...).Result()
	return err
}

// FindWithPagination 分页查询实体
func (r *RedisRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	pattern := r.prefix + "*"

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return []T{}, nil
	}

	// 计算分页
	start := offset
	if start >= len(keys) {
		return []T{}, nil
	}

	end := start + limit
	if end > len(keys) {
		end = len(keys)
	}

	pageKeys := keys[start:end]
	if len(pageKeys) == 0 {
		return []T{}, nil
	}

	// 使用MGET批量获取
	dataList, err := r.client.MGet(ctx, pageKeys...).Result()
	if err != nil {
		return nil, err
	}

	var entities []T
	for _, data := range dataList {
		if data == nil {
			continue
		}

		var entity T
		strData, ok := data.(string)
		if !ok {
			continue
		}

		if err := json.Unmarshal([]byte(strData), &entity); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// Count 统计实体总数
func (r *RedisRepository[T]) Count(ctx context.Context) (int64, error) {
	pattern := r.prefix + "*"

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, err
	}

	return int64(len(keys)), nil
}

// FindByField 按字段查找实体
// 注意：Redis不支持索引，这是全表扫描操作
func (r *RedisRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}

	pattern := r.prefix + "*"

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return []T{}, nil
	}

	// 使用MGET批量获取
	dataList, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	var entities []T
	for _, data := range dataList {
		if data == nil {
			continue
		}

		var entity T
		strData, ok := data.(string)
		if !ok {
			continue
		}

		if err := json.Unmarshal([]byte(strData), &entity); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
		}

		// 使用反射获取字段值
		entityValue := reflect.ValueOf(entity)
		if entityValue.Kind() == reflect.Ptr {
			entityValue = entityValue.Elem()
		}

		fieldValue := entityValue.FieldByName(field)
		if !fieldValue.IsValid() {
			continue // 跳过字段不存在的实体
		}

		// 比较字段值
		if reflect.DeepEqual(fieldValue.Interface(), value) {
			entities = append(entities, entity)
		}
	}

	return entities, nil
}

// Exists 检查实体是否存在
func (r *RedisRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}

	key := r.buildKey(id)

	exists, err := r.client.Exists(ctx, key).Result()
	return exists > 0, err
}

// buildKey 构建存储键
func (r *RedisRepository[T]) buildKey(id string) string {
	return r.prefix + id
}

// SetTTL 设置过期时间
func (r *RedisRepository[T]) SetTTL(ttl time.Duration) {
	r.ttl = ttl
}

// GetTTL 获取当前过期时间
func (r *RedisRepository[T]) GetTTL() time.Duration {
	return r.ttl
}

// SetTTLForKey 为特定键设置过期时间
func (r *RedisRepository[T]) SetTTLForKey(ctx context.Context, id string, ttl time.Duration) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "set_ttl",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	key := r.buildKey(id)
	return r.client.Expire(ctx, key, ttl).Err()
}

// GetTTLForKey 获取特定键的剩余过期时间
func (r *RedisRepository[T]) GetTTLForKey(ctx context.Context, id string) (time.Duration, error) {
	if id == "" {
		return 0, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "get_ttl",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	key := r.buildKey(id)
	return r.client.TTL(ctx, key).Result()
}

// 注意：Redis不支持以下PostgreSQL特有的方法
// - HardDelete（所有删除都是硬删除）
// - FindDeleted（没有软删除概念）
// - Restore（没有软删除概念）

// FindByFieldWithPagination 按字段查找并支持分页
// 注意：Redis不支持索引，这是全表扫描操作
func (r *RedisRepository[T]) FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error) {
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
	allEntities, err := r.FindByField(ctx, field, value)
	if err != nil {
		return nil, err
	}

	// 执行分页
	start := offset
	if start >= len(allEntities) {
		return []T{}, nil
	}

	end := start + limit
	if end > len(allEntities) {
		end = len(allEntities)
	}

	return allEntities[start:end], nil
}

// CountByField 按字段统计数量
// 注意：Redis不支持索引，这是全表扫描操作
func (r *RedisRepository[T]) CountByField(ctx context.Context, field string, value interface{}) (int64, error) {
	entities, err := r.FindByField(ctx, field, value)
	if err != nil {
		return 0, err
	}

	return int64(len(entities)), nil
}

// FindWithConditions 根据条件查询
// 注意：Redis不支持索引，这是全表扫描操作
func (r *RedisRepository[T]) FindWithConditions(ctx context.Context, options usecase.QueryOptions) ([]T, error) {
	if options.Limit <= 0 {
		options.Limit = 10
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	// 获取所有实体
	allEntities, err := r.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	// 应用条件过滤
	filteredEntities := make([]T, 0)
	for _, entity := range allEntities {
		if r.matchConditions(entity, options.Conditions) {
			filteredEntities = append(filteredEntities, entity)
		}
	}

	// 应用排序
	r.sortEntities(filteredEntities, options.SortFields)

	// 执行分页
	start := options.Offset
	if start >= len(filteredEntities) {
		return []T{}, nil
	}

	end := start + options.Limit
	if end > len(filteredEntities) {
		end = len(filteredEntities)
	}

	return filteredEntities[start:end], nil
}

// CountWithConditions 根据条件统计
// 注意：Redis不支持索引，这是全表扫描操作
func (r *RedisRepository[T]) CountWithConditions(ctx context.Context, conditions []usecase.QueryCondition) (int64, error) {
	// 获取所有实体
	allEntities, err := r.FindAll(ctx)
	if err != nil {
		return 0, err
	}

	// 应用条件过滤并计数
	count := 0
	for _, entity := range allEntities {
		if r.matchConditions(entity, conditions) {
			count++
		}
	}

	return int64(count), nil
}

// Random 返回随机实体
func (r *RedisRepository[T]) Random(ctx context.Context, take int) ([]T, error) {
	if take <= 0 {
		take = 1
	}

	// 获取所有实体的键
	pattern := r.prefix + "*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return []T{}, nil
	}

	// 提取ID并打乱
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		id := strings.TrimPrefix(key, r.prefix)
		ids = append(ids, id)
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

	// 根据选择的ID获取实体
	return r.FindByIDs(ctx, ids)
}

// matchConditions 检查实体是否满足所有条件
func (r *RedisRepository[T]) matchConditions(entity T, conditions []usecase.QueryCondition) bool {
	if len(conditions) == 0 {
		return true
	}

	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	for _, condition := range conditions {
		fieldValue := r.getFieldValue(entityValue, condition.Field)
		if !fieldValue.IsValid() {
			return false
		}

		if !r.compareField(fieldValue.Interface(), condition.Value, condition.Operator) {
			return false
		}
	}

	return true
}

// compareField 比较字段值和条件值
func (r *RedisRepository[T]) compareField(fieldValue, conditionValue interface{}, operator string) bool {
	switch operator {
	case "EQ":
		return reflect.DeepEqual(fieldValue, conditionValue)
	case "NEQ":
		return !reflect.DeepEqual(fieldValue, conditionValue)
	case "GT":
		return r.compareValues(fieldValue, conditionValue, func(a, b float64) bool { return a > b })
	case "GTE":
		return r.compareValues(fieldValue, conditionValue, func(a, b float64) bool { return a >= b })
	case "LT":
		return r.compareValues(fieldValue, conditionValue, func(a, b float64) bool { return a < b })
	case "LTE":
		return r.compareValues(fieldValue, conditionValue, func(a, b float64) bool { return a <= b })
	case "LIKE":
		if str1, ok1 := fieldValue.(string); ok1 {
			if str2, ok2 := conditionValue.(string); ok2 {
				return strings.Contains(str1, str2)
			}
		}
		return false
	case "IN":
		if slice, ok := conditionValue.([]interface{}); ok {
			for _, item := range slice {
				if reflect.DeepEqual(fieldValue, item) {
					return true
				}
			}
		}
		return false
	case "IS_NULL":
		return fieldValue == nil || reflect.ValueOf(fieldValue).IsZero()
	case "IS_NOT_NULL":
		return fieldValue != nil && !reflect.ValueOf(fieldValue).IsZero()
	default:
		return false
	}
}

// compareValues 辅助方法，将值转换为float64进行比较
func (r *RedisRepository[T]) compareValues(a, b interface{}, comparator func(float64, float64) bool) bool {
	floatA, okA := r.convertToFloat(a)
	floatB, okB := r.convertToFloat(b)

	if !okA || !okB {
		return false
	}

	return comparator(floatA, floatB)
}

// convertToFloat 尝试将值转换为float64
func (r *RedisRepository[T]) convertToFloat(value interface{}) (float64, bool) {
	v := reflect.ValueOf(value)
	v = reflect.Indirect(v)

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	case reflect.String:
		f, err := strconv.ParseFloat(v.String(), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// getFieldValue 获取实体字段值
func (r *RedisRepository[T]) getFieldValue(entityValue reflect.Value, fieldName string) reflect.Value {
	fieldValue := entityValue.FieldByName(fieldName)
	if fieldValue.IsValid() {
		return fieldValue
	}

	// 尝试查找带有json标签的字段
	for i := 0; i < entityValue.NumField(); i++ {
		field := entityValue.Type().Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			// 处理逗号分隔的json标签（如 "id,omitempty"）
			tagParts := strings.Split(jsonTag, ",")
			if len(tagParts) > 0 && tagParts[0] == fieldName {
				return entityValue.Field(i)
			}
		}
	}

	return reflect.Value{}
}

// sortEntities 对实体进行排序
// 使用通用的排序工具函数，简化排序逻辑
func (r *RedisRepository[T]) sortEntities(entities []T, sortFields []usecase.SortField) {
	utils.SortEntities(entities, sortFields, true)
}
