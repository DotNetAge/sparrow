package entity

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SchemaVersioned 可选接口：事件可声明自身的契约版本
// 未实现该接口的事件按版本 1 发布
type SchemaVersioned interface {
	GetSchemaVersion() int
}

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
	SchemaVersion int         `json:"schema_version"` // 事件契约版本，缺失视为 1
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

// SetVersion 设置事件版本
func (e *BaseEvent) SetVersion(version int) {
	e.Version = version
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
		SchemaVersion: 1,
	}
}

// currentSchemaVersion 当前支持的契约版本，仅接受不超过该版本的存量事件
const currentSchemaVersion = 1

func UnmarshalEvent[T any](event *BaseEvent) (T, error) {
	var payload T
	if event == nil {
		return payload, fmt.Errorf("unmarshal event: base event is nil")
	}
	if event.SchemaVersion > currentSchemaVersion {
		return payload, fmt.Errorf("unmarshal payload: 事件契约版本 %d 高于当前支持版本 %d，请升级消费方以支持新 schema_version", event.SchemaVersion, currentSchemaVersion)
	}
	if event.Payload == nil {
		// 2026-11-27 ray：Payload 缺失显式返回 error，禁止 (nil, nil) 穿透到下游 handler
		// 之前 Payload=nil 时 payload=T{零值}（对指针类型是 nil），调用方拿到 err=nil 直接
		// 把 nil event 传给业务 handler → NPE panic 炸进程。
		// 设计意图：Payload 缺失的事件应该走 handleMessage 的 error 路径 → msg.Nak()
		// → 增加 JetStream 投递计数 → MaxDeliver 达到后自动转死信流，而不是静默穿透。
		return payload, fmt.Errorf("unmarshal payload: event %s aggregate=%s payload 缺失（可能是发布端未序列化或版本不匹配）", event.EventType, event.AggregateID)
	}
	data, err := base64.StdEncoding.DecodeString(event.Payload.(string))
	if err != nil {
		return payload, fmt.Errorf("decode payload: %w", err)
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, fmt.Errorf("unmarshal payload: %w", err)
	}
	return payload, nil
}
