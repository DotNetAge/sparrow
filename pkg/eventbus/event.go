package eventbus

import (
	"time"

	"github.com/google/uuid"
)

// Event 通用事件实现，用于非领域事件场景
// 提供了基础的事件属性，不包含领域特定的聚合根信息
type Event struct {
	Id        string      `json:"id"`
	EventType string      `json:"event_type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"` // 事件负载数据
}

// NewEvent 创建通用事件
func NewEvent(eventType string, payload interface{}) *Event {
	return &Event{
		Id:        uuid.New().String(),
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}
