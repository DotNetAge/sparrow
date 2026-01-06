package entity

import (
	"time"
)

// Entity 定义实体接口约束
type Entity interface {
	// GetID 获取ID
	GetID() string
	// SetID 设置ID
	SetID(id string)
	// GetCreatedAt 获取创建时间
	GetCreatedAt() time.Time
	// GetUpdatedAt 获取更新时间
	GetUpdatedAt() time.Time
	// SetUpdatedAt 设置更新时间
	SetUpdatedAt(t time.Time)
}
