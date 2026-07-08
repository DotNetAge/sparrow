package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

// setupTestBoltRepository 创建测试用的 BoltRepository 实例
func setupTestBoltRepository(t *testing.T) (*BoltRepository[*entity.Task], func()) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := bolt.Open(dbPath, 0600, nil)
	assert.NoError(t, err)

	repo := NewBoltRepository[*entity.Task](db, "test_task").(*BoltRepository[*entity.Task])

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return repo, cleanup
}

// TestBoltRepository_Save 测试保存实体
func TestBoltRepository_Save(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"

	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	savedTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, task.Id, savedTask.Id)
	assert.Equal(t, task.Type, savedTask.Type)
}

// TestBoltRepository_Save_EmptyID 测试保存 ID 为空的实体
func TestBoltRepository_Save_EmptyID(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = ""

	err := repo.Save(context.Background(), task)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestBoltRepository_Update 测试更新实体
func TestBoltRepository_Update(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	task.Type = "updated_type"
	task.Status = entity.TaskStatusRunning
	err = repo.Update(context.Background(), task)
	assert.NoError(t, err)

	updatedTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, "updated_type", updatedTask.Type)
	assert.Equal(t, entity.TaskStatusRunning, updatedTask.Status)
}

// TestBoltRepository_Update_NotFound 测试更新不存在的实体
func TestBoltRepository_Update_NotFound(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "non_existent_id"

	err := repo.Update(context.Background(), task)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestBoltRepository_FindByID 测试根据 ID 查找实体
func TestBoltRepository_FindByID(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	foundTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, task.Id, foundTask.Id)
	assert.Equal(t, task.Type, foundTask.Type)
}

// TestBoltRepository_FindByID_NotFound 测试查找不存在的实体
func TestBoltRepository_FindByID_NotFound(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	_, err := repo.FindByID(context.Background(), "non_existent_id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestBoltRepository_FindAll 测试查找所有实体
func TestBoltRepository_FindAll(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	tasks := []*entity.Task{
		entity.NewTask("type1", map[string]interface{}{"key": "value1"}),
		entity.NewTask("type2", map[string]interface{}{"key": "value2"}),
	}
	for i, task := range tasks {
		task.Id = fmt.Sprintf("test_id_%d", i)
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	allTasks, err := repo.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, len(tasks), len(allTasks))
}

// TestBoltRepository_Delete 测试删除实体
func TestBoltRepository_Delete(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	err = repo.Delete(context.Background(), task.Id)
	assert.NoError(t, err)

	_, err = repo.FindByID(context.Background(), task.Id)
	assert.Error(t, err)
}

// TestBoltRepository_Delete_NotFound 测试删除不存在的实体
func TestBoltRepository_Delete_NotFound(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	err := repo.Delete(context.Background(), "non_existent_id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestBoltRepository_SaveBatch 测试批量保存实体
func TestBoltRepository_SaveBatch(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	tasks := []*entity.Task{
		entity.NewTask("type1", map[string]interface{}{"key": "value1"}),
		entity.NewTask("type2", map[string]interface{}{"key": "value2"}),
	}
	for i, task := range tasks {
		task.Id = fmt.Sprintf("test_id_%d", i)
	}

	err := repo.SaveBatch(context.Background(), tasks)
	assert.NoError(t, err)

	for _, task := range tasks {
		foundTask, err := repo.FindByID(context.Background(), task.Id)
		assert.NoError(t, err)
		assert.Equal(t, task.Id, foundTask.Id)
	}
}

// TestBoltRepository_FindByIDs 测试根据多个 ID 查找实体
func TestBoltRepository_FindByIDs(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

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

	foundTasks, err := repo.FindByIDs(context.Background(), ids)
	assert.NoError(t, err)
	assert.Equal(t, len(tasks), len(foundTasks))
}

// TestBoltRepository_DeleteBatch 测试批量删除实体
func TestBoltRepository_DeleteBatch(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

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

	err := repo.DeleteBatch(context.Background(), ids)
	assert.NoError(t, err)

	for _, id := range ids {
		_, err := repo.FindByID(context.Background(), id)
		assert.Error(t, err)
	}
}

// TestBoltRepository_FindWithPagination 测试分页查找实体
func TestBoltRepository_FindWithPagination(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	for i := 0; i < 5; i++ {
		task := entity.NewTask(fmt.Sprintf("type%d", i), map[string]interface{}{"key": fmt.Sprintf("value%d", i)})
		task.Id = fmt.Sprintf("test_id_%d", i)
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	page1, err := repo.FindWithPagination(context.Background(), 2, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(page1))

	page2, err := repo.FindWithPagination(context.Background(), 2, 2)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(page2))
}

// TestBoltRepository_Count 测试统计实体数量
func TestBoltRepository_Count(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	count := 3
	for i := 0; i < count; i++ {
		task := entity.NewTask(fmt.Sprintf("type%d", i), map[string]interface{}{"key": fmt.Sprintf("value%d", i)})
		task.Id = fmt.Sprintf("test_id_%d", i)
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	actualCount, err := repo.Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(count), actualCount)
}

// TestBoltRepository_FindByField 测试按字段查找实体
func TestBoltRepository_FindByField(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

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

	foundTasks, err := repo.FindByField(context.Background(), "Type", "same_type")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(foundTasks))
}

// TestBoltRepository_Exists 测试检查实体是否存在
func TestBoltRepository_Exists(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	exists, err := repo.Exists(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Exists(context.Background(), "non_existent_id")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestBoltRepository_Random 测试随机获取实体
func TestBoltRepository_Random(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

	task1 := entity.NewTask("test_type1", map[string]interface{}{"key": "value1"})
	task1.Id = "test_id1"
	task2 := entity.NewTask("test_type2", map[string]interface{}{"key": "value2"})
	task2.Id = "test_id2"
	task3 := entity.NewTask("test_type3", map[string]interface{}{"key": "value3"})
	task3.Id = "test_id3"

	err := repo.Save(context.Background(), task1)
	assert.NoError(t, err)
	err = repo.Save(context.Background(), task2)
	assert.NoError(t, err)
	err = repo.Save(context.Background(), task3)
	assert.NoError(t, err)

	randomTasks, err := repo.Random(context.Background(), 1)
	assert.NoError(t, err)
	assert.Len(t, randomTasks, 1)

	randomTasks, err = repo.Random(context.Background(), 2)
	assert.NoError(t, err)
	assert.Len(t, randomTasks, 2)

	randomTasks, err = repo.Random(context.Background(), 10)
	assert.NoError(t, err)
	assert.Len(t, randomTasks, 3)

	randomTasks, err = repo.Random(context.Background(), -1)
	assert.NoError(t, err)
	assert.Len(t, randomTasks, 1)
}

// TestBoltRepository_FindWithConditions 测试根据条件查询
func TestBoltRepository_FindWithConditions(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

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

	options := usecase.QueryOptions{
		Conditions: []usecase.QueryCondition{
			{Field: "Type", Operator: "EQ", Value: "type1"},
		},
	}

	foundTasks, err := repo.FindWithConditions(context.Background(), options)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(foundTasks))
}

// TestBoltRepository_CountWithConditions 测试根据条件统计
func TestBoltRepository_CountWithConditions(t *testing.T) {
	repo, cleanup := setupTestBoltRepository(t)
	defer cleanup()

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

	t.Run("测试 EQ 条件", func(t *testing.T) {
		conditions := []usecase.QueryCondition{
			{Field: "Type", Operator: "EQ", Value: "type1"},
		}

		count, err := repo.CountWithConditions(context.Background(), conditions)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("测试 LIKE 条件", func(t *testing.T) {
		conditions := []usecase.QueryCondition{
			{Field: "Type", Operator: "LIKE", Value: "type1"},
		}

		count, err := repo.CountWithConditions(context.Background(), conditions)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})

	t.Run("测试 NEQ 条件", func(t *testing.T) {
		conditions := []usecase.QueryCondition{
			{Field: "Type", Operator: "NEQ", Value: "type1"},
		}

		count, err := repo.CountWithConditions(context.Background(), conditions)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})

	t.Run("测试 IN 条件", func(t *testing.T) {
		conditions := []usecase.QueryCondition{
			{Field: "Type", Operator: "IN", Value: []string{"type1", "type2"}},
		}

		count, err := repo.CountWithConditions(context.Background(), conditions)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})
}

// 辅助函数：生成唯一 ID
func generateBoltTaskID() string {
	return utils.GenerateID()
}
