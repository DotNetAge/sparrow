package repo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/stretchr/testify/assert"
)

// 实体定义
// 测试实体

// User 测试用的用户实体
// 实现了entity.Entity接口
// 包含gorm.Model支持软删除

type User struct {
	gorm.Model
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Email     string    `gorm:"size:100;uniqueIndex" json:"email"`
	Age       int       `gorm:"default:0" json:"age"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	DeletedAt *time.Time
}

// GetID 获取实体ID
func (u User) GetID() string {
	return u.ID
}

// SetID 设置实体ID
func (u *User) SetID(id string) {
	u.ID = id
}

// GetCreatedAt 获取创建时间
func (u User) GetCreatedAt() time.Time {
	return u.CreatedAt
}

// SetCreatedAt 设置创建时间
func (u *User) SetCreatedAt(t time.Time) {
	u.CreatedAt = t
}

// GetUpdatedAt 获取更新时间
func (u User) GetUpdatedAt() time.Time {
	return u.UpdatedAt
}

// SetUpdatedAt 设置更新时间
func (u *User) SetUpdatedAt(t time.Time) {
	u.UpdatedAt = t
}

// 简单实体，不包含软删除字段

type SimpleEntity struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// GetID 获取实体ID
func (s SimpleEntity) GetID() string {
	return s.ID
}

// SetID 设置实体ID
func (s *SimpleEntity) SetID(id string) {
	s.ID = id
}

// GetCreatedAt 获取创建时间
func (s SimpleEntity) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// SetCreatedAt 设置创建时间
func (s *SimpleEntity) SetCreatedAt(t time.Time) {
	s.CreatedAt = t
}

// GetUpdatedAt 获取更新时间
func (s SimpleEntity) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}

// SetUpdatedAt 设置更新时间
func (s *SimpleEntity) SetUpdatedAt(t time.Time) {
	s.UpdatedAt = t
}

// OrderItem 订单项实体
type OrderItem struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

// Order 订单实体
type Order struct {
	entity.BaseEntity
	OrderNumber     string      `json:"order_number"`
	CustomerID      string      `json:"customer_id"`
	Status          string      `json:"status"`
	TotalAmount     float64     `json:"total_amount"`
	Items           []OrderItem `json:"items"`
	ShippingAddress struct {
		Name    string `json:"name"`
		Phone   string `json:"phone"`
		Address string `json:"address"`
		City    string `json:"city"`
		ZIPCode string `json:"zip_code"`
		Country string `json:"country"`
	} `json:"shipping_address"`
	PaymentInfo struct {
		Method        string `json:"method"`
		TransactionID string `json:"transaction_id"`
		Status        string `json:"status"`
	} `json:"payment_info"`
}

// 测试工具函数

// setupTestMongoDB 设置测试用的MongoDB容器
func setupTestMongoDB(t *testing.T) (*mongo.Client, string) {
	t.Helper()

	ctx := context.Background()

	// 创建MongoDB容器
	req := testcontainers.ContainerRequest{
		Image:        "mongo:6.0",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForLog("Waiting for connections"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	assert.NoError(t, err)

	// 获取容器的端口
	port, err := container.MappedPort(ctx, "27017")
	assert.NoError(t, err)

	// 获取容器的主机
	host, err := container.Host(ctx)
	assert.NoError(t, err)

	// 构建连接字符串
	uri := fmt.Sprintf("mongodb://%s:%s", host, port.Port())

	// 连接MongoDB
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	assert.NoError(t, err)

	// 测试连接
	err = client.Ping(ctx, nil)
	assert.NoError(t, err)

	// 生成随机数据库名，避免测试冲突
	dbName := fmt.Sprintf("test_db_%d", time.Now().UnixNano())

	// 确保测试结束时清理资源
	t.Cleanup(func() {
		// 删除测试数据库
		if err := client.Database(dbName).Drop(ctx); err != nil {
			fmt.Printf("Warning: Failed to drop test database: %v\n", err)
		}
		// 断开连接
		if err := client.Disconnect(ctx); err != nil {
			fmt.Printf("Warning: Failed to disconnect from MongoDB: %v\n", err)
		}
		// 停止容器
		if err := container.Terminate(ctx); err != nil {
			fmt.Printf("Warning: Failed to terminate MongoDB container: %v\n", err)
		}
	})

	return client, dbName
}

// 单元测试

// TestNewMongoDBRepository 测试创建仓储实例
func TestNewMongoDBRepository(t *testing.T) {
	client, dbName := setupTestMongoDB(t)

	// 测试创建用户仓储
	repo := NewMongoDBRepository[*User](client, dbName)
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.client)
	assert.Equal(t, dbName, repo.dbName)
	assert.NotEmpty(t, repo.collection)
	assert.NotEmpty(t, repo.entityType)

	// 测试创建简单实体仓储
	simpleRepo := NewMongoDBRepository[*SimpleEntity](client, dbName)
	assert.NotNil(t, simpleRepo)
	assert.NotNil(t, simpleRepo.client)
	assert.Equal(t, dbName, simpleRepo.dbName)
}

// TestMongoDBRepository_Save 测试保存实体
func TestMongoDBRepository_Save(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 测试保存新实体
	user := &User{
		ID:    "user-1",
		Name:  "Test User",
		Email: "test@example.com",
		Age:   25,
	}
	err := repo.Save(ctx, user)
	assert.NoError(t, err)
	assert.NotZero(t, user.CreatedAt)
	assert.NotZero(t, user.UpdatedAt)

	// 测试保存已存在的实体
	updatedUser := &User{
		ID:    "user-1",
		Name:  "Updated User",
		Email: "test@example.com",
		Age:   26,
	}
	t1 := updatedUser.UpdatedAt
	err = repo.Save(ctx, updatedUser)
	assert.NoError(t, err)
	assert.True(t, updatedUser.UpdatedAt.After(t1))

	// 测试保存空ID的实体
	invalidUser := &User{Name: "Invalid User"}
	err = repo.Save(ctx, invalidUser)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMongoDBRepository_FindByID 测试根据ID查找实体
func TestMongoDBRepository_FindByID(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	user := &User{
		ID:    "user-2",
		Name:  "Find User",
		Email: "find@example.com",
		Age:   30,
	}
	err := repo.Save(ctx, user)
	assert.NoError(t, err)

	// 测试查找存在的实体
	foundUser, err := repo.FindByID(ctx, "user-2")
	assert.NoError(t, err)
	assert.NotNil(t, foundUser)
	assert.Equal(t, "user-2", foundUser.ID)
	assert.Equal(t, "Find User", foundUser.Name)

	// 测试查找不存在的实体
	notFoundUser, err := repo.FindByID(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
	// 在MongoDB实现中，即使找不到实体，也会返回空结构体指针而非nil
	if notFoundUser != nil {
		assert.Empty(t, notFoundUser.ID)
	}

	// 测试查找空ID的实体
	emptyIDUser, err := repo.FindByID(ctx, "")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
	// 在MongoDB实现中，即使ID为空，也会返回空结构体指针而非nil
	if emptyIDUser != nil {
		assert.Empty(t, emptyIDUser.ID)
	}
}

// TestMongoDBRepository_FindAll 测试查找所有实体
func TestMongoDBRepository_FindAll(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	users := []*User{
		{ID: "user-3", Name: "User 3", Email: "user3@example.com", Age: 31},
		{ID: "user-4", Name: "User 4", Email: "user4@example.com", Age: 32},
		{ID: "user-5", Name: "User 5", Email: "user5@example.com", Age: 33},
	}

	for _, user := range users {
		err := repo.Save(ctx, user)
		assert.NoError(t, err)
	}

	// 测试查找所有实体
	allUsers, err := repo.FindAll(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, allUsers)
	assert.GreaterOrEqual(t, len(allUsers), len(users))

	// 检查是否所有插入的用户都能找到
	foundIDs := make(map[string]bool)
	for _, user := range allUsers {
		foundIDs[user.ID] = true
	}

	for _, user := range users {
		assert.True(t, foundIDs[user.ID])
	}
}

// TestMongoDBRepository_Update 测试更新实体
func TestMongoDBRepository_Update(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	user := &User{
		ID:    "user-6",
		Name:  "Update User",
		Email: "update@example.com",
		Age:   35,
	}
	err := repo.Save(ctx, user)
	assert.NoError(t, err)

	// 保存更新前的时间
	oldUpdatedAt := user.UpdatedAt

	// 更新实体
	user.Name = "Updated User Name"
	user.Age = 36
	err = repo.Update(ctx, user)
	assert.NoError(t, err)
	assert.True(t, user.UpdatedAt.After(oldUpdatedAt))

	// 验证更新是否成功
	updatedUser, err := repo.FindByID(ctx, "user-6")
	assert.NoError(t, err)
	assert.Equal(t, "Updated User Name", updatedUser.Name)
	assert.Equal(t, 36, updatedUser.Age)

	// 测试更新不存在的实体
	nonExistentUser := &User{
		ID:    "non-existent-id",
		Name:  "Non Existent",
		Email: "non-existent@example.com",
		Age:   40,
	}
	err = repo.Update(ctx, nonExistentUser)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)

	// 测试更新空ID的实体
	emptyIDUser := &User{Name: "Empty ID"}
	err = repo.Update(ctx, emptyIDUser)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMongoDBRepository_Delete 测试删除实体
func TestMongoDBRepository_Delete(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	user := &User{
		ID:    "user-7",
		Name:  "Delete User",
		Email: "delete@example.com",
		Age:   40,
	}
	err := repo.Save(ctx, user)
	assert.NoError(t, err)

	// 测试删除实体
	err = repo.Delete(ctx, "user-7")
	assert.NoError(t, err)

	// 验证实体是否已删除
	_, err = repo.FindByID(ctx, "user-7")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)

	// 测试删除不存在的实体
	err = repo.Delete(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)

	// 测试删除空ID的实体
	err = repo.Delete(ctx, "")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMongoDBRepository_SaveBatch 测试批量保存实体
func TestMongoDBRepository_SaveBatch(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	users := []*User{
		{ID: "batch-user-1", Name: "Batch User 1", Email: "batch1@example.com", Age: 25},
		{ID: "batch-user-2", Name: "Batch User 2", Email: "batch2@example.com", Age: 26},
		{ID: "batch-user-3", Name: "Batch User 3", Email: "batch3@example.com", Age: 27},
	}

	// 测试批量保存
	err := repo.SaveBatch(ctx, users)
	assert.NoError(t, err)

	// 验证是否保存成功
	for _, user := range users {
		foundUser, err := repo.FindByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.NotNil(t, foundUser)
		assert.Equal(t, user.ID, foundUser.ID)
		assert.Equal(t, user.Name, foundUser.Name)
	}

	// 测试批量保存包含空ID的实体
	invalidUsers := []*User{
		{ID: "batch-user-4", Name: "Valid User", Email: "valid@example.com", Age: 28},
		{Name: "Invalid User", Email: "invalid@example.com", Age: 29}, // 空ID
	}

	err = repo.SaveBatch(ctx, invalidUsers)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)

	// 测试空数组
	err = repo.SaveBatch(ctx, []*User{})
	assert.NoError(t, err)
}

// TestMongoDBRepository_FindByIDs 测试根据ID列表批量查找实体
func TestMongoDBRepository_FindByIDs(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	users := []*User{
		{ID: "multi-user-1", Name: "Multi User 1", Email: "multi1@example.com", Age: 30},
		{ID: "multi-user-2", Name: "Multi User 2", Email: "multi2@example.com", Age: 31},
		{ID: "multi-user-3", Name: "Multi User 3", Email: "multi3@example.com", Age: 32},
	}

	for _, user := range users {
		err := repo.Save(ctx, user)
		assert.NoError(t, err)
	}

	// 测试批量查找
	ids := []string{"multi-user-1", "multi-user-3"}
	foundUsers, err := repo.FindByIDs(ctx, ids)
	assert.NoError(t, err)
	assert.NotNil(t, foundUsers)
	assert.Equal(t, 2, len(foundUsers))

	// 验证查找结果
	foundIDs := make(map[string]bool)
	for _, user := range foundUsers {
		foundIDs[user.ID] = true
	}

	assert.True(t, foundIDs["multi-user-1"])
	assert.True(t, foundIDs["multi-user-3"])

	// 测试空ID列表
	emptyUsers, err := repo.FindByIDs(ctx, []string{})
	assert.NoError(t, err)
	assert.NotNil(t, emptyUsers)
	assert.Empty(t, emptyUsers)
}

// TestMongoDBRepository_DeleteBatch 测试批量删除实体
func TestMongoDBRepository_DeleteBatch(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	users := []*User{
		{ID: "delete-user-1", Name: "Delete User 1", Email: "delete1@example.com", Age: 35},
		{ID: "delete-user-2", Name: "Delete User 2", Email: "delete2@example.com", Age: 36},
		{ID: "delete-user-3", Name: "Delete User 3", Email: "delete3@example.com", Age: 37},
	}

	for _, user := range users {
		err := repo.Save(ctx, user)
		assert.NoError(t, err)
	}

	// 测试批量删除
	ids := []string{"delete-user-1", "delete-user-3"}
	err := repo.DeleteBatch(ctx, ids)
	assert.NoError(t, err)

	// 验证删除结果
	for _, id := range ids {
		_, err := repo.FindByID(ctx, id)
		assert.Error(t, err)
		assert.IsType(t, &errs.RepositoryError{}, err)
	}

	// 验证未删除的实体
	existingUser, err := repo.FindByID(ctx, "delete-user-2")
	assert.NoError(t, err)
	assert.NotNil(t, existingUser)
	assert.Equal(t, "delete-user-2", existingUser.ID)

	// 测试空ID列表
	err = repo.DeleteBatch(ctx, []string{})
	assert.NoError(t, err)
}

// TestMongoDBRepository_FindWithPagination 测试分页查询
func TestMongoDBRepository_FindWithPagination(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*SimpleEntity](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	entities := []*SimpleEntity{}
	for i := 1; i <= 25; i++ {
		entities = append(entities, &SimpleEntity{
			ID:    fmt.Sprintf("entity-%d", i),
			Name:  fmt.Sprintf("Entity %d", i),
			Value: fmt.Sprintf("Value %d", i),
		})
	}

	for _, entity := range entities {
		err := repo.Save(ctx, entity)
		assert.NoError(t, err)
	}

	// 测试第一页
	page1, err := repo.FindWithPagination(ctx, 10, 0)
	assert.NoError(t, err)
	assert.NotNil(t, page1)
	assert.Equal(t, 10, len(page1))

	// 测试第二页
	page2, err := repo.FindWithPagination(ctx, 10, 10)
	assert.NoError(t, err)
	assert.NotNil(t, page2)
	assert.Equal(t, 10, len(page2))

	// 测试第三页
	page3, err := repo.FindWithPagination(ctx, 10, 20)
	assert.NoError(t, err)
	assert.NotNil(t, page3)
	assert.Equal(t, 5, len(page3))

	// 测试无效的分页参数
	invalidPage, err := repo.FindWithPagination(ctx, -5, -10)
	assert.NoError(t, err)
	assert.NotNil(t, invalidPage)
	// 应该返回默认的第一页
	assert.LessOrEqual(t, len(invalidPage), 10)
}

// TestMongoDBRepository_Count 测试统计实体数量
func TestMongoDBRepository_Count(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*SimpleEntity](client, dbName)
	ctx := context.Background()

	// 初始计数应为0
	count, err := repo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 添加一些实体
	entities := []*SimpleEntity{
		{ID: "count-entity-1", Name: "Count Entity 1", Value: "Value 1"},
		{ID: "count-entity-2", Name: "Count Entity 2", Value: "Value 2"},
		{ID: "count-entity-3", Name: "Count Entity 3", Value: "Value 3"},
	}

	for _, entity := range entities {
		err := repo.Save(ctx, entity)
		assert.NoError(t, err)
	}

	// 验证计数
	count, err = repo.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// TestMongoDBRepository_FindByField 测试按字段查找实体
func TestMongoDBRepository_FindByField(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	users := []*User{
		{ID: "field-user-1", Name: "Field User", Email: "field1@example.com", Age: 25},
		{ID: "field-user-2", Name: "Field User", Email: "field2@example.com", Age: 26},
		{ID: "field-user-3", Name: "Different User", Email: "different@example.com", Age: 27},
	}

	for _, user := range users {
		err := repo.Save(ctx, user)
		assert.NoError(t, err)
	}

	// 测试按字段查找
	foundUsers, err := repo.FindByField(ctx, "name", "Field User")
	assert.NoError(t, err)
	assert.NotNil(t, foundUsers)
	assert.Equal(t, 2, len(foundUsers))

	// 测试查找不存在的字段值
	notFoundUsers, err := repo.FindByField(ctx, "name", "Non Existent")
	assert.NoError(t, err)
	assert.NotNil(t, notFoundUsers)
	assert.Empty(t, notFoundUsers)

	// 测试空字段名
	_, err = repo.FindByField(ctx, "", "Value")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMongoDBRepository_FindByFieldWithPagination 测试按字段查找并分页
func TestMongoDBRepository_FindByFieldWithPagination(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*SimpleEntity](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	for i := 1; i <= 15; i++ {
		entity := &SimpleEntity{
			ID:    fmt.Sprintf("pagination-entity-%d", i),
			Name:  "Pagination Entity",
			Value: fmt.Sprintf("Value %d", i),
		}
		err := repo.Save(ctx, entity)
		assert.NoError(t, err)
	}

	// 测试第一页
	page1, err := repo.FindByFieldWithPagination(ctx, "name", "Pagination Entity", 10, 0)
	assert.NoError(t, err)
	assert.NotNil(t, page1)
	assert.Equal(t, 10, len(page1))

	// 测试第二页
	page2, err := repo.FindByFieldWithPagination(ctx, "name", "Pagination Entity", 10, 10)
	assert.NoError(t, err)
	assert.NotNil(t, page2)
	assert.Equal(t, 5, len(page2))

	// 测试空字段名
	_, err = repo.FindByFieldWithPagination(ctx, "", "Value", 10, 0)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMongoDBRepository_CountByField 测试按字段统计数量
func TestMongoDBRepository_CountByField(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	users := []*User{
		{ID: "count-field-user-1", Name: "Count Field User", Email: "count1@example.com", Age: 25},
		{ID: "count-field-user-2", Name: "Count Field User", Email: "count2@example.com", Age: 26},
		{ID: "count-field-user-3", Name: "Different User", Email: "different@example.com", Age: 27},
	}

	for _, user := range users {
		err := repo.Save(ctx, user)
		assert.NoError(t, err)
	}

	// 测试按字段统计
	count, err := repo.CountByField(ctx, "name", "Count Field User")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// 测试统计不存在的字段值
	zeroCount, err := repo.CountByField(ctx, "name", "Non Existent")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), zeroCount)

	// 测试空字段名
	_, err = repo.CountByField(ctx, "", "Value")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMongoDBRepository_Exists 测试检查实体是否存在
func TestMongoDBRepository_Exists(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	user := &User{
		ID:    "exists-user",
		Name:  "Exists User",
		Email: "exists@example.com",
		Age:   40,
	}
	err := repo.Save(ctx, user)
	assert.NoError(t, err)

	// 测试存在的实体
	exists, err := repo.Exists(ctx, "exists-user")
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试不存在的实体
	doesNotExist, err := repo.Exists(ctx, "non-existent-id")
	assert.NoError(t, err)
	assert.False(t, doesNotExist)

	// 测试空ID
	emptyIDExists, err := repo.Exists(ctx, "")
	assert.NoError(t, err)
	assert.False(t, emptyIDExists)
}

// TestMongoDBRepository_FindWithConditions 测试多条件查询
func TestMongoDBRepository_FindWithConditions(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	users := []*User{
		{ID: "condition-user-1", Name: "Condition User 1", Email: "cond1@example.com", Age: 25},
		{ID: "condition-user-2", Name: "Condition User 2", Email: "cond2@example.com", Age: 30},
		{ID: "condition-user-3", Name: "Condition User 3", Email: "cond3@example.com", Age: 35},
		{ID: "condition-user-4", Name: "Other User", Email: "other@example.com", Age: 30},
	}

	for _, user := range users {
		err := repo.Save(ctx, user)
		assert.NoError(t, err)
	}

	// 构建查询条件
	conditions := []usecase.QueryCondition{
		{Field: "name", Operator: "LIKE", Value: "Condition"},
		{Field: "age", Operator: "GT", Value: 28},
	}

	options := usecase.QueryOptions{
		Conditions: conditions,
		SortFields: []usecase.SortField{
			{Field: "age", Ascending: false},
		},
		Limit:  10,
		Offset: 0,
	}

	// 执行条件查询
	foundUsers, err := repo.FindWithConditions(ctx, options)
	assert.NoError(t, err)
	assert.NotNil(t, foundUsers)
	assert.Equal(t, 2, len(foundUsers))

	// 验证结果按年龄降序排序
	assert.True(t, foundUsers[0].Age >= foundUsers[1].Age)
}

// TestMongoDBRepository_CountWithConditions 测试按条件统计
func TestMongoDBRepository_CountWithConditions(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 准备测试数据
	users := []*User{
		{ID: "count-condition-user-1", Name: "Count Condition User", Email: "countcond1@example.com", Age: 25},
		{ID: "count-condition-user-2", Name: "Count Condition User", Email: "countcond2@example.com", Age: 30},
		{ID: "count-condition-user-3", Name: "Other User", Email: "countother@example.com", Age: 35},
	}

	for _, user := range users {
		err := repo.Save(ctx, user)
		assert.NoError(t, err)
	}

	// 构建查询条件
	conditions := []usecase.QueryCondition{
		{Field: "name", Operator: "LIKE", Value: "Count"},
	}

	// 执行条件统计
	count, err := repo.CountWithConditions(ctx, conditions)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// 其他MongoDB特定测试

// TestMongoDBRepository_WithTransaction 测试事务支持
func TestMongoDBRepository_WithTransaction(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 启动会话
	session, err := client.StartSession()
	assert.NoError(t, err)
	defer session.EndSession(ctx)

	// 创建事务上下文
	sessionCtx := mongo.NewSessionContext(ctx, session)

	// 测试WithTransaction方法
	transactionalRepo := repo.WithTransaction(sessionCtx)
	assert.NotNil(t, transactionalRepo)
	assert.Equal(t, repo, transactionalRepo) // 方法返回自身
}

// TestMongoDBRepository_GetClient 测试获取MongoDB客户端
func TestMongoDBRepository_GetClient(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)

	// 测试GetClient方法
	returnedClient := repo.GetClient()
	assert.NotNil(t, returnedClient)
	assert.Equal(t, client, returnedClient)
}

// TestMongoDBRepository_GetCollectionName 测试获取集合名称
func TestMongoDBRepository_GetCollectionName(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)

	// 测试GetCollectionName方法
	collectionName := repo.GetCollectionName()
	assert.NotEmpty(t, collectionName)
}

// TestMongoDBRepository_WithComplexEntity 测试复杂实体(订单)的操作
func TestMongoDBRepository_WithComplexEntity(t *testing.T) {
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*Order](client, dbName)
	ctx := context.Background()

	// 准备测试数据 - 创建一个带有多个订单项的订单
	order := &Order{
		OrderNumber: "ORD-2024-001",
		CustomerID:  "cust-456",
		Status:      "pending",
		TotalAmount: 349.97,
		Items: []OrderItem{
			{
				ID:          "item-1",
				ProductID:   "prod-001",
				ProductName: "高级智能手表",
				Quantity:    1,
				UnitPrice:   299.99,
			},
			{
				ID:          "item-2",
				ProductID:   "prod-002",
				ProductName: "手表充电器",
				Quantity:    2,
				UnitPrice:   24.99,
			},
		},
		ShippingAddress: struct {
			Name    string `json:"name"`
			Phone   string `json:"phone"`
			Address string `json:"address"`
			City    string `json:"city"`
			ZIPCode string `json:"zip_code"`
			Country string `json:"country"`
		}{"张三", "13800138000", "科技路100号", "深圳", "518000", "中国"},
		PaymentInfo: struct {
			Method        string `json:"method"`
			TransactionID string `json:"transaction_id"`
			Status        string `json:"status"`
		}{"credit_card", "pay-tx-789", "completed"},
	}
	// 使用MongoDB风格的ID
	objectID := primitive.NewObjectID()
	order.SetID(objectID.Hex())

	// 测试保存复杂实体
	err := repo.Save(ctx, order)
	assert.NoError(t, err)
	assert.NotZero(t, order.CreatedAt)
	assert.NotZero(t, order.UpdatedAt)

	// 测试查找复杂实体
	foundOrder, err := repo.FindByID(ctx, order.GetID())
	assert.NoError(t, err)
	assert.NotNil(t, foundOrder)
	assert.Equal(t, order.GetID(), foundOrder.GetID())
	assert.Equal(t, "ORD-2024-001", foundOrder.OrderNumber)
	assert.Equal(t, 2, len(foundOrder.Items))
	assert.Equal(t, "高级智能手表", foundOrder.Items[0].ProductName)
	assert.Equal(t, "张三", foundOrder.ShippingAddress.Name)

	// 测试更新复杂实体
	foundOrder.Status = "shipped"
	foundOrder.Items[0].Quantity = 2
	foundOrder.TotalAmount = 649.97
	err = repo.Update(ctx, foundOrder)
	assert.NoError(t, err)

	// 验证更新是否成功
	updatedOrder, err := repo.FindByID(ctx, order.GetID())
	assert.NoError(t, err)
	assert.Equal(t, "shipped", updatedOrder.Status)
	assert.Equal(t, 2, updatedOrder.Items[0].Quantity)
	assert.Equal(t, 649.97, updatedOrder.TotalAmount)

	// 测试删除复杂实体
	err = repo.Delete(ctx, order.GetID())
	assert.NoError(t, err)

	// 验证实体是否已删除
	_, err = repo.FindByID(ctx, order.GetID())
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMongoDBRepository_Random 测试随机获取实体
func TestMongoDBRepository_Random(t *testing.T) {
	// 设置测试环境
	client, dbName := setupTestMongoDB(t)
	repo := NewMongoDBRepository[*User](client, dbName)
	ctx := context.Background()

	// 创建多个测试用户
	user1 := &User{
		ID:    "user-1",
		Name:  "User 1",
		Email: "user1@example.com",
		Age:   25,
	}
	user2 := &User{
		ID:    "user-2",
		Name:  "User 2",
		Email: "user2@example.com",
		Age:   30,
	}
	user3 := &User{
		ID:    "user-3",
		Name:  "User 3",
		Email: "user3@example.com",
		Age:   35,
	}

	// 保存测试用户
	err := repo.Save(ctx, user1)
	assert.NoError(t, err)
	err = repo.Save(ctx, user2)
	assert.NoError(t, err)
	err = repo.Save(ctx, user3)
	assert.NoError(t, err)

	// 测试随机获取1个用户
	randomUsers, err := repo.Random(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, randomUsers, 1)

	// 测试随机获取2个用户
	randomUsers, err = repo.Random(ctx, 2)
	assert.NoError(t, err)
	assert.Len(t, randomUsers, 2)

	// 测试随机获取超过用户总数的情况
	randomUsers, err = repo.Random(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, randomUsers, 3)

	// 测试负数参数
	randomUsers, err = repo.Random(ctx, -1)
	assert.NoError(t, err)
	assert.Len(t, randomUsers, 1)
}

// 确保测试通过
func TestMain(m *testing.M) {
	// 这里可以添加全局设置
	m.Run()
}
