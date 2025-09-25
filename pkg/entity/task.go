package entity

import (
	"fmt"
	"time"
)

// TaskStatus 任务状态枚举
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task 任务实体
// 位于整洁架构的核心层 - 实体层
// 包含任务的核心业务数据和规则

type Task struct {
	BaseEntity
	Type      string                 `json:"type"`
	Status    TaskStatus             `json:"status"`
	Payload   map[string]interface{} `json:"payload"`
	Result    map[string]interface{} `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Priority  int                    `json:"priority"`
	Retries   int                    `json:"retries"`
	MaxRetry  int                    `json:"max_retry"`
	StartedAt *time.Time             `json:"started_at,omitempty"`
	EndedAt   *time.Time             `json:"ended_at,omitempty"`
}

// 确保Task实现了Entity接口
var _ Entity = (*Task)(nil)

// NewTask 创建新任务实体
func NewTask(taskType string, payload map[string]interface{}) *Task {

	id := generateTaskID()
	baseEntity := NewBaseEntity(id)
	return &Task{
		BaseEntity: *baseEntity,
		Type:       taskType,
		Status:     TaskStatusPending,
		Payload:    payload,
		Priority:   0,
		Retries:    0,
		MaxRetry:   3,
	}
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d_%d", time.Now().UnixNano(), time.Now().UnixMilli())
}

// Start 标记任务开始
func (t *Task) Start() {
	now := time.Now()
	t.Status = TaskStatusRunning
	t.StartedAt = &now
	t.SetUpdatedAt(now)
}

// Complete 标记任务完成
func (t *Task) Complete(result map[string]interface{}) {
	now := time.Now()
	t.Status = TaskStatusCompleted
	t.Result = result
	t.EndedAt = &now
	t.SetUpdatedAt(now)
}

// Fail 标记任务失败
func (t *Task) Fail(err error) {
	now := time.Now()
	t.Status = TaskStatusFailed
	t.Error = err.Error()
	t.EndedAt = &now
	t.SetUpdatedAt(now)
	t.Retries++
}

// Cancel 取消任务
func (t *Task) Cancel() {
	now := time.Now()
	t.Status = TaskStatusCancelled
	t.EndedAt = &now
	t.SetUpdatedAt(now)
}

// CanRetry 检查是否可以重试
func (t *Task) CanRetry() bool {
	return t.Retries < t.MaxRetry && t.Status == TaskStatusFailed
}

// GetExecutionTime 获取执行时长
func (t *Task) GetExecutionTime() *time.Duration {
	if t.StartedAt == nil || t.EndedAt == nil {
		return nil
	}
	duration := t.EndedAt.Sub(*t.StartedAt)
	return &duration
}

// IsExpired 检查任务是否过期（超过24小时）
func (t *Task) IsExpired() bool {
	return time.Since(t.CreatedAt) > 24*time.Hour
}
