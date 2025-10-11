package repo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
)

// setupTestBadgerRepository 创建测试用的BadgerRepository实例
func setupTestBadgerRepository(t *testing.T) (*BadgerRepository[*entity.Task], func()) {
	t.Helper()
	
	// 创建临时内存数据库
	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opts)
	assert.NoError(t, err)

	// 创建仓库实例 - 添加类型断言，因为NewBadgerRepository现在返回接口类型
	repo := NewBadgerRepository[*entity.Task](db, "test:task:").(*BadgerRepository[*entity.Task])

	// 清理函数
	cleanup := func() {
	db.Close()
	}

	return repo, cleanup
}

// TestBadgerRepository_Save 测试保存实体
func TestBadgerRepository_Save(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建测试任务
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"

	// 测试保存
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 验证保存结果
	savedTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, task.Id, savedTask.Id)
	assert.Equal(t, task.Type, savedTask.Type)
}

// TestBadgerRepository_Save_EmptyID 测试保存ID为空的实体
func TestBadgerRepository_Save_EmptyID(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建ID为空的测试任务
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = ""

	// 测试保存
	err := repo.Save(context.Background(), task)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestBadgerRepository_Update 测试更新实体
func TestBadgerRepository_Update(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存测试任务
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 更新任务
	task.Type = "updated_type"
	task.Status = entity.TaskStatusRunning
	err = repo.Update(context.Background(), task)
	assert.NoError(t, err)

	// 验证更新结果
	updatedTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, "updated_type", updatedTask.Type)
	assert.Equal(t, entity.TaskStatusRunning, updatedTask.Status)
}

// TestBadgerRepository_Update_NotFound 测试更新不存在的实体
func TestBadgerRepository_Update_NotFound(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建不存在的任务
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "non_existent_id"

	// 测试更新
	err := repo.Update(context.Background(), task)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestBadgerRepository_FindByID 测试根据ID查找实体
func TestBadgerRepository_FindByID(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存测试任务
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 测试查找
	foundTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, task.Id, foundTask.Id)
	assert.Equal(t, task.Type, foundTask.Type)
}

// TestBadgerRepository_FindByID_NotFound 测试查找不存在的实体
func TestBadgerRepository_FindByID_NotFound(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 测试查找不存在的ID
	_, err := repo.FindByID(context.Background(), "non_existent_id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestBadgerRepository_FindAll 测试查找所有实体
func TestBadgerRepository_FindAll(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存多个测试任务
	tasks := []*entity.Task{
		entity.NewTask("type1", map[string]interface{}{"key": "value1"}),
		entity.NewTask("type2", map[string]interface{}{"key": "value2"}),
	}
	for i, task := range tasks {
		task.Id = fmt.Sprintf("test_id_%d", i)
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试查找所有
	allTasks, err := repo.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, len(tasks), len(allTasks))
}

// TestBadgerRepository_Delete 测试删除实体
func TestBadgerRepository_Delete(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存测试任务
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 测试删除
	err = repo.Delete(context.Background(), task.Id)
	assert.NoError(t, err)

	// 验证删除结果
	_, err = repo.FindByID(context.Background(), task.Id)
	assert.Error(t, err)
}

// TestBadgerRepository_Delete_NotFound 测试删除不存在的实体
func TestBadgerRepository_Delete_NotFound(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 测试删除不存在的ID
	err := repo.Delete(context.Background(), "non_existent_id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestBadgerRepository_SaveBatch 测试批量保存实体
func TestBadgerRepository_SaveBatch(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建多个测试任务
	tasks := []*entity.Task{
		entity.NewTask("type1", map[string]interface{}{"key": "value1"}),
		entity.NewTask("type2", map[string]interface{}{"key": "value2"}),
	}
	for i, task := range tasks {
		task.Id = fmt.Sprintf("test_id_%d", i)
	}

	// 测试批量保存
	err := repo.SaveBatch(context.Background(), tasks)
	assert.NoError(t, err)

	// 验证保存结果
	for _, task := range tasks {
		foundTask, err := repo.FindByID(context.Background(), task.Id)
		assert.NoError(t, err)
		assert.Equal(t, task.Id, foundTask.Id)
	}
}

// TestBadgerRepository_FindByIDs 测试根据多个ID查找实体
func TestBadgerRepository_FindByIDs(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存多个测试任务
	tasks := []*entity.Task{
		entity.NewTask("type1", map[string]interface{}{"key": "value1"}),
		entity.NewTask("type2", map[string]interface{}{"key": "value2"}),
	}
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		task.Id = fmt.Sprintf("test_id_%d", i)
		ids[i] = task.Id
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试批量查找
	foundTasks, err := repo.FindByIDs(context.Background(), ids)
	assert.NoError(t, err)
	assert.Equal(t, len(tasks), len(foundTasks))
}

// TestBadgerRepository_DeleteBatch 测试批量删除实体
func TestBadgerRepository_DeleteBatch(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存多个测试任务
	tasks := []*entity.Task{
		entity.NewTask("type1", map[string]interface{}{"key": "value1"}),
		entity.NewTask("type2", map[string]interface{}{"key": "value2"}),
	}
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		task.Id = fmt.Sprintf("test_id_%d", i)
		ids[i] = task.Id
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试批量删除
	err := repo.DeleteBatch(context.Background(), ids)
	assert.NoError(t, err)

	// 验证删除结果
	for _, id := range ids {
		_, err := repo.FindByID(context.Background(), id)
		assert.Error(t, err)
	}
}

// TestBadgerRepository_FindWithPagination 测试分页查找实体
func TestBadgerRepository_FindWithPagination(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存多个测试任务
	for i := 0; i < 5; i++ {
		task := entity.NewTask(fmt.Sprintf("type%d", i), map[string]interface{}{"key": fmt.Sprintf("value%d", i)})
		task.Id = fmt.Sprintf("test_id_%d", i)
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试分页查找
	page1, err := repo.FindWithPagination(context.Background(), 2, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(page1))

	page2, err := repo.FindWithPagination(context.Background(), 2, 2)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(page2))
}

// TestBadgerRepository_Count 测试统计实体数量
func TestBadgerRepository_Count(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存多个测试任务
	count := 3
	for i := 0; i < count; i++ {
		task := entity.NewTask(fmt.Sprintf("type%d", i), map[string]interface{}{"key": fmt.Sprintf("value%d", i)})
		task.Id = fmt.Sprintf("test_id_%d", i)
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试统计
	actualCount, err := repo.Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(count), actualCount)
}

// TestBadgerRepository_FindByField 测试按字段查找实体
func TestBadgerRepository_FindByField(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存多个测试任务
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "test_id_1"}, Type: "same_type", Status: entity.TaskStatusPending},
		{BaseEntity: entity.BaseEntity{Id: "test_id_2"}, Type: "same_type", Status: entity.TaskStatusRunning},
		{BaseEntity: entity.BaseEntity{Id: "test_id_3"}, Type: "different_type", Status: entity.TaskStatusPending},
	}
	for _, task := range tasks {
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试按字段查找
	foundTasks, err := repo.FindByField(context.Background(), "Type", "same_type")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(foundTasks))
}

// TestBadgerRepository_Exists 测试检查实体是否存在
func TestBadgerRepository_Exists(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存测试任务
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 测试存在的实体
	exists, err := repo.Exists(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试不存在的实体
	exists, err = repo.Exists(context.Background(), "non_existent_id")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestBadgerRepository_FindWithConditions 测试根据条件查询
func TestBadgerRepository_FindWithConditions(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存多个测试任务
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "test_id_1"}, Type: "type1", Status: entity.TaskStatusPending},
		{BaseEntity: entity.BaseEntity{Id: "test_id_2"}, Type: "type1", Status: entity.TaskStatusRunning},
		{BaseEntity: entity.BaseEntity{Id: "test_id_3"}, Type: "type2", Status: entity.TaskStatusPending},
	}
	for _, task := range tasks {
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试条件查询
	options := usecase.QueryOptions{
		Conditions: []usecase.QueryCondition{
			{Field: "Type", Operator: "EQ", Value: "type1"},
		},
	}

	foundTasks, err := repo.FindWithConditions(context.Background(), options)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(foundTasks))
}

// TestBadgerRepository_CountWithConditions 测试根据条件统计
func TestBadgerRepository_CountWithConditions(t *testing.T) {
	// 设置测试环境
	repo, cleanup := setupTestBadgerRepository(t)
	defer cleanup()

	// 创建并保存多个测试任务
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "test_id_1"}, Type: "type1", Status: entity.TaskStatusPending},
		{BaseEntity: entity.BaseEntity{Id: "test_id_2"}, Type: "type1", Status: entity.TaskStatusRunning},
		{BaseEntity: entity.BaseEntity{Id: "test_id_3"}, Type: "type2", Status: entity.TaskStatusCompleted},
		{BaseEntity: entity.BaseEntity{Id: "test_id_4"}, Type: "type3", Status: entity.TaskStatusPending},
		{BaseEntity: entity.BaseEntity{Id: "test_id_5"}, Type: "type123", Status: entity.TaskStatusPending},
	}
	for _, task := range tasks {
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试EQ条件
	t.Run("测试EQ条件", func(t *testing.T) {
		conditions := []usecase.QueryCondition{
			{Field: "Type", Operator: "EQ", Value: "type1"},
		}

		count, err := repo.CountWithConditions(context.Background(), conditions)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	// 测试LIKE条件
	t.Run("测试LIKE条件", func(t *testing.T) {
		conditions := []usecase.QueryCondition{
			{Field: "Type", Operator: "LIKE", Value: "type1"},
		}

		count, err := repo.CountWithConditions(context.Background(), conditions)
		assert.NoError(t, err)
		// type1和type123都包含type1
		assert.Equal(t, int64(3), count)
	})

	// 测试NEQ条件
	t.Run("测试NEQ条件", func(t *testing.T) {
		conditions := []usecase.QueryCondition{
			{Field: "Type", Operator: "NEQ", Value: "type1"},
		}

		count, err := repo.CountWithConditions(context.Background(), conditions)
		assert.NoError(t, err)
		// type2、type3、type123都不等于type1
		assert.Equal(t, int64(3), count)
	})

	// 测试IN条件
	t.Run("测试IN条件", func(t *testing.T) {
		conditions := []usecase.QueryCondition{
			{Field: "Type", Operator: "IN", Value: []string{"type1", "type2"}},
		}

		count, err := repo.CountWithConditions(context.Background(), conditions)
		assert.NoError(t, err)
		// type1和type2的任务总数
		assert.Equal(t, int64(3), count)
	})
}

// 辅助函数：生成唯一ID
func generateTaskID() string {
	return utils.GenerateID()
}