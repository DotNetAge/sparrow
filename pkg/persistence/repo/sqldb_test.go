package repo

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// TestEntity 测试实体
type TestEntity struct {
	entity.BaseEntity
	// ID        string     `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
	// CreatedAt time.Time  `json:"created_at"`
	// UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// GetID 获取ID
// func (e *TestEntity) GetID() string {
// 	return e.ID
// }

// // SetID 设置ID
// func (e *TestEntity) SetID(id string) {
// 	e.ID = id
// }

// // GetCreatedAt 获取创建时间
// func (e *TestEntity) GetCreatedAt() time.Time {
// 	return e.CreatedAt
// }

// // GetUpdatedAt 获取更新时间
// func (e *TestEntity) GetUpdatedAt() time.Time {
// 	return e.UpdatedAt
// }

// // SetUpdatedAt 设置更新时间
// func (e *TestEntity) SetUpdatedAt(t time.Time) {
// 	e.UpdatedAt = t
// }

var _ entity.Entity = (*TestEntity)(nil)

// 初始化测试数据库
func initTestDB(t *testing.T) *sql.DB {
	// 创建内存中的SQLite数据库
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	require.NotNil(t, db)

	// 创建测试表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS testentity (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		age INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		deleted_at DATETIME
	);
	`
	_, err = db.Exec(createTableSQL)
	require.NoError(t, err)

	return db
}

// 清理测试数据
func cleanupTestDB(db *sql.DB, t *testing.T) {
	_, err := db.Exec("DROP TABLE IF EXISTS testentity")
	require.NoError(t, err)
	db.Close()
}

func TestSqlDBRepository_Save(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	testEntity := &TestEntity{
		BaseEntity: *entity.NewBaseEntity("1"),
		Name:       "Test User",
		Age:        30,
	}

	// 测试保存
	err := repo.Save(context.Background(), testEntity)
	require.NoError(t, err)

	// 验证保存结果
	result, err := repo.FindByID(context.Background(), "1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "1", result.Id)
	assert.Equal(t, "Test User", result.Name)
	assert.Equal(t, 30, result.Age)
	assert.True(t, result.CreatedAt.Before(time.Now()) || result.CreatedAt.Equal(time.Now()))
	assert.True(t, result.UpdatedAt.Before(time.Now()) || result.UpdatedAt.Equal(time.Now()))

	// 测试更新
	testEntity.Name = "Updated User"
	testEntity.Age = 31
	err = repo.Save(context.Background(), testEntity)
	require.NoError(t, err)

	// 验证更新结果
	result, err = repo.FindByID(context.Background(), "1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Updated User", result.Name)
	assert.Equal(t, 31, result.Age)
	assert.True(t, result.UpdatedAt.After(result.CreatedAt))
}

func TestSqlDBRepository_FindByID(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	testEntity := &TestEntity{
		BaseEntity: *entity.NewBaseEntity("1"),
		Name:       "Test User",
		Age:        30,
	}

	// 保存实体
	err := repo.Save(context.Background(), testEntity)
	require.NoError(t, err)

	// 测试查找存在的实体
	result, err := repo.FindByID(context.Background(), "1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "1", result.Id)
	assert.Equal(t, "Test User", result.Name)

	// 测试查找不存在的实体
	_, err = repo.FindByID(context.Background(), "non-existent")
	require.Error(t, err)
}

func TestSqlDBRepository_FindAll(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	entities := []*TestEntity{
		{BaseEntity: *entity.NewBaseEntity("1"), Name: "User 1", Age: 20},
		{BaseEntity: *entity.NewBaseEntity("2"), Name: "User 2", Age: 25},
		{BaseEntity: *entity.NewBaseEntity("3"), Name: "User 3", Age: 30},
	}

	// 保存实体
	for _, entity := range entities {
		err := repo.Save(context.Background(), entity)
		require.NoError(t, err)
	}

	// 测试查找所有实体
	result, err := repo.FindAll(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 3)
}

func TestSqlDBRepository_Update(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	testEntity := &TestEntity{
		BaseEntity: *entity.NewBaseEntity("1"),
		Name:       "Test User",
		Age:        30,
	}

	// 保存实体
	err := repo.Save(context.Background(), testEntity)
	require.NoError(t, err)

	// 测试更新
	testEntity.Name = "Updated User"
	testEntity.Age = 35
	err = repo.Update(context.Background(), testEntity)
	require.NoError(t, err)

	// 验证更新结果
	result, err := repo.FindByID(context.Background(), "1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Updated User", result.Name)
	assert.Equal(t, 35, result.Age)
}

func TestSqlDBRepository_Delete(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	testEntity := &TestEntity{
		BaseEntity: *entity.NewBaseEntity("1"),
		Name:       "Test User",
		Age:        30,
	}

	// 保存实体
	err := repo.Save(context.Background(), testEntity)
	require.NoError(t, err)

	// 测试删除
	err = repo.Delete(context.Background(), "1")
	require.NoError(t, err)

	// 验证删除结果
	_, err = repo.FindByID(context.Background(), "1")
	require.Error(t, err)
}

func TestSqlDBRepository_SaveBatch(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	entities := []*TestEntity{
		{BaseEntity: *entity.NewBaseEntity("1"), Name: "User 1", Age: 20},
		{BaseEntity: *entity.NewBaseEntity("2"), Name: "User 2", Age: 25},
		{BaseEntity: *entity.NewBaseEntity("3"), Name: "User 3", Age: 30},
	}

	// 测试批量保存
	err := repo.SaveBatch(context.Background(), entities)
	require.NoError(t, err)

	// 验证批量保存结果
	result, err := repo.FindAll(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 3)
}

func TestSqlDBRepository_FindByIDs(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	entities := []*TestEntity{
		{BaseEntity: *entity.NewBaseEntity("1"), Name: "User 1", Age: 20},
		{BaseEntity: *entity.NewBaseEntity("2"), Name: "User 2", Age: 25},
		{BaseEntity: *entity.NewBaseEntity("3"), Name: "User 3", Age: 30},
	}

	// 保存实体
	for _, entity := range entities {
		err := repo.Save(context.Background(), entity)
		require.NoError(t, err)
	}

	// 测试批量查找
	result, err := repo.FindByIDs(context.Background(), []string{"1", "3"})
	require.NoError(t, err)
	require.Len(t, result, 2)

	// 验证结果
	ids := make([]string, len(result))
	for i, entity := range result {
		ids[i] = entity.Id
	}
	assert.Contains(t, ids, "1")
	assert.Contains(t, ids, "3")
}

func TestSqlDBRepository_FindWithPagination(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	for i := 1; i <= 5; i++ {
		testEntity := &TestEntity{
			BaseEntity: *entity.NewBaseEntity(fmt.Sprintf("%d", i)),
			Name:       fmt.Sprintf("User %d", i),
			Age:        20 + i,
		}
		err := repo.Save(context.Background(), testEntity)
		require.NoError(t, err)
	}

	// 测试分页查询
	result, err := repo.FindWithPagination(context.Background(), 2, 1)
	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestSqlDBRepository_Count(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	entities := []*TestEntity{
		{BaseEntity: *entity.NewBaseEntity("1"), Name: "User 1", Age: 20},
		{BaseEntity: *entity.NewBaseEntity("2"), Name: "User 2", Age: 25},
	}

	// 保存实体
	for _, entity := range entities {
		err := repo.Save(context.Background(), entity)
		require.NoError(t, err)
	}

	// 测试统计
	count, err := repo.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestSqlDBRepository_FindByField(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	entities := []*TestEntity{
		{BaseEntity: *entity.NewBaseEntity("1"), Name: "User 1", Age: 20},
		{BaseEntity: *entity.NewBaseEntity("2"), Name: "User 2", Age: 25},
		{BaseEntity: *entity.NewBaseEntity("3"), Name: "User 3", Age: 25},
	}

	// 保存实体
	for _, entity := range entities {
		err := repo.Save(context.Background(), entity)
		require.NoError(t, err)
	}

	// 测试按字段查找
	result, err := repo.FindByField(context.Background(), "age", 25)
	require.NoError(t, err)
	require.Len(t, result, 2)

	// 验证结果
	for _, entity := range result {
		assert.Equal(t, 25, entity.Age)
	}
}

func TestSqlDBRepository_Exists(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	testEntity := &TestEntity{
		BaseEntity: *entity.NewBaseEntity("1"),
		Name:       "Test User",
		Age:        30,
	}

	// 保存实体
	err := repo.Save(context.Background(), testEntity)
	require.NoError(t, err)

	// 测试存在的实体
	exists, err := repo.Exists(context.Background(), "1")
	require.NoError(t, err)
	assert.True(t, exists)

	// 测试不存在的实体
	exists, err = repo.Exists(context.Background(), "non-existent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestSqlDBRepository_FindWithConditions(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	entities := []*TestEntity{
		{BaseEntity: *entity.NewBaseEntity("1"), Name: "User 1", Age: 20},
		{BaseEntity: *entity.NewBaseEntity("2"), Name: "User 2", Age: 25},
		{BaseEntity: *entity.NewBaseEntity("3"), Name: "User 3", Age: 30},
		{BaseEntity: *entity.NewBaseEntity("4"), Name: "User 4", Age: 35},
	}

	// 保存实体
	for _, entity := range entities {
		err := repo.Save(context.Background(), entity)
		require.NoError(t, err)
	}

	// 测试条件查询
	options := usecase.QueryOptions{
		Conditions: []usecase.QueryCondition{
			{Field: "Age", Operator: "GT", Value: 25},
			{Field: "Age", Operator: "LTE", Value: 35},
		},
		SortFields: []usecase.SortField{
			{Field: "Age", Ascending: true},
		},
		Limit:  2,
		Offset: 0,
	}

	result, err := repo.FindWithConditions(context.Background(), options)
	require.NoError(t, err)
	require.Len(t, result, 2)

	// 验证结果
	assert.Equal(t, 30, result[0].Age)
	assert.Equal(t, 35, result[1].Age)
}

func TestSqlDBRepository_CountWithConditions(t *testing.T) {
	// 初始化数据库
	db := initTestDB(t)
	defer cleanupTestDB(db, t)

	// 创建仓储
	repo := NewSqlDBRepository[*TestEntity](db)

	// 创建测试实体
	entities := []*TestEntity{
		{BaseEntity: *entity.NewBaseEntity("1"), Name: "User 1", Age: 20},
		{BaseEntity: *entity.NewBaseEntity("2"), Name: "User 2", Age: 25},
		{BaseEntity: *entity.NewBaseEntity("3"), Name: "User 3", Age: 30},
		{BaseEntity: *entity.NewBaseEntity("4"), Name: "User 4", Age: 35},
	}

	// 保存实体
	for _, entity := range entities {
		err := repo.Save(context.Background(), entity)
		require.NoError(t, err)
	}

	// 测试条件统计
	conditions := []usecase.QueryCondition{
		{Field: "age", Operator: "GT", Value: 25},
	}

	count, err := repo.CountWithConditions(context.Background(), conditions)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
