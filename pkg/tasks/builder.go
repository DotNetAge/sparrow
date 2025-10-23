package tasks

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TaskBuilder 任务构建器
type TaskBuilder struct {
	id         string
	typeName   string
	schedule   TaskSchedule
	handler    func(ctx context.Context) error
	onComplete func(ctx context.Context, err error)
	onCancel   func(ctx context.Context)
}

// NewTaskBuilder 创建新的任务构建器
func NewTaskBuilder() *TaskBuilder {
	return &TaskBuilder{
		id:       uuid.New().String(),
		typeName: "default",
	}
}

// WithID 设置任务ID
func (b *TaskBuilder) WithID(id string) *TaskBuilder {
	b.id = id
	return b
}

// WithType 设置任务类型
func (b *TaskBuilder) WithType(taskType string) *TaskBuilder {
	b.typeName = taskType
	return b
}

// Immediate 设置为即时执行
func (b *TaskBuilder) Immediate() *TaskBuilder {
	b.schedule = ImmediateExecution()
	return b
}

// ScheduleAt 设置为在指定时间执行
func (b *TaskBuilder) ScheduleAt(at time.Time) *TaskBuilder {
	b.schedule = ScheduleAt(at)
	return b
}

// ScheduleRecurring 设置为周期性执行
func (b *TaskBuilder) ScheduleRecurring(interval time.Duration) *TaskBuilder {
	b.schedule = ScheduleRecurring(interval)
	return b
}

// WithHandler 设置任务处理函数
func (b *TaskBuilder) WithHandler(handler func(ctx context.Context) error) *TaskBuilder {
	b.handler = handler
	return b
}

// WithOnComplete 设置任务完成回调
func (b *TaskBuilder) WithOnComplete(callback func(ctx context.Context, err error)) *TaskBuilder {
	b.onComplete = callback
	return b
}

// WithOnCancel 设置任务取消回调
func (b *TaskBuilder) WithOnCancel(callback func(ctx context.Context)) *TaskBuilder {
	b.onCancel = callback
	return b
}

// Build 构建任务
func (b *TaskBuilder) Build() Task {
	if b.handler == nil {
		panic("任务处理函数不能为空")
	}

	// 计算执行时间
	execTime := time.Time{}
	switch b.schedule.Type {
	case ScheduleTypeImmediate:
		execTime = time.Now()
	case ScheduleTypeOnce:
		execTime = b.schedule.At
	case ScheduleTypeRecurring:
		execTime = time.Now().Add(b.schedule.Interval)
	}

	return &builtTask{
		id:         b.id,
		typeName:   b.typeName,
		execTime:   execTime,
		handler:    b.handler,
		onComplete: b.onComplete,
		onCancel:   b.onCancel,
		schedule:   b.schedule,
	}
}

// builtTask 内部任务实现
type builtTask struct {
	id         string
	typeName   string
	execTime   time.Time
	handler    func(ctx context.Context) error
	onComplete func(ctx context.Context, err error)
	onCancel   func(ctx context.Context)
	schedule   TaskSchedule
}

// 实现Task接口的方法
func (t *builtTask) ID() string {
	return t.id
}

func (t *builtTask) Type() string {
	return t.typeName
}

func (t *builtTask) Schedule() time.Time {
	return t.execTime
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
