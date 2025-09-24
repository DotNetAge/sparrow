package entity

import (
	"time"
)

// BaseEntity 基础实体结构体，提供标准字段和方法
// 所有实体都应该嵌入这个结构体来实现Entity接口
type BaseEntity struct {
	Id        string    `json:"id"`         // 实体唯一标识
	CreatedAt time.Time `json:"created_at"` // 创建时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// GetID 返回实体ID
func (e *BaseEntity) GetID() string {
	return e.Id
}

// SetID 设置实体ID
func (e *BaseEntity) SetID(id string) {
	e.Id = id
}

// GetCreatedAt 返回创建时间
func (e *BaseEntity) GetCreatedAt() time.Time {
	return e.CreatedAt
}

// GetUpdatedAt 返回更新时间
func (e *BaseEntity) GetUpdatedAt() time.Time {
	return e.UpdatedAt
}

// SetUpdatedAt 设置更新时间
func (e *BaseEntity) SetUpdatedAt(t time.Time) {
	e.UpdatedAt = t
}

// NewBaseEntity 创建新的基础实体
func NewBaseEntity(id string) *BaseEntity {
	now := time.Now()
	return &BaseEntity{
		Id:        id,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
