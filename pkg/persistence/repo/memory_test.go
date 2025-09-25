package repo

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// TestNewMemoryRepository 测试创建MemoryRepository实例
func TestNewMemoryRepository(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	assert.NotNil(t, repo)
	// 使用类型断言访问具体类型的字段
	memoryRepo, ok := repo.(*MemoryRepository[*entity.Task])
	assert.True(t, ok)
	assert.NotNil(t, memoryRepo.entities)
	assert.Empty(t, memoryRepo.entities)
}

// TestMemoryRepository_Save 测试保存实体
func TestMemoryRepository_Save(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"

	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	savedTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, task.Id, savedTask.Id)
	assert.Equal(t, task.Type, savedTask.Type)
}

// TestMemoryRepository_Save_EmptyID 测试保存ID为空的实体
func TestMemoryRepository_Save_EmptyID(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = ""

	err := repo.Save(context.Background(), task)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_Save_UpdateExisting 测试保存已存在的实体（更新操作）
func TestMemoryRepository_Save_UpdateExisting(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"

	// 首次保存
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 修改并再次保存（更新）
	task.Type = "updated_type"
	err = repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 验证更新结果
	savedTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, "updated_type", savedTask.Type)
}

// TestMemoryRepository_Update 测试更新实体
func TestMemoryRepository_Update(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"

	// 先保存
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 然后更新
	task.Type = "updated_type"
	task.Status = entity.TaskStatusRunning
	err = repo.Update(context.Background(), task)
	assert.NoError(t, err)

	// 验证更新结果
	savedTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, "updated_type", savedTask.Type)
	assert.Equal(t, entity.TaskStatusRunning, savedTask.Status)
}

// TestMemoryRepository_Update_EmptyID 测试更新ID为空的实体
func TestMemoryRepository_Update_EmptyID(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = ""

	err := repo.Update(context.Background(), task)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_Update_NotFound 测试更新不存在的实体
func TestMemoryRepository_Update_NotFound(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "non_existent_id"

	err := repo.Update(context.Background(), task)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_FindByID 测试根据ID查找实体
func TestMemoryRepository_FindByID(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"

	// 先保存
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 然后查找
	foundTask, err := repo.FindByID(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.Equal(t, task.Id, foundTask.Id)
	assert.Equal(t, task.Type, foundTask.Type)
}

// TestMemoryRepository_FindByID_EmptyID 测试查找ID为空的实体
func TestMemoryRepository_FindByID_EmptyID(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	_, err := repo.FindByID(context.Background(), "")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_FindByID_NotFound 测试查找不存在的实体
func TestMemoryRepository_FindByID_NotFound(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	_, err := repo.FindByID(context.Background(), "non_existent_id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_FindAll 测试查找所有实体
func TestMemoryRepository_FindAll(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "id1"}, Type: "type1"},
		{BaseEntity: entity.BaseEntity{Id: "id2"}, Type: "type2"},
		{BaseEntity: entity.BaseEntity{Id: "id3"}, Type: "type3"},
	}

	for _, task := range tasks {
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 查找所有实体
	allTasks, err := repo.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, len(tasks), len(allTasks))
}

// TestMemoryRepository_FindAll_Empty 测试查找所有实体（空仓库）
func TestMemoryRepository_FindAll_Empty(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	allTasks, err := repo.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, allTasks)
}

// TestMemoryRepository_Delete 测试删除实体
func TestMemoryRepository_Delete(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "test_id"

	// 先保存
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 然后删除
	err = repo.Delete(context.Background(), task.Id)
	assert.NoError(t, err)

	// 验证删除结果
	_, err = repo.FindByID(context.Background(), task.Id)
	assert.Error(t, err)
}

// TestMemoryRepository_Delete_EmptyID 测试删除ID为空的实体
func TestMemoryRepository_Delete_EmptyID(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	err := repo.Delete(context.Background(), "")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_Delete_NotFound 测试删除不存在的实体
func TestMemoryRepository_Delete_NotFound(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	err := repo.Delete(context.Background(), "non_existent_id")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_SaveBatch 测试批量保存实体
func TestMemoryRepository_SaveBatch(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建多个实体
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "batch_id1"}, Type: "batch_type1"},
		{BaseEntity: entity.BaseEntity{Id: "batch_id2"}, Type: "batch_type2"},
	}

	// 批量保存
	err := repo.SaveBatch(context.Background(), tasks)
	assert.NoError(t, err)

	// 验证保存结果
	for _, task := range tasks {
		foundTask, err := repo.FindByID(context.Background(), task.Id)
		assert.NoError(t, err)
		assert.Equal(t, task.Id, foundTask.Id)
	}
}

// TestMemoryRepository_SaveBatch_Empty 测试批量保存空列表
func TestMemoryRepository_SaveBatch_Empty(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	err := repo.SaveBatch(context.Background(), []*entity.Task{})
	assert.NoError(t, err)
}

// TestMemoryRepository_SaveBatch_EmptyID 测试批量保存包含空ID的实体
func TestMemoryRepository_SaveBatch_EmptyID(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建包含空ID的实体列表
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "batch_id1"}, Type: "batch_type1"},
		{BaseEntity: entity.BaseEntity{Id: ""}, Type: "batch_type2"},
	}

	err := repo.SaveBatch(context.Background(), tasks)
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_FindByIDs 测试批量查找实体
func TestMemoryRepository_FindByIDs(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "batch_find_id1"}, Type: "batch_find_type1"},
		{BaseEntity: entity.BaseEntity{Id: "batch_find_id2"}, Type: "batch_find_type2"},
	}

	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.Id
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 批量查找
	foundTasks, err := repo.FindByIDs(context.Background(), ids)
	assert.NoError(t, err)
	assert.Equal(t, len(tasks), len(foundTasks))
}

// TestMemoryRepository_FindByIDs_Empty 测试批量查找空列表
func TestMemoryRepository_FindByIDs_Empty(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	foundTasks, err := repo.FindByIDs(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Empty(t, foundTasks)
}

// TestMemoryRepository_FindByIDs_PartialFound 测试批量查找部分存在的实体
func TestMemoryRepository_FindByIDs_PartialFound(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存一个实体
	task := &entity.Task{BaseEntity: entity.BaseEntity{Id: "exists_id"}, Type: "exists_type"}
	task.SetUpdatedAt(time.Now())
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 批量查找（包含存在和不存在的ID）
	foundTasks, err := repo.FindByIDs(context.Background(), []string{"exists_id", "non_exists_id"})
	assert.NoError(t, err)
	assert.Len(t, foundTasks, 1)
	assert.Equal(t, "exists_id", foundTasks[0].Id)
}

// TestMemoryRepository_DeleteBatch 测试批量删除实体
func TestMemoryRepository_DeleteBatch(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "batch_delete_id1"}, Type: "batch_delete_type1"},
		{BaseEntity: entity.BaseEntity{Id: "batch_delete_id2"}, Type: "batch_delete_type2"},
	}

	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.Id
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 批量删除
	err := repo.DeleteBatch(context.Background(), ids)
	assert.NoError(t, err)

	// 验证删除结果
	for _, id := range ids {
		_, err := repo.FindByID(context.Background(), id)
		assert.Error(t, err)
	}
}

// TestMemoryRepository_DeleteBatch_Empty 测试批量删除空列表
func TestMemoryRepository_DeleteBatch_Empty(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	err := repo.DeleteBatch(context.Background(), []string{})
	assert.NoError(t, err)
}

// TestMemoryRepository_FindWithPagination 测试分页查找实体
func TestMemoryRepository_FindWithPagination(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	for i := 0; i < 5; i++ {
		task := &entity.Task{BaseEntity: entity.BaseEntity{Id: utils.GenerateID()}, Type: fmt.Sprintf("page_type_%d", i)}
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试分页
	page1, err := repo.FindWithPagination(context.Background(), 2, 0)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := repo.FindWithPagination(context.Background(), 2, 2)
	assert.NoError(t, err)
	assert.Len(t, page2, 2)

	// 测试无效的分页参数
	defaultPage, err := repo.FindWithPagination(context.Background(), 0, -1)
	assert.NoError(t, err)
	// 默认应该返回最多10条记录
	assert.Len(t, defaultPage, 5)

	// 测试超出范围的偏移量
	emptyPage, err := repo.FindWithPagination(context.Background(), 2, 10)
	assert.NoError(t, err)
	assert.Empty(t, emptyPage)
}

// TestMemoryRepository_Count 测试统计实体数量
func TestMemoryRepository_Count(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	expectedCount := 3
	for i := 0; i < expectedCount; i++ {
		task := &entity.Task{BaseEntity: entity.BaseEntity{Id: utils.GenerateID()}, Type: fmt.Sprintf("count_type_%d", i)}
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 统计数量
	actualCount, err := repo.Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(expectedCount), actualCount)
}

// TestMemoryRepository_Count_Empty 测试统计空仓库的实体数量
func TestMemoryRepository_Count_Empty(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	count, err := repo.Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// TestMemoryRepository_FindByField 测试按字段查找实体
func TestMemoryRepository_FindByField(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "field_id1"}, Type: "same_type", Status: entity.TaskStatusPending},
		{BaseEntity: entity.BaseEntity{Id: "field_id2"}, Type: "same_type", Status: entity.TaskStatusRunning},
		{BaseEntity: entity.BaseEntity{Id: "field_id3"}, Type: "different_type", Status: entity.TaskStatusPending},
	}

	for _, task := range tasks {
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 按字段查找
	foundTasks, err := repo.FindByField(context.Background(), "Type", "same_type")
	assert.NoError(t, err)
	assert.Len(t, foundTasks, 2)

	// 测试查找不存在的字段
	foundTasks, err = repo.FindByField(context.Background(), "NonExistentField", "value")
	assert.NoError(t, err)
	assert.Empty(t, foundTasks)

	// 测试空字段名
	_, err = repo.FindByField(context.Background(), "", "value")
	assert.Error(t, err)
	assert.IsType(t, &errs.RepositoryError{}, err)
}

// TestMemoryRepository_Exists 测试检查实体是否存在
func TestMemoryRepository_Exists(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	task := entity.NewTask("test_type", map[string]interface{}{"key": "value"})
	task.Id = "exists_id"

	// 先保存
	err := repo.Save(context.Background(), task)
	assert.NoError(t, err)

	// 检查存在的实体
	exists, err := repo.Exists(context.Background(), task.Id)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 检查不存在的实体
	exists, err = repo.Exists(context.Background(), "non_exists_id")
	assert.NoError(t, err)
	assert.False(t, exists)

	// 检查空ID
	exists, err = repo.Exists(context.Background(), "")
	assert.Error(t, err)
	assert.False(t, exists)
}

// TestMemoryRepository_Clear 测试清空所有实体
func TestMemoryRepository_Clear(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	for i := 0; i < 3; i++ {
		task := &entity.Task{BaseEntity: entity.BaseEntity{Id: utils.GenerateID()}, Type: fmt.Sprintf("clear_type_%d", i)}
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 验证清空前的数量
	countBefore, err := repo.Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(3), countBefore)

	// 清空所有实体，使用类型断言调用具体类型的方法
	memoryRepo, ok := repo.(*MemoryRepository[*entity.Task])
	assert.True(t, ok)
	err = memoryRepo.Clear(context.Background())
	assert.NoError(t, err)

	// 验证清空后的数量
	countAfter, err := repo.Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), countAfter)

	// 验证所有实体都被清空
	allEntities, err := repo.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, allEntities)
}

// TestMemoryRepository_Insert_Concurrency 测试并发插入操作
func TestMemoryRepository_Insert_Concurrency(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()
	concurrency := 10
	var wg sync.WaitGroup
	errChan := make(chan error, concurrency)

	// 并发插入
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task := &entity.Task{BaseEntity: entity.BaseEntity{Id: fmt.Sprintf("concurrency_id_%d", id)}, Type: "concurrency_type"}
			task.SetUpdatedAt(time.Now())
			err := repo.Save(context.Background(), task)
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		t.Fatalf("并发插入失败: %v", err)
	}

	// 验证所有实体都被正确插入
	count, err := repo.Count(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(concurrency), count)
}

// TestMemoryRepository_FindWithConditions 测试条件查询
func TestMemoryRepository_FindWithConditions(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "cond_id1"}, Type: "type1", Status: entity.TaskStatusPending},
		{BaseEntity: entity.BaseEntity{Id: "cond_id2"}, Type: "type1", Status: entity.TaskStatusRunning},
		{BaseEntity: entity.BaseEntity{Id: "cond_id3"}, Type: "type2", Status: entity.TaskStatusPending},
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
	assert.Len(t, foundTasks, 2)
}

// TestMemoryRepository_CountWithConditions 测试条件统计
func TestMemoryRepository_CountWithConditions(t *testing.T) {
	repo := NewMemoryRepository[*entity.Task]()

	// 创建并保存多个实体
	tasks := []*entity.Task{
		{BaseEntity: entity.BaseEntity{Id: "count_cond_id1"}, Type: "type1", Status: entity.TaskStatusPending},
		{BaseEntity: entity.BaseEntity{Id: "count_cond_id2"}, Type: "type1", Status: entity.TaskStatusRunning},
		{BaseEntity: entity.BaseEntity{Id: "count_cond_id3"}, Type: "type2", Status: entity.TaskStatusPending},
	}

	for _, task := range tasks {
		task.SetUpdatedAt(time.Now())
		err := repo.Save(context.Background(), task)
		assert.NoError(t, err)
	}

	// 测试条件统计
	conditions := []usecase.QueryCondition{
		{Field: "Type", Operator: "EQ", Value: "type1"},
	}

	count, err := repo.CountWithConditions(context.Background(), conditions)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}