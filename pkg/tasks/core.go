package tasks

import (
	"context"
	"time"

	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// TaskScheduler 任务调度器接口
type TaskScheduler interface {
	usecase.GracefulClose                            // 必须实现GracefulClose接口，用于优雅关闭和资源清理
	usecase.Startable                                // 必须实现Startable接口，由主程序启动调度器
	Schedule(task Task) error                        // Schedule 调度一个任务
	Stop() error                                     // Stop 停止任务调度器
	Cancel(taskID string) error                      // Cancel 取消一个任务
	GetTaskStatus(taskID string) (TaskStatus, error) // GetTaskStatus 获取任务状态
	ListTasks() []TaskInfo                           // ListTasks 列出所有任务
	SetMaxConcurrentTasks(max int) error             // SetMaxConcurrentTasks 设置最大并发任务数
}

// TaskInfo 任务信息
type TaskInfo struct {
	ID          string     `json:"id"`            // 任务ID
	Type        string     `json:"type"`          // 任务类型,用于关联任务的执行模式，任务调度器中根据类型选择执行策略
	Status      TaskStatus `json:"status"`        // 任务状态
	Schedule    time.Time  `json:"schedule"`      // 任务调度时间
	CreatedAt   time.Time  `json:"created_at"`    // 任务创建时间
	UpdatedAt   time.Time  `json:"updated_at"`    // 任务更新时间
	RetryCount  int        `json:"retry_count"`   // 当前重试次数
	MaxRetries  int        `json:"max_retries"`   // 最大重试次数
	LastError   string     `json:"last_error"`    // 最后一次错误信息
	NextRetryAt time.Time  `json:"next_retry_at"` // 下次重试时间
	TTL         time.Time  `json:"ttl"`           // 任务过期时间 -1 表示永不过期
}

// Task 任务接口
type Task interface {
	ID() string                                       // ID 返回任务的唯一标识符
	Type() string                                     // Type 返回任务的类型
	Schedule() time.Time                              // Schedule 返回任务的执行时间
	Handler() func(ctx context.Context) error         // Handler 返回任务的处理方法
	OnComplete() func(ctx context.Context, err error) // OnComplete 返回任务完成后的回调方法
	OnCancel() func(ctx context.Context)              // OnCancel 返回任务取消后的回调方法
	IsRecurring() bool                                // IsRecurring 判断任务是否为周期性任务
	GetInterval() time.Duration                       // GetInterval 获取任务的执行间隔
	GetTimeout() time.Duration                        // GetTimeout 获取任务的超时时间
}

// TaskInfoProvider 任务信息提供接口
type TaskInfoProvider interface {
	// TaskInfo 返回任务信息
	TaskInfo() *TaskInfo
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusUnknown    TaskStatus = "unknown"     // 未知状态
	TaskStatusWaiting    TaskStatus = "waiting"     // 等待中
	TaskStatusRunning    TaskStatus = "running"     // 执行中
	TaskStatusCompleted  TaskStatus = "completed"   // 已完成
	TaskStatusCancelled  TaskStatus = "cancelled"   // 已取消
	TaskStatusFailed     TaskStatus = "failed"      // 执行失败
	TaskStatusRetrying   TaskStatus = "retrying"    // 重试中
	TaskStatusDeadLetter TaskStatus = "dead_letter" // 死信
)

// TaskScheduleType 任务调度类型
type TaskScheduleType int

const (
	// ScheduleTypeImmediate 即时执行
	ScheduleTypeImmediate TaskScheduleType = iota // 即时执行
	ScheduleTypeTimed                             // 定时执行
	ScheduleTypeRecurring                         // 周期性执行
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
	ConcurrentMode ExecutionMode = iota // 并发执行
	Sequential                          // 顺序执行
)

// ImmediateExecution 创建即时执行的调度配置
func ImmediateExecution() TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeImmediate}
}

// ScheduleAt 创建在指定时间执行的调度配置
func ScheduleAt(at time.Time) TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeTimed, At: at}
}

// ScheduleRecurring 创建周期性执行的调度配置
func ScheduleRecurring(interval time.Duration) TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeRecurring, Interval: interval}
}
