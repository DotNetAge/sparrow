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

func (w *SchedulerWrapper) RunTaskAt(at time.Time, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()

	// 构建任务，默认支持重试
	builder := NewTaskBuilder().
		WithID(taskId).
		ScheduleAt(at).
		WithHandler(handler)

	task := w.buildTaskWithRetry(builder)
	w.Instance.Schedule(task)

	return taskId
}
func (w *SchedulerWrapper) RunTaskRecurring(interval time.Duration, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()

	// 构建任务，默认支持重试
	builder := NewTaskBuilder().
		WithID(taskId).
		ScheduleRecurring(interval).
		WithHandler(handler)

	task := w.buildTaskWithRetry(builder)
	w.Instance.Schedule(task)

	return taskId
}

func (w *SchedulerWrapper) RunTask(handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()

	// 构建任务，默认支持重试
	builder := NewTaskBuilder().
		WithID(taskId).
		Immediate().
		WithHandler(handler)

	task := w.buildTaskWithRetry(builder)
	w.Instance.Schedule(task)

	return taskId
}

// RunTypedTask 提交一个指定类型的任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTypedTask(taskType string, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()

	// 构建任务，默认支持重试
	builder := NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		Immediate().
		WithHandler(handler)

	task := w.buildTaskWithRetry(builder)
	w.Instance.Schedule(task)

	return taskId
}

// RunTypedTaskAt 提交一个指定类型的定时任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// at: 定时执行时间
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTypedTaskAt(taskType string, at time.Time, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()

	// 构建任务，默认支持重试
	builder := NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		ScheduleAt(at).
		WithHandler(handler)

	task := w.buildTaskWithRetry(builder)
	w.Instance.Schedule(task)

	return taskId
}

// RunTypedTaskRecurring 提交一个指定类型的周期性任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// interval: 执行间隔
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTypedTaskRecurring(taskType string, interval time.Duration, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()

	// 构建任务，默认支持重试
	builder := NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		ScheduleRecurring(interval).
		WithHandler(handler)

	task := w.buildTaskWithRetry(builder)
	w.Instance.Schedule(task)

	return taskId
}

// buildTaskWithRetry 根据重试配置构建任务
func (w *SchedulerWrapper) buildTaskWithRetry(builder *TaskBuilder) Task {
	// 如果配置了重试，添加重试能力
	if w.RetryConfig != nil && w.RetryConfig.Enabled {
		retryBuilder := builder.WithRetry(w.RetryConfig.MaxRetries)

		// 根据退避策略配置
		if w.RetryConfig.BackoffMultiplier == 1.0 {
			if w.RetryConfig.MaxBackoff == w.RetryConfig.InitialBackoff {
				retryBuilder.WithFixedBackoff(w.RetryConfig.InitialBackoff)
			} else {
				retryBuilder.WithLinearBackoff(w.RetryConfig.InitialBackoff)
			}
		} else {
			retryBuilder.WithExponentialBackoff(w.RetryConfig.InitialBackoff).
				WithMaxDelay(w.RetryConfig.MaxBackoff)
		}

		return retryBuilder.Build()
	} else {
		return builder.Build()
	}
}
