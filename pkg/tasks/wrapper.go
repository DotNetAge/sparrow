package tasks

import (
	"context"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/google/uuid"
)

type SchedulerWrapper struct {
	Instance    TaskScheduler
	RetryConfig *config.RetryConfig // 重试配置
}

func NewSchedulerWrapper(scheduler TaskScheduler, retryConfig *config.RetryConfig) *SchedulerWrapper {
	return &SchedulerWrapper{
		Instance:    scheduler,
		RetryConfig: retryConfig,
	}
}

// RunTaskAt 提交一个在指定时间执行的任务
// at: 任务执行时间
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTaskAt(at time.Time, handler func(ctx context.Context) error) (string, error) {
	taskId := uuid.New().String()
	err := w.RunTaskWithIDAt(taskId, at, handler)
	return taskId, err
}

// RunTaskWithIDAt 提交一个在指定时间执行的任务
// taskId: 任务ID
// at: 任务执行时间
// handler: 任务处理函数
// 返回: 错误信息
func (w *SchedulerWrapper) RunTaskWithIDAt(taskId string, at time.Time, handler func(ctx context.Context) error) error {
	return w.Instance.Schedule(w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithSchedule(at).
		WithHandler(handler)))
}

// RunTaskRecurring 提交一个周期性执行的任务
// interval: 任务执行间隔
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTaskRecurring(interval time.Duration, handler func(ctx context.Context) error) (string, error) {
	taskId := uuid.New().String()
	err := w.RunTaskWithIDRecurring(taskId, interval, handler)
	return taskId, err
}

// RunTaskWithIDRecurring 提交一个周期性执行的任务
// taskId: 任务ID
// interval: 任务执行间隔
// handler: 任务处理函数
// 返回: 错误信息
func (w *SchedulerWrapper) RunTaskWithIDRecurring(taskId string, interval time.Duration, handler func(ctx context.Context) error) error {
	return w.Instance.Schedule(w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithRecurring(interval).
		WithHandler(handler)))
}

// RunTask 提交一个立即执行的任务
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTask(handler func(ctx context.Context) error) (string, error) {
	taskId := uuid.New().String()
	err := w.RunTaskWithID(taskId, handler)
	return taskId, err
}

// RunTaskWithID 提交一个立即执行的任务
// taskId: 任务ID
// handler: 任务处理函数
// 返回: 错误信息
func (w *SchedulerWrapper) RunTaskWithID(taskId string, handler func(ctx context.Context) error) error {
	return w.Instance.Schedule(w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithHandler(handler)))
}

// RunTypedTask 提交一个指定类型的任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTypedTaskWithID(taskId, taskType string, handler func(ctx context.Context) error) error {
	// 构建任务，默认支持重试
	return w.Instance.Schedule(w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		WithHandler(handler)))
}

// RunTypedTask 提交一个指定类型的任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTypedTask(taskType string, handler func(ctx context.Context) error) (string, error) {
	taskId := uuid.New().String()
	err := w.RunTypedTaskWithID(taskId, taskType, handler)
	return taskId, err
}

// RunTypedTaskAt 提交一个指定类型的定时任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// at: 定时执行时间
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTypedTaskAt(taskType string, at time.Time, handler func(ctx context.Context) error) (string, error) {
	taskId := uuid.New().String()
	err := w.RunTypedTaskWithIDAt(taskId, taskType, at, handler)
	return taskId, err
}

func (w *SchedulerWrapper) RunTypedTaskWithIDAt(taskId, taskType string, at time.Time, handler func(ctx context.Context) error) error {
	// 构建任务，默认支持重试
	return w.Instance.Schedule(w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		WithSchedule(at).
		WithHandler(handler)))
}

// RunTypedTaskRecurring 提交一个指定类型的周期性任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// interval: 执行间隔
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTypedTaskRecurring(taskType string, interval time.Duration, handler func(ctx context.Context) error) (string, error) {
	taskId := uuid.New().String()
	err := w.Instance.Schedule(w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		WithRecurring(interval).
		WithHandler(handler)))
	return taskId, err
}

func (w *SchedulerWrapper) RunTypedTaskWithIDRecurring(taskId, taskType string, interval time.Duration, handler func(ctx context.Context) error) error {
	// 构建任务，默认支持重试
	return w.Instance.Schedule(w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		WithRecurring(interval).
		WithHandler(handler)))
}

// buildTaskWithRetry 根据重试配置构建任务
func (w *SchedulerWrapper) buildTaskWithRetry(builder *TaskBuilder) Task {
	// 默认启用重试，设置最大重试次数为3次
	maxRetries := 3
	
	// 如果有配置，使用配置中的重试次数
	if w.RetryConfig != nil && w.RetryConfig.Enabled && w.RetryConfig.MaxRetries > 0 {
		maxRetries = w.RetryConfig.MaxRetries
	}
	
	// 设置最大重试次数，重试逻辑移至任务执行器内部处理
	return builder.WithRetry(maxRetries).Build()
}
