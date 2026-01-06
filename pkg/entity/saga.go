package entity

import (
	"fmt"
	"time"
)

// Step 事务步骤实体
// 位于整洁架构的最内层 - 实体层
// 不包含任何业务逻辑，仅定义数据结构

type Step struct {
	Name       string                 `json:"name"`
	Handler    string                 `json:"handler"`
	Payload    map[string]interface{} `json:"payload"`
	Compensate string                 `json:"compensate,omitempty"`
}

// Transaction 事务实体
// 位于整洁架构的最内层 - 实体层
// 实现Entity接口以便使用基础仓储

type Transaction struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Steps   []Step    `json:"steps"`
	Current int       `json:"current"`
	Status  string    `json:"status"`
	Created time.Time `json:"created_at"`
	Updated time.Time `json:"updated_at"`
}

// GetID 返回实体ID
func (t *Transaction) GetID() string {
	return t.ID
}

// SetID 设置实体ID
func (t *Transaction) SetID(id string) {
	t.ID = id
}

// GetCreatedAt 返回创建时间
func (t *Transaction) GetCreatedAt() time.Time {
	return t.Created
}

// GetUpdatedAt 返回更新时间
func (t *Transaction) GetUpdatedAt() time.Time {
	return t.Updated
}

// SetUpdatedAt 设置更新时间
func (t *Transaction) SetUpdatedAt(updated time.Time) {
	t.Updated = updated
}

// TransactionStatus 事务状态常量
const (
	StatusPending     = "pending"
	StatusRunning     = "running"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusCompensated = "compensated"
)

// NewTransaction 创建新事务实体的工厂函数
// 这是一个纯函数，不包含任何副作用
func NewTransaction(name string, steps []Step) *Transaction {
	now := time.Now()
	return &Transaction{
		ID:      generateID(now),
		Name:    name,
		Steps:   steps,
		Current: 0,
		Status:  StatusPending,
		Created: now,
		Updated: now,
	}
}

// generateID 生成事务ID的辅助函数
func generateID(t time.Time) string {
	return fmt.Sprintf("tx_%d", t.UnixNano())
}
