package entity

import (
	"time"
	"github.com/google/uuid"
)

// BaseEvent 基础领域事件结构体，实现DomainEvent接口
type BaseEvent struct {
	Id            string    `json:"id"`
	AggregateID   string    `json:"aggregate_id"`
	EventType     string    `json:"event_type"`
	AggregateType string    `json:"aggregate_type"`
	Timestamp     time.Time `json:"timestamp"`
	Version       int       `json:"version"`
	Payload       interface{} `json:"payload,omitempty"` // 事件负载数据
}

// 实现Event接口
func (e *BaseEvent) GetEventID() string {
	return e.Id
}

func (e *BaseEvent) GetEventType() string {
	return e.EventType
}

func (e *BaseEvent) GetAggregateID() string {
	return e.AggregateID
}

func (e *BaseEvent) GetAggregateType() string {
	return e.AggregateType
}

func (e *BaseEvent) GetCreatedAt() time.Time {
	return e.Timestamp
}

func (e *BaseEvent) GetVersion() int {
	return e.Version
}

// NewBaseEvent 创建基础事件
func NewBaseEvent(aggregateID, eventType, aggregateType string, version int) *BaseEvent {
	return &BaseEvent{
		Id:            uuid.New().String(),
		AggregateID:   aggregateID,
		EventType:     eventType,
		AggregateType: aggregateType,
		Timestamp:     time.Now(),
		Version:       version,
	}
}