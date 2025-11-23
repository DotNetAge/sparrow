package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// 任务执行模式枚举
type TaskExecutionMode int

const (
	ModeDefault TaskExecutionMode = iota
	ModeSequential
	ModeConcurrent
)

// SchedulerWrapperOptions 任务调度器包装器配置选项
type SchedulerWrapperOptions struct {
	defaultRetryCount  int                          // 默认重试次数
	scheduler          TaskScheduler                // 自定义调度器实例
	taskModeMapping    map[string]TaskExecutionMode // 任务类型与执行模式映射
	maxConcurrentTasks int                          // 最大并发任务数
}

// SchedulerWrapperOption 定义配置选项函数类型
type SchedulerWrapperOption func(*SchedulerWrapperOptions)

// WithRetryCount 设置默认重试次数
func WithRetryCount(count int) SchedulerWrapperOption {
	return func(o *SchedulerWrapperOptions) {
		o.defaultRetryCount = count
	}
}

// WithScheduler 设置自定义调度器实例
func WithScheduler(scheduler TaskScheduler) SchedulerWrapperOption {
	return func(o *SchedulerWrapperOptions) {
		o.scheduler = scheduler
	}
}

// WithMaxConcurrent 设置最大并发任务数（仅对并发调度器和混合调度器内的并发生效）
func WithMaxConcurrent(count int) SchedulerWrapperOption {
	return func(o *SchedulerWrapperOptions) {
		// 确保并发数大于0
		if count > 0 {
			o.maxConcurrentTasks = count
		}
	}
}

// WithConcurrent 将指定的任务类型设置为并发执行模式
func WithConcurrent(taskTypes ...string) SchedulerWrapperOption {
	return func(o *SchedulerWrapperOptions) {
		if o.taskModeMapping == nil {
			o.taskModeMapping = make(map[string]TaskExecutionMode)
		}
		for _, taskType := range taskTypes {
			o.taskModeMapping[taskType] = ModeConcurrent
		}
	}
}

// WithSequential 将指定的任务类型设置为顺序执行模式
func WithSequential(taskTypes ...string) SchedulerWrapperOption {
	return func(o *SchedulerWrapperOptions) {
		if o.taskModeMapping == nil {
			o.taskModeMapping = make(map[string]TaskExecutionMode)
		}
		for _, taskType := range taskTypes {
			o.taskModeMapping[taskType] = ModeSequential
		}
	}
}

// SchedulerWrapper 任务调度器包装器
// 作为客户使用任务管理所有功能的唯一入口
// 负责任务的注册、配置和调度

type SchedulerWrapper struct {
	Instance TaskScheduler
	options  *SchedulerWrapperOptions
}

// NewSchedulerWrapper 创建一个新的调度器包装器
func NewSchedulerWrapper(options ...SchedulerWrapperOption) *SchedulerWrapper {
	// 设置默认配置
	opts := &SchedulerWrapperOptions{
		defaultRetryCount: 3,                                  // 默认重试3次
		taskModeMapping:   make(map[string]TaskExecutionMode), // 初始化任务类型映射
	}

	// 应用用户提供的配置
	for _, option := range options {
		option(opts)
	}

	// 如果没有提供调度器，创建默认的混合调度器
	if opts.scheduler == nil {
		opts.scheduler = NewHybridScheduler()
	}

	// 设置最大并发数（如果配置了）
	if opts.maxConcurrentTasks > 0 {
		// 处理直接的并发调度器
		if cs, ok := opts.scheduler.(*concurrentScheduler); ok {
			cs.maxConcurrent = opts.maxConcurrentTasks
		}
		// 处理混合调度器中的并发部分
		if hs, ok := opts.scheduler.(*HybridScheduler); ok {
			if cs, ok := hs.concurrentScheduler.(*concurrentScheduler); ok {
				cs.maxConcurrent = opts.maxConcurrentTasks
			}
		}
	}

	// 应用任务类型执行模式设置到混合调度器
	if hs, ok := opts.scheduler.(*HybridScheduler); ok {
		for taskType, mode := range opts.taskModeMapping {
			switch mode {
			case ModeSequential:
				hs.WithSequential(taskType)
			case ModeConcurrent:
				hs.WithConcurrent(taskType)
			}
		}
	}

	// 保存配置到包装器实例中，方便后续使用
	wrapper := &SchedulerWrapper{
		Instance: opts.scheduler,
		options:  opts,
	}

	return wrapper
}

// RunTaskAt 提交一个在指定时间执行的任务
// 定时任务默认使用顺序模式执行
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
	task, err := w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithSchedule(at).
		WithHandler(handler))
	if err != nil {
		return err
	}
	return w.Instance.Schedule(task)
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
	task, err := w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithRecurring(interval).
		WithHandler(handler))
	if err != nil {
		return err
	}
	return w.Instance.Schedule(task)
}

// RunTask 提交一个立即执行的任务
// 根据任务类型和配置选择执行模式
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
	task, err := w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithHandler(handler))
	if err != nil {
		return err
	}
	return w.Instance.Schedule(task)
}

// RunTypedTask 提交一个指定类型的任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// handler: 任务处理函数
// 返回: 任务ID
func (w *SchedulerWrapper) RunTypedTaskWithID(taskId, taskType string, handler func(ctx context.Context) error) error {
	// 构建任务，默认支持重试
	task, err := w.buildTaskWithRetry(NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		WithHandler(handler))
	if err != nil {
		return err
	}
	return w.Instance.Schedule(task)
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

// RunTaskAfter 提交一个延迟执行的任务
func (w *SchedulerWrapper) RunTaskAfter(delay time.Duration, handler func(ctx context.Context) error) (string, error) {
	return w.RunTaskAt(time.Now().Add(delay), handler)
}

// RunTaskAfterWithID 提交一个延迟执行的任务
func (w *SchedulerWrapper) RunTaskAfterWithID(taskId string, delay time.Duration, handler func(ctx context.Context) error) error {
	return w.RunTaskWithIDAt(taskId, time.Now().Add(delay), handler)
}

// CancelTask 取消指定的任务
func (w *SchedulerWrapper) CancelTask(taskID string) error {
	return w.Instance.Cancel(taskID)
}

// SetTaskTypeMode 设置指定任务类型的执行模式
// 仅当底层调度器是混合调度器时有效
func (w *SchedulerWrapper) SetTaskTypeMode(taskType string, mode ExecutionMode) {
	if hybrid, ok := w.Instance.(*HybridScheduler); ok {
		switch mode {
		case Sequential:
			hybrid.WithSequential(taskType)
		case ConcurrentMode:
			hybrid.WithConcurrent(taskType)
		}
	}
}

// 修改wrapper.go中的类型断言，确保正确设置并发数
func (w *SchedulerWrapper) SetMaxConcurrentTasks(max int) error {
	if cs, ok := w.Instance.(interface{ SetMaxConcurrentTasks(int) error }); ok {
		return cs.SetMaxConcurrentTasks(max)
	}
	return fmt.Errorf("不支持设置最大并发数")
}

// Stop 停止任务调度器
func (w *SchedulerWrapper) Stop() error {
	return w.Instance.Stop()
}

// Close 优雅关闭任务调度器并清理资源（实现GracefulClose接口）
func (w *SchedulerWrapper) Close(ctx context.Context) error {
	return w.Instance.Close(ctx)
}

// GetTaskStatus 获取任务状态
func (w *SchedulerWrapper) GetTaskStatus(taskID string) (TaskStatus, error) {
	return w.Instance.GetTaskStatus(taskID)
}

// ListTasks 列出所有任务
func (w *SchedulerWrapper) ListTasks() []TaskInfo {
	return w.Instance.ListTasks()
}

// 注意：移除了GracefulClose方法，因为TaskScheduler接口中不包含该方法

// buildTaskWithRetry 构建任务，支持默认重试机制
// buildTaskWithRetry 构建带有重试功能的任务
// 使用通过Option模式注入的默认重试次数配置
func (w *SchedulerWrapper) buildTaskWithRetry(builder *TaskBuilder) (Task, error) {
	// 使用通过配置注入的默认重试次数
	return builder.WithRetry(w.options.defaultRetryCount).Build()
}
