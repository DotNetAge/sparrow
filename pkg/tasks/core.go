package tasks

import (
	"context"
	"time"

	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// TaskScheduler 任务调度器接口
type TaskScheduler interface {
	// GracefulClose 优雅关闭任务调度器
	usecase.GracefulClose
	usecase.Startable
	// Schedule 调度一个任务
	Schedule(task Task) error
	// Start 启动任务调度器
	// Start() error
	// Stop 停止任务调度器
	Stop() error
	// Cancel 取消指定的任务
	Cancel(taskID string) error
	// GetTaskStatus 获取任务状态
	GetTaskStatus(taskID string) (TaskStatus, error)
	// ListTasks 列出所有任务
	ListTasks() []TaskInfo
	// SetMaxConcurrentTasks 设置最大并发任务数
	SetMaxConcurrentTasks(max int) error
	// SetExecutionMode 设置执行模式
	SetExecutionMode(mode ExecutionMode) error
	// GetExecutionMode 获取当前执行模式
	GetExecutionMode() ExecutionMode
}

// TaskInfo 任务信息
type TaskInfo struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Status    TaskStatus `json:"status"`
	Schedule  time.Time  `json:"schedule"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Task 任务接口
type Task interface {
	// ID 返回任务的唯一标识符
	ID() string
	// Type 返回任务的类型
	Type() string
	// Schedule 返回任务的执行时间
	Schedule() time.Time
	// Handler 返回任务的处理方法
	Handler() func(ctx context.Context) error
	// OnComplete 返回任务完成后的回调方法
	OnComplete() func(ctx context.Context, err error)
	// OnCancel 返回任务取消后的回调方法
	OnCancel() func(ctx context.Context)
	// IsRecurring 判断任务是否为周期性任务
	IsRecurring() bool
	// GetInterval 获取任务的执行间隔
	GetInterval() time.Duration
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusWaiting   TaskStatus = "waiting"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusFailed    TaskStatus = "failed"
)

// TaskScheduleType 任务调度类型
type TaskScheduleType int

const (
	// ScheduleTypeImmediate 即时执行
	ScheduleTypeImmediate TaskScheduleType = iota
	// ScheduleTypeOnce 一次性执行
	ScheduleTypeOnce
	// ScheduleTypeRecurring 周期性执行
	ScheduleTypeRecurring
)

// TaskSchedule 任务调度配置
type TaskSchedule struct {
	Type     TaskScheduleType
	At       time.Time     // 一次性执行的时间点
	Interval time.Duration // 周期性执行的间隔
}

// ExecutionMode 任务执行模式
type ExecutionMode int

const (
	// ExecutionModeConcurrent 并发执行模式（默认）
	ExecutionModeConcurrent ExecutionMode = iota
	// ExecutionModeSequential 顺序执行模式
	ExecutionModeSequential
	// ExecutionModePipeline 流水线执行模式
	ExecutionModePipeline
)

// ImmediateExecution 创建即时执行的调度配置
func ImmediateExecution() TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeImmediate}
}

// ScheduleAt 创建在指定时间执行的调度配置
func ScheduleAt(at time.Time) TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeOnce, At: at}
}

// ScheduleRecurring 创建周期性执行的调度配置
func ScheduleRecurring(interval time.Duration) TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeRecurring, Interval: interval}
}
