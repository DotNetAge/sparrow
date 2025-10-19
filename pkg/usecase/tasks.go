package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// TaskHandler 任务处理器函数类型
type TaskHandler func(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error)

// TaskService 任务调度用例
// 位于整洁架构的核心层 - 用例层
// 包含任务调度的所有业务逻辑
type TaskService struct {
	repo     Repository[*entity.Task]
	handlers map[string]TaskHandler
	mu       sync.RWMutex
	log      *logger.Logger
}

// NewTaskService 创建任务调度用例实例
func NewTaskService(repo Repository[*entity.Task], log *logger.Logger) *TaskService {
	return &TaskService{
		repo:     repo,
		handlers: make(map[string]TaskHandler),
		log:      log,
	}
}

// RegisterHandler 注册任务处理器
func (uc *TaskService) RegisterHandler(taskType string, handler TaskHandler) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.handlers[taskType] = handler
}

// CreateTask 创建新任务
// 业务逻辑：创建新的任务实体并保存
func (uc *TaskService) CreateTask(ctx context.Context, taskType string, payload map[string]interface{}) (*entity.Task, error) {
	if taskType == "" {
		uc.log.Error("任务类型不能为空")
		return nil, errors.New("任务类型不能为空")
	}

	task := entity.NewTask(taskType, payload)
	if err := uc.repo.Save(ctx, task); err != nil {
		uc.log.Error("创建任务失败: %v", err)
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	return task, nil
}

// GetTask 获取任务
// 业务逻辑：获取任务详情
func (uc *TaskService) GetTask(ctx context.Context, id string) (*entity.Task, error) {
	task, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		uc.log.Error("获取任务失败: %v", err)
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	return task, nil
}

// ListTasks 列出任务
// 业务逻辑：按状态筛选任务
func (uc *TaskService) ListTasks(ctx context.Context, status *entity.TaskStatus) ([]*entity.Task, error) {
	var tasks []*entity.Task
	var err error

	if status == nil {
		tasks, err = uc.repo.FindAll(ctx)
	} else {
		// 使用条件查询
		conditions := []QueryCondition{
			{
				Field:    "status",
				Operator: "EQ",
				Value:    *status,
			},
		}
		options := QueryOptions{
			Conditions: conditions,
		}
		tasks, err = uc.repo.FindWithConditions(ctx, options)
	}

	if err != nil {
		uc.log.Error("列出任务失败: %v", err)
		return nil, fmt.Errorf("列出任务失败: %w", err)
	}

	return tasks, nil
}

// DeleteTask 删除任务
// 业务逻辑：删除任务（仅允许删除已完成或失败的任务）
func (uc *TaskService) DeleteTask(ctx context.Context, id string) error {
	task, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		uc.log.Error("获取任务失败: %v", err)
		return fmt.Errorf("获取任务失败: %w", err)
	}

	// 不允许删除正在运行的任务
	if task.Status == entity.TaskStatusRunning {
		uc.log.Error("不能删除正在运行的任务: %s", task.Id)
		return errors.New("不能删除正在运行的任务")
	}

	return uc.repo.Delete(ctx, id)
}

// ExecuteTask 执行任务
// 业务逻辑：同步执行任务，包含重试机制
func (uc *TaskService) ExecuteTask(ctx context.Context, id string) error {
	task, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		uc.log.Error("获取任务失败: %v", err)
		return fmt.Errorf("获取任务失败: %w", err)
	}

	// 检查任务状态
	if task.Status == entity.TaskStatusRunning {
		uc.log.Error("任务已经在运行中: %s", task.Id)
		return errors.New("任务已经在运行中")
	}

	if task.Status == entity.TaskStatusCompleted {
		uc.log.Error("任务已经完成: %s", task.Id)
		return errors.New("任务已经完成")
	}

	// 获取处理器
	uc.mu.RLock()
	handler, exists := uc.handlers[task.Type]
	uc.mu.RUnlock()

	if !exists {
		uc.log.Error("未注册该任务类型的处理器: %s", task.Type)
		return fmt.Errorf("未注册该任务类型的处理器: %s", task.Type)
	}

	// 执行任务
	task.Start()
	if err = uc.repo.Save(ctx, task); err != nil {
		uc.log.Error("更新任务状态失败: %v", err)
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	result, err := handler(ctx, task.Payload)
	if err != nil {
		task.Fail(err)
		if task.CanRetry() {
			// 自动重试
			return uc.ExecuteTask(ctx, id)
		}
		uc.log.Error("任务执行失败: %v", err)
	} else {
		task.Complete(result)
	}

	return uc.repo.Save(ctx, task)
}

// ExecuteTaskAsync 异步执行任务
// 业务逻辑：在后台goroutine中执行任务
func (uc *TaskService) ExecuteTaskAsync(ctx context.Context, id string) error {
	go func() {
		// 使用新的context避免父context被取消的影响
		workerCtx := context.Background()
		_ = uc.ExecuteTask(workerCtx, id)
	}()

	return nil
}

// GetTaskStats 获取任务统计
// 业务逻辑：统计各状态任务数量
func (s *TaskService) GetTaskStats(ctx context.Context) (map[entity.TaskStatus]int, error) {
	// 获取所有任务
	tasks, err := s.repo.FindAll(ctx)
	if err != nil {
		s.log.Error("获取任务失败: %v", err)
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	// 统计各状态任务数量
	stats := make(map[entity.TaskStatus]int)
	for _, task := range tasks {
		stats[task.Status]++
	}

	return stats, nil
}

// ListPendingTasks 获取待处理任务
// 业务逻辑：获取待处理任务，按优先级排序
func (uc *TaskService) ListPendingTasks(ctx context.Context, limit int) ([]*entity.Task, error) {
	// 使用条件查询获取待处理任务
	conditions := []QueryCondition{
		{
			Field:    "status",
			Operator: "EQ",
			Value:    entity.TaskStatusPending,
		},
	}

	// 按优先级排序（优先级高的在前）
	sortFields := []SortField{
		{
			Field:     "priority",
			Ascending: false,
		},
		{
			Field:     "created_at",
			Ascending: true, // 相同优先级按创建时间排序
		},
	}

	options := QueryOptions{
		Conditions: conditions,
		SortFields: sortFields,
		Limit:      limit,
	}

	return uc.repo.FindWithConditions(ctx, options)
}

// CancelTask 取消任务
// 业务逻辑：取消待处理任务
func (uc *TaskService) CancelTask(ctx context.Context, id string) error {
	task, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		uc.log.Error("获取任务失败: %v", err)
		return fmt.Errorf("获取任务失败: %w", err)
	}

	if task.Status != entity.TaskStatusPending {
		uc.log.Error("无法取消任务: %s", task.Id)
		return errors.New("只能取消待处理的任务")
	}

	task.Cancel()
	return uc.repo.Save(ctx, task)
}

// CleanupExpiredTasks 清理过期任务
// 业务逻辑：清理超过24小时的任务
func (uc *TaskService) CleanupExpiredTasks(ctx context.Context) error {
	tasks, err := uc.repo.FindAll(ctx)
	if err != nil {
		uc.log.Error("列出任务失败: %v", err)
		return fmt.Errorf("列出任务失败: %w", err)
	}

	for _, task := range tasks {
		if task.IsExpired() {
			if err := uc.repo.Delete(ctx, task.GetID()); err != nil {
				// 记录错误但不中断清理过程
				uc.log.Error("删除过期任务失败: %v", err)
				continue
			}
		}
	}

	return nil
}
