package entity

import (
	"time"

	"github.com/google/uuid"
)

// BaseEvent 基础领域事件结构体，实现DomainEvent接口
// NOTES: 一切的BaseEvent都不可以重新实现GetAggregateID()方法，以及重新定义 AggregateID 字段,否则会引发系统性异常。
// 除非所有事件处理机制都重新实现，系统会默认从JSON或HTTP的Body中将id转换成AggregateID字段。
type BaseEvent struct {
	Id            string      `json:"id"`
	AggregateID   string      `json:"aggregate_id"`
	EventType     string      `json:"event_type"`
	AggregateType string      `json:"aggregate_type"`
	Timestamp     time.Time   `json:"timestamp"`
	Version       int         `json:"version"`
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
