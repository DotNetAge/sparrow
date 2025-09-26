package repo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// MongoDBRepository MongoDB仓储实现
// 基于BaseRepository[T]的完整MongoDB实现
// 支持泛型实体类型，提供完整的CRUD操作
type MongoDBRepository[T entity.Entity] struct {
	*usecase.BaseRepository[T] // 修改为指针类型以匹配NewBaseRepository的返回值
	client                     *mongo.Client
	dbName                     string
	collection                 string
	entityType                 string
	model                      T
}

var _ usecase.Repository[entity.Entity] = (*MongoDBRepository[entity.Entity])(nil)

// NewMongoDBRepository 创建MongoDB仓储实例
// 参数:
//   - client: MongoDB客户端连接
//   - dbName: 数据库名称
//
// 返回: 初始化的MongoDB仓储实例
func NewMongoDBRepository[T entity.Entity](client *mongo.Client, dbName string) *MongoDBRepository[T] {
	var model T
	entityType := fmt.Sprintf("%T", model)

	// 默认集合名为实体类型名的小写形式
	collectionName := strings.ToLower(strings.ReplaceAll(entityType, ".", "_"))
	if lastDotIndex := strings.LastIndex(collectionName, "_"); lastDotIndex > 0 {
		collectionName = collectionName[lastDotIndex+1:]
	}

	return &MongoDBRepository[T]{
		BaseRepository: usecase.NewBaseRepository[T](),
		client:         client,
		dbName:         dbName,
		collection:     collectionName,
		entityType:     entityType,
		model:          model,
	}
}

// 获取集合
func (r *MongoDBRepository[T]) getCollection() *mongo.Collection {
	return r.client.Database(r.dbName).Collection(r.collection)
}

// Save 保存实体（插入或更新）
// 如果实体ID已存在则执行更新，否则执行插入
func (r *MongoDBRepository[T]) Save(ctx context.Context, entity T) error {
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

	collection := r.getCollection()

	// 转换ID为ObjectID
	objectID, err := primitive.ObjectIDFromHex(entity.GetID())
	if err != nil {
		// 如果不是有效的ObjectID，可能是自定义ID格式，不进行转换
		objectID = primitive.NilObjectID
	}

	entity.SetUpdatedAt(time.Now())

	if exists {
		// 更新实体
		filter := bson.M{"_id": entity.GetID()}
		if objectID != primitive.NilObjectID {
			filter = bson.M{"_id": objectID}
		}

		update := bson.M{"$set": entity}
		_, err = collection.UpdateOne(ctx, filter, update)
	} else {
		// 新实体设置创建时间
		entityValue := reflect.ValueOf(entity)
		if entityValue.Kind() == reflect.Ptr {
			entityValue = entityValue.Elem()
		}

		createdAtField := entityValue.FieldByName("CreatedAt")
		if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
			createdAtField.Set(reflect.ValueOf(time.Now()))
		}

		// 插入实体
		if objectID != primitive.NilObjectID {
			// 如果是有效的ObjectID，使用ID字段
			doc := make(map[string]interface{})
			doc["_id"] = objectID

			// 遍历实体字段
			entityElem := entityValue
			entityType := entityElem.Type()
			for i := 0; i < entityType.NumField(); i++ {
				field := entityType.Field(i)
				// 跳过ID字段
				if strings.EqualFold(field.Name, "ID") || strings.EqualFold(field.Name, "Id") {
					continue
				}
				doc[field.Name] = entityElem.Field(i).Interface()
			}

			_, err = collection.InsertOne(ctx, doc)
		} else {
			// 使用自定义ID
			_, err = collection.InsertOne(ctx, entity)
		}
	}

	return err
}

// FindByID 根据ID查找实体
func (r *MongoDBRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	var entity T

	if id == "" {
		return entity, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	collection := r.getCollection()

	// 转换ID为ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": id}
	if err == nil {
		// 如果是有效的ObjectID，使用ObjectID查询
		filter = bson.M{"_id": objectID}
	}

	result := collection.FindOne(ctx, filter)
	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			return entity, &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "find_by_id",
				ID:         id,
				Message:    "entity not found",
			}
		}
		return entity, result.Err()
	}

	// 解码结果
	err = result.Decode(&entity)
	return entity, err
}

// FindAll 查找所有实体
func (r *MongoDBRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	var entities []T
	collection := r.getCollection()

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &entities); err != nil {
		return nil, err
	}

	return entities, nil
}

// Update 更新实体
func (r *MongoDBRepository[T]) Update(ctx context.Context, entity T) error {
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
	collection := r.getCollection()

	// 转换ID为ObjectID
	objectID, err := primitive.ObjectIDFromHex(entity.GetID())
	filter := bson.M{"_id": entity.GetID()}
	if err == nil {
		filter = bson.M{"_id": objectID}
	}

	update := bson.M{"$set": entity}
	_, err = collection.UpdateOne(ctx, filter, update)
	return err
}

// Delete 删除实体
func (r *MongoDBRepository[T]) Delete(ctx context.Context, id string) error {
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

	collection := r.getCollection()

	// 转换ID为ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": id}
	if err == nil {
		filter = bson.M{"_id": objectID}
	}

	_, err = collection.DeleteOne(ctx, filter)
	return err
}

// SaveBatch 批量保存实体
func (r *MongoDBRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
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

	collection := r.getCollection()

	// 使用事务批量处理
	session, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	return mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		// 开始事务
		if err := session.StartTransaction(); err != nil {
			return err
		}

		for _, entity := range entities {
			// 检查是否存在
			exists, err := r.existsInSession(sc, entity.GetID())
			if err != nil {
				return err
			}

			entity.SetUpdatedAt(time.Now())

			if exists {
				// 更新实体
				objectID, _ := primitive.ObjectIDFromHex(entity.GetID())
				filter := bson.M{"_id": entity.GetID()}
				if objectID != primitive.NilObjectID {
					filter = bson.M{"_id": objectID}
				}

				update := bson.M{"$set": entity}
				if _, err := collection.UpdateOne(sc, filter, update); err != nil {
					return err
				}
			} else {
				// 新实体设置创建时间
				entityValue := reflect.ValueOf(entity)
				if entityValue.Kind() == reflect.Ptr {
					entityValue = entityValue.Elem()
				}

				createdAtField := entityValue.FieldByName("CreatedAt")
				if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
					createdAtField.Set(reflect.ValueOf(time.Now()))
				}

				// 插入实体
				if _, err := collection.InsertOne(sc, entity); err != nil {
					return err
				}
			}
		}

		// 提交事务
		return session.CommitTransaction(sc)
	})
}

// FindByIDs 根据多个ID查找实体
func (r *MongoDBRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	var entities []T

	if len(ids) == 0 {
		return entities, nil
	}

	collection := r.getCollection()

	// 处理ID列表，支持ObjectID和自定义ID
	var objectIDs []primitive.ObjectID
	var customIDs []string

	for _, id := range ids {
		if objectID, err := primitive.ObjectIDFromHex(id); err == nil {
			objectIDs = append(objectIDs, objectID)
		} else {
			customIDs = append(customIDs, id)
		}
	}

	// 构建查询条件
	var filter bson.M
	if len(objectIDs) > 0 && len(customIDs) > 0 {
		filter = bson.M{
			"$or": []bson.M{
				{"_id": bson.M{"$in": objectIDs}},
				{"_id": bson.M{"$in": customIDs}},
			},
		}
	} else if len(objectIDs) > 0 {
		filter = bson.M{"_id": bson.M{"$in": objectIDs}}
	} else {
		filter = bson.M{"_id": bson.M{"$in": customIDs}}
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &entities); err != nil {
		return nil, err
	}

	return entities, nil
}

// DeleteBatch 批量删除实体
func (r *MongoDBRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
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

	collection := r.getCollection()

	// 处理ID列表，支持ObjectID和自定义ID
	var objectIDs []primitive.ObjectID
	var customIDs []string

	for _, id := range ids {
		if objectID, err := primitive.ObjectIDFromHex(id); err == nil {
			objectIDs = append(objectIDs, objectID)
		} else {
			customIDs = append(customIDs, id)
		}
	}

	// 构建查询条件
	var filter bson.M
	if len(objectIDs) > 0 && len(customIDs) > 0 {
		filter = bson.M{
			"$or": []bson.M{
				{"_id": bson.M{"$in": objectIDs}},
				{"_id": bson.M{"$in": customIDs}},
			},
		}
	} else if len(objectIDs) > 0 {
		filter = bson.M{"_id": bson.M{"$in": objectIDs}}
	} else {
		filter = bson.M{"_id": bson.M{"$in": customIDs}}
	}

	_, err := collection.DeleteMany(ctx, filter)
	return err
}

// FindWithPagination 分页查询实体
func (r *MongoDBRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	var entities []T
	collection := r.getCollection()

	findOptions := options.Find()
	findOptions.SetLimit(int64(limit))
	findOptions.SetSkip(int64(offset))

	cursor, err := collection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &entities); err != nil {
		return nil, err
	}

	return entities, nil
}

// Count 统计实体总数
func (r *MongoDBRepository[T]) Count(ctx context.Context) (int64, error) {
	collection := r.getCollection()
	return collection.CountDocuments(ctx, bson.M{})
}

// FindByField 按字段查找实体
func (r *MongoDBRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}

	var entities []T
	collection := r.getCollection()

	filter := bson.M{field: value}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &entities); err != nil {
		return nil, err
	}

	return entities, nil
}

// FindByFieldWithPagination 按字段查找并支持分页
func (r *MongoDBRepository[T]) FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error) {
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
	collection := r.getCollection()

	filter := bson.M{field: value}

	findOptions := options.Find()
	findOptions.SetLimit(int64(limit))
	findOptions.SetSkip(int64(offset))

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &entities); err != nil {
		return nil, err
	}

	return entities, nil
}

// CountByField 按字段统计数量
func (r *MongoDBRepository[T]) CountByField(ctx context.Context, field string, value interface{}) (int64, error) {
	if field == "" {
		return 0, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "count_by_field",
			Message:    "field name cannot be empty",
		}
	}

	collection := r.getCollection()
	filter := bson.M{field: value}
	return collection.CountDocuments(ctx, filter)
}

// Exists 检查实体是否存在
func (r *MongoDBRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}

	collection := r.getCollection()

	// 转换ID为ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": id}
	if err == nil {
		filter = bson.M{"_id": objectID}
	}

	count, err := collection.CountDocuments(ctx, filter)
	return count > 0, err
}

// FindWithConditions 根据条件查询
func (r *MongoDBRepository[T]) FindWithConditions(ctx context.Context, queryOptions usecase.QueryOptions) ([]T, error) {
	if queryOptions.Limit <= 0 {
		queryOptions.Limit = 10
	}
	if queryOptions.Offset < 0 {
		queryOptions.Offset = 0
	}

	var entities []T
	collection := r.getCollection()

	// 构建查询条件
	filter := bson.M{}
	if len(queryOptions.Conditions) > 0 {
		filter = r.buildMongoFilter(queryOptions.Conditions)
	}

	// 设置查询选项
	findOptions := options.Find()
	findOptions.SetLimit(int64(queryOptions.Limit))
	findOptions.SetSkip(int64(queryOptions.Offset))

	// 应用排序
	if len(queryOptions.SortFields) > 0 {
		sortFields := bson.D{}
		for _, sortField := range queryOptions.SortFields {
			order := 1
			if !sortField.Ascending {
				order = -1
			}
			sortFields = append(sortFields, bson.E{Key: sortField.Field, Value: order})
		}
		findOptions.SetSort(sortFields)
	} else {
		// 默认排序
		findOptions.SetSort(bson.D{{Key: "created_at", Value: -1}})
	}

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &entities); err != nil {
		return nil, err
	}

	return entities, nil
}

// CountWithConditions 根据条件统计
func (r *MongoDBRepository[T]) CountWithConditions(ctx context.Context, conditions []usecase.QueryCondition) (int64, error) {
	collection := r.getCollection()

	// 构建查询条件
	filter := bson.M{}
	if len(conditions) > 0 {
		filter = r.buildMongoFilter(conditions)
	}

	return collection.CountDocuments(ctx, filter)
}

// 辅助方法

// existsInSession 在事务中检查实体是否存在
func (r *MongoDBRepository[T]) existsInSession(ctx mongo.SessionContext, id string) (bool, error) {
	collection := r.getCollection()

	// 转换ID为ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": id}
	if err == nil {
		filter = bson.M{"_id": objectID}
	}

	count, err := collection.CountDocuments(ctx, filter)
	return count > 0, err
}

// buildMongoFilter 根据查询条件构建MongoDB查询过滤器
func (r *MongoDBRepository[T]) buildMongoFilter(conditions []usecase.QueryCondition) bson.M {
	filter := bson.M{}

	for _, condition := range conditions {
		r.addConditionToFilter(filter, condition)
	}

	return filter
}

// addConditionToFilter 将单个查询条件添加到MongoDB过滤器
func (r *MongoDBRepository[T]) addConditionToFilter(filter bson.M, condition usecase.QueryCondition) {
	field := condition.Field
	op := condition.Operator
	value := condition.Value

	switch op {
	case "EQ":
		filter[field] = value
	case "NEQ":
		filter[field] = bson.M{"$ne": value}
	case "GT":
		filter[field] = bson.M{"$gt": value}
	case "GTE":
		filter[field] = bson.M{"$gte": value}
	case "LT":
		filter[field] = bson.M{"$lt": value}
	case "LTE":
		filter[field] = bson.M{"$lte": value}
	case "LIKE":
		filter[field] = bson.M{"$regex": fmt.Sprintf(".*%v.*", value), "$options": "i"}
	case "IN":
		filter[field] = bson.M{"$in": value}
	case "NOT_IN":
		filter[field] = bson.M{"$nin": value}
	case "IS_NULL":
		filter[field] = nil
	case "IS_NOT_NULL":
		filter[field] = bson.M{"$ne": nil}
	default:
		// 默认使用等于
		filter[field] = value
	}
}

// MongoDB特有的方法

// WithTransaction 设置事务会话
func (r *MongoDBRepository[T]) WithTransaction(session mongo.SessionContext) *MongoDBRepository[T] {
	return r
}

// GetClient 获取MongoDB客户端连接
func (r *MongoDBRepository[T]) GetClient() *mongo.Client {
	return r.client
}

// GetCollectionName 获取集合名称
func (r *MongoDBRepository[T]) GetCollectionName() string {
	return r.collection
}
