package repo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	redis_client "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	redis_container "github.com/testcontainers/testcontainers-go/modules/redis"
)

// 创建一个复杂的测试实体类
// 包含嵌套结构、数组、各种数据类型等

type ComplexEntity struct {
	entity.BaseEntity
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Priority    int                    `json:"priority"`
	Score       float64                `json:"score"`
	Tags        []string               `json:"tags"`
	Metadata    map[string]interface{} `json:"metadata"`
	Active      bool                   `json:"active"`
	ExpiresAt   time.Time              `json:"expires_at"`
	Nested      *NestedEntity          `json:"nested,omitempty"`
}

type NestedEntity struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Count int    `json:"count"`
}

// 实现Entity接口
func (e *ComplexEntity) GetID() string {
	return e.Id
}

func (e *ComplexEntity) SetID(id string) {
	e.Id = id
}

func (e *ComplexEntity) GetCreatedAt() time.Time {
	return e.CreatedAt
}

func (e *ComplexEntity) SetCreatedAt(t time.Time) {
	e.CreatedAt = t
}

func (e *ComplexEntity) GetUpdatedAt() time.Time {
	return e.UpdatedAt
}

func (e *ComplexEntity) SetUpdatedAt(t time.Time) {
	e.UpdatedAt = t
}

// 测试用辅助函数
func generateComplexEntityID(idx int) string {
	return fmt.Sprintf("complex-%d", idx)
}

func createComplexEntity(idx int) *ComplexEntity {
	now := time.Now().UTC()
	entity := &ComplexEntity{
		BaseEntity: entity.BaseEntity{
		Id:        generateComplexEntityID(idx),
		CreatedAt: now,
		UpdatedAt: now,
	},
		Name:        fmt.Sprintf("Complex Entity %d", idx),
		Description: fmt.Sprintf("This is a complex entity with index %d", idx),
		Status:      "active",
		Priority:    idx * 10,
		Score:       float64(idx) * 1.5,
		Tags:        []string{"test", fmt.Sprintf("entity-%d", idx), "complex"},
		Metadata: map[string]interface{}{
			"key1": fmt.Sprintf("value-%d", idx),
			"key2": idx * 2,
			"key3": true,
		},
		Active:    true,
		ExpiresAt: now.Add(time.Hour * 24),
		Nested: &NestedEntity{
			ID:    fmt.Sprintf("nested-%d", idx),
			Value: fmt.Sprintf("Nested Value %d", idx),
			Count: idx * 5,
		},
	}

	// 每三个实体设置不同的状态和嵌套结构
	if idx%3 == 0 {
		entity.Status = "pending"
		entity.Active = false
		entity.Nested = nil
	} else if idx%3 == 1 {
		entity.Status = "completed"
	}

	return entity
}

// setupTestRedisRepository 设置测试环境，使用testcontainers启动Redis容器
func setupTestRedisRepository(t *testing.T) (*RedisRepository[*ComplexEntity], func()) {
	t.Helper()

	// 创建Redis容器
	redisContainer, err := redis_container.RunContainer(context.Background(),
		testcontainers.WithImage("redis:6-alpine"),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}

	// 获取Redis连接字符串
	connStr, err := redisContainer.ConnectionString(context.Background())
	if err != nil {
		t.Fatalf("Failed to get Redis connection string: %v", err)
	}

	// 解析连接字符串，创建Redis客户端
	// 解析连接字符串，创建Redis客户端
	opt, err := redis_client.ParseURL(connStr)
	if err != nil {
		t.Fatalf("Failed to parse Redis URL: %v", err)
	}

	client := redis_client.NewClient(opt)

	// 测试连接
	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		t.Fatalf("Failed to ping Redis: %v", err)
	}

	// 创建Repository - 添加类型断言，因为NewRedisRepository现在返回接口类型
	repo := NewRedisRepository[*ComplexEntity](client, "test:", 0).(*RedisRepository[*ComplexEntity])

	// 创建清理函数
	cleanup := func() {
		// 删除所有测试键
		keys, err := client.Keys(context.Background(), repo.prefix+"*").Result()
		if err != nil {
			t.Logf("Warning: Failed to get keys for cleanup: %v", err)
		} else if len(keys) > 0 {
			_, err = client.Del(context.Background(), keys...).Result()
			if err != nil {
				t.Logf("Warning: Failed to delete keys during cleanup: %v", err)
			}
		}

		// 关闭Redis客户端
		if err := client.Close(); err != nil {
			t.Logf("Warning: Failed to close Redis client: %v", err)
		}

		// 停止并移除容器
		if err := redisContainer.Terminate(context.Background()); err != nil {
			t.Logf("Warning: Failed to terminate Redis container: %v", err)
		}
	}

	return repo, cleanup
}

// TestRedisRepository_Save 测试保存实体功能
func TestRedisRepository_Save(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试实体
	testEntity := createComplexEntity(1)

	// 测试保存新实体
	err := repo.Save(ctx, testEntity)
	assert.NoError(t, err)

	// 验证实体是否保存成功
	savedEntity, err := repo.FindByID(ctx, testEntity.GetID())
	assert.NoError(t, err)
	assert.NotNil(t, savedEntity)
	assert.Equal(t, testEntity.GetID(), savedEntity.GetID())
	assert.Equal(t, testEntity.Name, savedEntity.Name)
	assert.Equal(t, testEntity.Description, savedEntity.Description)
	assert.Equal(t, testEntity.Status, savedEntity.Status)
	assert.Equal(t, testEntity.Priority, savedEntity.Priority)
	assert.Equal(t, testEntity.Score, savedEntity.Score)
	assert.Equal(t, testEntity.Tags, savedEntity.Tags)
	assert.Equal(t, testEntity.Metadata["key1"], savedEntity.Metadata["key1"])
	// 处理JSON反序列化可能将整数转为float64的情况
	assert.Equal(t, fmt.Sprintf("%v", testEntity.Metadata["key2"]), fmt.Sprintf("%v", savedEntity.Metadata["key2"]))
	assert.Equal(t, testEntity.Metadata["key3"], savedEntity.Metadata["key3"])
	assert.Equal(t, testEntity.Active, savedEntity.Active)
	assert.WithinDuration(t, testEntity.ExpiresAt, savedEntity.ExpiresAt, time.Second)
	assert.Equal(t, testEntity.Nested.ID, savedEntity.Nested.ID)
	assert.Equal(t, testEntity.Nested.Value, savedEntity.Nested.Value)
	assert.Equal(t, testEntity.Nested.Count, savedEntity.Nested.Count)

	// 测试更新实体
	testEntity.Name = "Updated Complex Entity"
	testEntity.Status = "updated"
	testEntity.Priority = 999
	testEntity.Metadata["updated"] = true

	err = repo.Save(ctx, testEntity)
	assert.NoError(t, err)

	// 验证实体是否更新成功
	updatedEntity, err := repo.FindByID(ctx, testEntity.GetID())
	assert.NoError(t, err)
	assert.NotNil(t, updatedEntity)
	assert.Equal(t, "Updated Complex Entity", updatedEntity.Name)
	assert.Equal(t, "updated", updatedEntity.Status)
	assert.Equal(t, 999, updatedEntity.Priority)
	assert.Equal(t, true, updatedEntity.Metadata["updated"])

	// 测试保存空ID实体
	invalidEntity := createComplexEntity(2)
	invalidEntity.SetID("")
	err = repo.Save(ctx, invalidEntity)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestRedisRepository_FindByID 测试根据ID查找实体功能
func TestRedisRepository_FindByID(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	testEntity := createComplexEntity(1)
	err := repo.Save(ctx, testEntity)
	assert.NoError(t, err)

	// 测试查找存在的实体
	foundEntity, err := repo.FindByID(ctx, testEntity.GetID())
	assert.NoError(t, err)
	assert.NotNil(t, foundEntity)
	assert.Equal(t, testEntity.GetID(), foundEntity.GetID())

	// 测试查找不存在的实体
	notFoundEntity, err := repo.FindByID(ctx, "not-exist-id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
	assert.Nil(t, notFoundEntity)

	// 测试查找空ID实体
	emptyIDEntity, err := repo.FindByID(ctx, "")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
	assert.Nil(t, emptyIDEntity)
}

// TestRedisRepository_FindAll 测试查找所有实体功能
func TestRedisRepository_FindAll(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	entityCount := 5
	for i := 0; i < entityCount; i++ {
		testEntity := createComplexEntity(i)
		err := repo.Save(ctx, testEntity)
		assert.NoError(t, err)
	}

	// 测试查找所有实体
	allEntities, err := repo.FindAll(ctx)
	assert.NoError(t, err)
	assert.Equal(t, entityCount, len(allEntities))

	// 验证实体内容
	foundIDs := make(map[string]bool)
	for _, entity := range allEntities {
		foundIDs[entity.GetID()] = true
	}

	for i := 0; i < entityCount; i++ {
		assert.Contains(t, foundIDs, generateComplexEntityID(i))
	}

	// 测试空存储库
	emptyRepo, emptyCleanup := setupTestRedisRepository(t)
	defer emptyCleanup()

	emptyEntities, err := emptyRepo.FindAll(ctx)
	assert.NoError(t, err)
	assert.Empty(t, emptyEntities)
}

// TestRedisRepository_Delete 测试删除实体功能
func TestRedisRepository_Delete(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	testEntity := createComplexEntity(1)
	err := repo.Save(ctx, testEntity)
	assert.NoError(t, err)

	// 验证实体存在
	exists, err := repo.Exists(ctx, testEntity.GetID())
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试删除实体
	err = repo.Delete(ctx, testEntity.GetID())
	assert.NoError(t, err)

	// 验证实体不存在
	exists, err = repo.Exists(ctx, testEntity.GetID())
	assert.NoError(t, err)
	assert.False(t, exists)

	// 测试删除不存在的实体
	err = repo.Delete(ctx, "not-exist-id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)

	// 测试删除空ID实体
	err = repo.Delete(ctx, "")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestRedisRepository_SaveBatch 测试批量保存实体功能
func TestRedisRepository_SaveBatch(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	entityCount := 10
	entities := make([]*ComplexEntity, entityCount)
	for i := 0; i < entityCount; i++ {
		entities[i] = createComplexEntity(i)
	}

	// 测试批量保存
	err := repo.SaveBatch(ctx, entities)
	assert.NoError(t, err)

	// 验证所有实体都保存成功
	for _, entity := range entities {
		exists, err := repo.Exists(ctx, entity.GetID())
		assert.NoError(t, err)
		assert.True(t, exists)
	}

	// 测试批量更新
	for i := 0; i < entityCount; i++ {
		entities[i].Name = fmt.Sprintf("Updated Entity %d", i)
		entities[i].Status = "batch-updated"
	}

	err = repo.SaveBatch(ctx, entities)
	assert.NoError(t, err)

	// 验证所有实体都更新成功
	for _, entity := range entities {
		updatedEntity, err := repo.FindByID(ctx, entity.GetID())
		assert.NoError(t, err)
		assert.Equal(t, entity.Name, updatedEntity.Name)
		assert.Equal(t, "batch-updated", updatedEntity.Status)
	}

	// 测试批量保存包含空ID的实体
	invalidEntities := make([]*ComplexEntity, 2)
	invalidEntities[0] = createComplexEntity(99)
	invalidEntities[1] = createComplexEntity(100)
	invalidEntities[1].SetID("") // 设置一个空ID

	err = repo.SaveBatch(ctx, invalidEntities)
	assert.Error(t, err)

	// 测试批量保存空列表
	err = repo.SaveBatch(ctx, []*ComplexEntity{})
	assert.NoError(t, err)
}

// TestRedisRepository_FindByIDs 测试根据多个ID查找实体功能
func TestRedisRepository_FindByIDs(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	testEntities := make([]*ComplexEntity, 5)
	for i := 0; i < 5; i++ {
		testEntities[i] = createComplexEntity(i)
		err := repo.Save(ctx, testEntities[i])
		assert.NoError(t, err)
	}

	// 准备查询ID列表
	ids := make([]string, 3)
	ids[0] = testEntities[0].GetID()
	ids[1] = testEntities[2].GetID()
	ids[2] = testEntities[4].GetID()

	// 测试查找多个存在的实体
	foundEntities, err := repo.FindByIDs(ctx, ids)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(foundEntities))

	// 验证实体ID
	foundIDs := make(map[string]bool)
	for _, entity := range foundEntities {
		foundIDs[entity.GetID()] = true
	}

	for _, id := range ids {
		assert.Contains(t, foundIDs, id)
	}

	// 测试查找包含不存在ID的列表
	mixedIds := append(ids, "not-exist-id")
	mixedEntities, err := repo.FindByIDs(ctx, mixedIds)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(mixedEntities)) // 仍然返回3个存在的实体

	// 测试查找空ID列表
	emptyEntities, err := repo.FindByIDs(ctx, []string{})
	assert.NoError(t, err)
	assert.Empty(t, emptyEntities)
}

// TestRedisRepository_DeleteBatch 测试批量删除实体功能
func TestRedisRepository_DeleteBatch(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	testEntities := make([]*ComplexEntity, 5)
	for i := 0; i < 5; i++ {
		testEntities[i] = createComplexEntity(i)
		err := repo.Save(ctx, testEntities[i])
		assert.NoError(t, err)
	}

	// 准备删除ID列表
	ids := make([]string, 3)
	ids[0] = testEntities[0].GetID()
	ids[1] = testEntities[2].GetID()
	ids[2] = testEntities[4].GetID()

	// 测试批量删除
	err := repo.DeleteBatch(ctx, ids)
	assert.NoError(t, err)

	// 验证实体是否删除成功
	for _, id := range ids {
		exists, err := repo.Exists(ctx, id)
		assert.NoError(t, err)
		assert.False(t, exists)
	}

	// 验证未删除的实体是否存在
	for i := 1; i < 5; i += 2 {
		exists, err := repo.Exists(ctx, testEntities[i].GetID())
		assert.NoError(t, err)
		assert.True(t, exists)
	}

	// 测试批量删除包含不存在ID的列表
	mixedIds := append(ids, "not-exist-id")
	err = repo.DeleteBatch(ctx, mixedIds)
	assert.NoError(t, err) // Redis允许删除不存在的键

	// 测试批量删除空列表
	err = repo.DeleteBatch(ctx, []string{})
	assert.NoError(t, err)
}

// TestRedisRepository_FindWithPagination 测试分页查询实体功能
func TestRedisRepository_FindWithPagination(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	entityCount := 20
	for i := 0; i < entityCount; i++ {
		testEntity := createComplexEntity(i)
		err := repo.Save(ctx, testEntity)
		assert.NoError(t, err)
	}

	// 测试第一页，每页5个
	page1, err := repo.FindWithPagination(ctx, 5, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page1))

	// 测试第二页，每页5个
	page2, err := repo.FindWithPagination(ctx, 5, 5)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page2))

	// 测试超出范围的分页
	emptyPage, err := repo.FindWithPagination(ctx, 5, 100)
	assert.NoError(t, err)
	assert.Empty(t, emptyPage)

	// 测试无效的分页参数
	invalidPage, err := repo.FindWithPagination(ctx, -1, -5)
	assert.NoError(t, err)
	assert.Len(t, invalidPage, 10) // 应该使用默认值10
}

// TestRedisRepository_Count 测试统计实体总数功能
func TestRedisRepository_Count(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 初始计数应为0
	count, err := repo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 准备测试数据
	entityCount := 7
	for i := 0; i < entityCount; i++ {
		testEntity := createComplexEntity(i)
		err := repo.Save(ctx, testEntity)
		assert.NoError(t, err)
	}

	// 验证计数是否正确
	count, err = repo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(entityCount), count)

	// 删除部分实体后验证计数
	deleteCount := 3
	for i := 0; i < deleteCount; i++ {
		err := repo.Delete(ctx, generateComplexEntityID(i))
		assert.NoError(t, err)
	}

	count, err = repo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(entityCount-deleteCount), count)
}

// TestRedisRepository_FindByField 测试按字段查找实体功能
func TestRedisRepository_FindByField(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	entityCount := 10
	for i := 0; i < entityCount; i++ {
		testEntity := createComplexEntity(i)
		// 设置一些特殊字段值用于测试
		if i%3 == 0 {
			testEntity.Status = "special-status"
		}
		err := repo.Save(ctx, testEntity)
		assert.NoError(t, err)
	}

	// 测试按字符串字段查找
	specialEntities, err := repo.FindByField(ctx, "Status", "special-status")
	assert.NoError(t, err)
	assert.NotEmpty(t, specialEntities)

	// 验证找到的实体数量
	expectedCount := 4 // 0, 3, 6, 9
	assert.Equal(t, expectedCount, len(specialEntities))

	// 验证每个实体的Status字段
	for _, entity := range specialEntities {
		assert.Equal(t, "special-status", entity.Status)
	}

	// 测试按数值字段查找
	priorityEntities, err := repo.FindByField(ctx, "Priority", 20)
	assert.NoError(t, err)
	assert.NotEmpty(t, priorityEntities)
	assert.Equal(t, 1, len(priorityEntities)) // 只有ID=2的实体Priority=20
	assert.Equal(t, "complex-2", priorityEntities[0].GetID())

	// 测试查找不存在的字段值
	notFoundEntities, err := repo.FindByField(ctx, "Status", "non-existent-status")
	assert.NoError(t, err)
	assert.Empty(t, notFoundEntities)

	// 测试空字段名
	_, err = repo.FindByField(ctx, "", "any-value")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestRedisRepository_Exists 测试检查实体是否存在功能
func TestRedisRepository_Exists(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	testEntity := createComplexEntity(1)
	err := repo.Save(ctx, testEntity)
	assert.NoError(t, err)

	// 测试存在的实体
	exists, err := repo.Exists(ctx, testEntity.GetID())
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试不存在的实体
	exists, err = repo.Exists(ctx, "not-exist-id")
	assert.NoError(t, err)
	assert.False(t, exists)

	// 测试空ID
	exists, err = repo.Exists(ctx, "")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestRedisRepository_TTL 测试TTL相关功能
func TestRedisRepository_TTL(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 创建一个设置了TTL的新仓库 - 添加类型断言，因为我们需要访问具体类型的方法
	redisWithTTL := NewRedisRepository[*ComplexEntity](repo.client, "ttl:", time.Second*10).(*RedisRepository[*ComplexEntity])
	assert.Equal(t, time.Second*10, redisWithTTL.GetTTL())

	// 保存实体
	testEntity := createComplexEntity(1)
	err := redisWithTTL.Save(ctx, testEntity)
	assert.NoError(t, err)

	// 验证TTL设置
	ttl, err := redisWithTTL.GetTTLForKey(ctx, testEntity.GetID())
	assert.NoError(t, err)
	assert.True(t, ttl > 0) // 应该有TTL设置

	// 测试修改TTL
	newTTL := time.Second * 5
	err = redisWithTTL.SetTTLForKey(ctx, testEntity.GetID(), newTTL)
	assert.NoError(t, err)

	// 验证TTL修改
	ttl, err = redisWithTTL.GetTTLForKey(ctx, testEntity.GetID())
	assert.NoError(t, err)
	assert.True(t, ttl > 0) // 仍然有TTL设置

	// 测试批量修改TTL
	entities := make([]*ComplexEntity, 3)
	for i := 0; i < 3; i++ {
		entities[i] = createComplexEntity(i + 10)
		err := redisWithTTL.Save(ctx, entities[i])
		assert.NoError(t, err)
	}

	// 验证批量实体的TTL
	for _, entity := range entities {
		ttl, err := redisWithTTL.GetTTLForKey(ctx, entity.GetID())
		assert.NoError(t, err)
		assert.True(t, ttl > 0)
	}

	// 测试获取不存在实体的TTL
	nonExistentTTL, err := redisWithTTL.GetTTLForKey(ctx, "not-exist-id")
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(-2), nonExistentTTL) // Redis返回-2表示键不存在

	// 测试设置空ID的TTL
	err = redisWithTTL.SetTTLForKey(ctx, "", time.Second*5)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)

	// 测试获取空ID的TTL
	nonExistentTTL, err = redisWithTTL.GetTTLForKey(ctx, "")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestRedisRepository_FindWithConditions 测试根据条件查询实体功能
func TestRedisRepository_FindWithConditions(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	entityCount := 20
	for i := 0; i < entityCount; i++ {
		testEntity := createComplexEntity(i)
		err := repo.Save(ctx, testEntity)
		assert.NoError(t, err)
	}

	// 测试单一条件查询 (EQ)
	conditions1 := []usecase.QueryCondition{
		{Field: "Status", Operator: "EQ", Value: "active"},
	}
	options1 := usecase.QueryOptions{
		Conditions: conditions1,
		Limit:      10,
		Offset:     0,
	}

	result1, err := repo.FindWithConditions(ctx, options1)
	assert.NoError(t, err)
	assert.NotEmpty(t, result1)
	for _, entity := range result1 {
		assert.Equal(t, "active", entity.Status)
	}

	// 测试多条件查询 (AND 组合)
	conditions2 := []usecase.QueryCondition{
		{Field: "Status", Operator: "EQ", Value: "active"},
		{Field: "Priority", Operator: "GT", Value: 50},
	}
	options2 := usecase.QueryOptions{
		Conditions: conditions2,
		Limit:      10,
		Offset:     0,
	}

	result2, err := repo.FindWithConditions(ctx, options2)
	assert.NoError(t, err)
	for _, entity := range result2 {
		assert.Equal(t, "active", entity.Status)
		assert.Greater(t, entity.Priority, 50)
	}

	// 测试排序和分页
	sortFields := []usecase.SortField{
		{Field: "Priority", Ascending: false},
	}
	options3 := usecase.QueryOptions{
		Conditions: conditions1,
		SortFields: sortFields,
		Limit:      5,
		Offset:     0,
	}

	result3, err := repo.FindWithConditions(ctx, options3)
	assert.NoError(t, err)
	assert.Len(t, result3, 5)

	// 验证排序
	for i := 0; i < len(result3)-1; i++ {
		assert.GreaterOrEqual(t, result3[i].Priority, result3[i+1].Priority)
	}

	// 测试不存在的条件
	conditions4 := []usecase.QueryCondition{
		{Field: "Status", Operator: "EQ", Value: "non-existent-status"},
	}
	options4 := usecase.QueryOptions{
		Conditions: conditions4,
		Limit:      10,
		Offset:     0,
	}

	result4, err := repo.FindWithConditions(ctx, options4)
	assert.NoError(t, err)
	assert.Empty(t, result4)
}

// TestRedisRepository_CountWithConditions 测试根据条件统计实体数量功能
func TestRedisRepository_CountWithConditions(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	entityCount := 20
	for i := 0; i < entityCount; i++ {
		testEntity := createComplexEntity(i)
		err := repo.Save(ctx, testEntity)
		assert.NoError(t, err)
	}

	// 测试单一条件统计
	conditions1 := []usecase.QueryCondition{
		{Field: "Status", Operator: "EQ", Value: "active"},
	}

	count1, err := repo.CountWithConditions(ctx, conditions1)
	assert.NoError(t, err)
	assert.True(t, count1 > 0)

	// 测试多条件统计
	conditions2 := []usecase.QueryCondition{
		{Field: "Status", Operator: "EQ", Value: "active"},
		{Field: "Priority", Operator: "GT", Value: 100},
	}

	count2, err := repo.CountWithConditions(ctx, conditions2)
	assert.NoError(t, err)
	// 应该比count1小
	assert.True(t, count2 >= 0 && count2 <= count1)

	// 测试不存在的条件统计
	conditions3 := []usecase.QueryCondition{
		{Field: "Status", Operator: "EQ", Value: "non-existent-status"},
	}

	count3, err := repo.CountWithConditions(ctx, conditions3)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count3)
}

// TestRedisRepository_FindByFieldWithPagination 测试按字段查找并支持分页功能
func TestRedisRepository_FindByFieldWithPagination(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	entityCount := 15
	for i := 0; i < entityCount; i++ {
		testEntity := createComplexEntity(i)
		// 设置一些特殊字段值用于测试
		if i%2 == 0 {
			testEntity.Status = "even-status"
		}
		err := repo.Save(ctx, testEntity)
		assert.NoError(t, err)
	}

	// 测试按字段查找并分页
	pageSize := 3
	page1, err := repo.FindByFieldWithPagination(ctx, "Status", "even-status", pageSize, 0)
	assert.NoError(t, err)
	assert.Len(t, page1, pageSize)

	// 验证每个实体的Status字段
	for _, entity := range page1 {
		assert.Equal(t, "even-status", entity.Status)
	}

	// 测试第二页
	page2, err := repo.FindByFieldWithPagination(ctx, "Status", "even-status", pageSize, 3)
	assert.NoError(t, err)
	assert.Len(t, page2, pageSize)

	// 验证分页是否正确（不同页的实体ID应不同）
	page1IDs := make(map[string]bool)
	for _, entity := range page1 {
		page1IDs[entity.GetID()] = true
	}

	for _, entity := range page2 {
		assert.NotContains(t, page1IDs, entity.GetID())
	}

	// 测试超出范围的分页
	emptyPage, err := repo.FindByFieldWithPagination(ctx, "Status", "even-status", pageSize, 100)
	assert.NoError(t, err)
	assert.Empty(t, emptyPage)

	// 测试无效的分页参数
	invalidPage, err := repo.FindByFieldWithPagination(ctx, "Status", "even-status", -1, -5)
	assert.NoError(t, err)
	assert.True(t, len(invalidPage) > 0) // 只要有结果就可以，不严格要求数量

	// 测试空字段名
	_, err = repo.FindByFieldWithPagination(ctx, "", "any-value", 10, 0)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestRedisRepository_CountByField 测试按字段统计数量功能
func TestRedisRepository_CountByField(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	expectedCount := 0
	entityCount := 10
	for i := 0; i < entityCount; i++ {
		testEntity := createComplexEntity(i)
		// 设置一些特殊字段值用于测试
		if i%3 == 0 {
			testEntity.Status = "count-test-status"
			expectedCount++
		}
		err := repo.Save(ctx, testEntity)
		assert.NoError(t, err)
	}

	// 测试按字段统计
	count, err := repo.CountByField(ctx, "Status", "count-test-status")
	assert.NoError(t, err)
	assert.Equal(t, int64(expectedCount), count)

	// 测试统计不存在的字段值
	zeroCount, err := repo.CountByField(ctx, "Status", "non-existent-status")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), zeroCount)
}

// TestRedisRepository_Random 测试随机获取实体功能
func TestRedisRepository_Random(t *testing.T) {
	repo, cleanup := setupTestRedisRepository(t)
	defer cleanup()

	ctx := context.Background()

	// 准备测试数据
	entity1 := createComplexEntity(1)
	entity2 := createComplexEntity(2)
	entity3 := createComplexEntity(3)

	// 保存测试实体
	err := repo.Save(ctx, entity1)
	assert.NoError(t, err)
	err = repo.Save(ctx, entity2)
	assert.NoError(t, err)
	err = repo.Save(ctx, entity3)
	assert.NoError(t, err)

	// 测试随机获取1个实体
	randomEntities, err := repo.Random(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, randomEntities, 1)

	// 测试随机获取2个实体
	randomEntities, err = repo.Random(ctx, 2)
	assert.NoError(t, err)
	assert.Len(t, randomEntities, 2)

	// 测试随机获取超过实体总数的情况
	randomEntities, err = repo.Random(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, randomEntities, 3)

	// 测试负数参数
	randomEntities, err = repo.Random(ctx, -1)
	assert.NoError(t, err)
	assert.Len(t, randomEntities, 1)
}