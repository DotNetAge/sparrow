package entity

import (
	"time"
	"github.com/google/uuid"
)

// GenericEvent 通用事件实现，用于非领域事件场景
// 提供了基础的事件属性，不包含领域特定的聚合根信息
type GenericEvent struct {
	Id        string      `json:"id"`
	EventType string      `json:"event_type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"` // 事件负载数据
}

// NewGenericEvent 创建通用事件
func NewGenericEvent(eventType string, payload interface{}) *GenericEvent {
	return &GenericEvent{
		Id:        uuid.New().String(),
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

// GetEventID 实现Event接口
func (e *GenericEvent) GetEventID() string {
	return e.Id
}

// GetEventType 实现Event接口
func (e *GenericEvent) GetEventType() string {
	return e.EventType
}

// GetCreatedAt 实现Event接口
func (e *GenericEvent) GetCreatedAt() time.Time {
	return e.Timestamp
}