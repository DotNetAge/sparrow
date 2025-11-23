package tasks

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// TaskBuilder 任务构建器
// 简化版：直接在Task中集成重试功能

type TaskBuilder struct {
	id         string
	typeName   string
	handler    func(ctx context.Context) error
	onComplete func(ctx context.Context, err error)
	onCancel   func(ctx context.Context)
	schedule   TaskSchedule
	timeout    time.Duration
	maxRetries int
	priority   int
}

// NewTask 创建新的任务构建器
func NewTask() *TaskBuilder {
	return &TaskBuilder{
		id:       uuid.New().String(),
		schedule: TaskSchedule{Type: ScheduleTypeImmediate},
	}
}

// NewTaskBuilder 创建新的任务构建器（保持向后兼容）
func NewTaskBuilder() *TaskBuilder {
	return NewTask()
}

// WithID 设置任务ID
func (b *TaskBuilder) WithID(id string) *TaskBuilder {
	b.id = id
	return b
}

// WithType 设置任务类型
func (b *TaskBuilder) WithType(typeName string) *TaskBuilder {
	b.typeName = typeName
	return b
}

// WithHandler 设置任务处理函数
func (b *TaskBuilder) WithHandler(handler func(ctx context.Context) error) *TaskBuilder {
	b.handler = handler
	return b
}

// WithOnComplete 设置任务完成回调
func (b *TaskBuilder) WithOnComplete(onComplete func(ctx context.Context, err error)) *TaskBuilder {
	b.onComplete = onComplete
	return b
}

// WithOnCancel 设置任务取消回调
func (b *TaskBuilder) WithOnCancel(onCancel func(ctx context.Context)) *TaskBuilder {
	b.onCancel = onCancel
	return b
}

// WithSchedule 设置任务调度时间
func (b *TaskBuilder) WithSchedule(at time.Time) *TaskBuilder {
	b.schedule.Type = ScheduleTypeTimed
	b.schedule.At = at
	return b
}

// WithRecurring 设置定期任务
func (b *TaskBuilder) WithRecurring(interval time.Duration) *TaskBuilder {
	b.schedule.Type = ScheduleTypeRecurring
	b.schedule.Interval = interval
	return b
}

// WithTimeout 设置任务超时时间
func (b *TaskBuilder) WithTimeout(timeout time.Duration) *TaskBuilder {
	b.timeout = timeout
	return b
}

// WithRetry 设置重试次数
func (b *TaskBuilder) WithRetry(maxRetries int) *TaskBuilder {
	b.maxRetries = maxRetries
	return b
}

// WithPriority 设置任务优先级 (1-10，1为最高优先级)
func (b *TaskBuilder) WithPriority(priority int) *TaskBuilder {
	// 限制优先级范围在1-10之间
	if priority < 1 {
		priority = 1
	} else if priority > 10 {
		priority = 10
	}
	b.priority = priority
	return b
}

// Build 构建任务
func (b *TaskBuilder) Build() (Task, error) {
	if b.handler == nil {
		return nil, errors.New("任务处理器不能为空")
	}

	// 验证任务调度时间
	if b.schedule.Type == ScheduleTypeTimed && !b.schedule.At.After(time.Now()) {
		return nil, errors.New("定时任务的执行时间必须大于当前时间")
	}

	// 验证重试参数
	if b.maxRetries < 0 {
		return nil, errors.New("重试次数不能为负数")
	}

	// 验证超时时间
	if b.timeout < 0 {
		return nil, errors.New("超时时间不能为负数")
	}

	// 计算执行时间
	execTime := time.Time{}
	switch b.schedule.Type {
	case ScheduleTypeImmediate:
		execTime = time.Now()
	case ScheduleTypeTimed:
		execTime = b.schedule.At
	case ScheduleTypeRecurring:
		execTime = time.Now().Add(b.schedule.Interval)
	}

	return &builtTask{
		taskInfo: &TaskInfo{
			ID:         b.id,
			Type:       b.typeName,
			Status:     TaskStatusWaiting,
			Schedule:   execTime,
			MaxRetries: b.maxRetries,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		handler:    b.handler,
		onComplete: b.onComplete,
		onCancel:   b.onCancel,
		schedule:   b.schedule,
		timeout:    b.timeout,
	}, nil
}

// builtTask 内部任务实现
// 同时实现Task和TaskInfoProvider接口
type builtTask struct {
	taskInfo   *TaskInfo
	handler    func(ctx context.Context) error
	onComplete func(ctx context.Context, err error)
	onCancel   func(ctx context.Context)
	schedule   TaskSchedule
	timeout    time.Duration
}

// 实现Task接口的方法
func (t *builtTask) ID() string {
	return t.taskInfo.ID
}

func (t *builtTask) Type() string {
	return t.taskInfo.Type
}

// GetTimeout 返回任务超时时间
func (t *builtTask) GetTimeout() time.Duration {
	return t.timeout
}

func (t *builtTask) Schedule() time.Time {
	return t.taskInfo.Schedule
}

func (t *builtTask) Handler() func(ctx context.Context) error {
	return t.handler
}

func (t *builtTask) OnComplete() func(ctx context.Context, err error) {
	return t.onComplete
}

func (t *builtTask) OnCancel() func(ctx context.Context) {
	return t.onCancel
}

func (t *builtTask) IsRecurring() bool {
	return t.schedule.Type == ScheduleTypeRecurring
}

func (t *builtTask) GetInterval() time.Duration {
	return t.schedule.Interval
}

func (t *builtTask) SetSchedule(nextTime time.Time) {
	t.taskInfo.Schedule = nextTime
}

// 实现TaskInfoProvider接口
func (t *builtTask) TaskInfo() *TaskInfo {
	return t.taskInfo
}
