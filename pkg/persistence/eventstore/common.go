package eventstore

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/utils"
)

// EventMeta 事件元数据
type EventMeta struct {
	EventID       string    `json:"event_id"`
	AggregateID   string    `json:"aggregate_id"`
	AggregateType string    `json:"aggregate_type"`
	EventType     string    `json:"event_type"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	EventData     []byte    `json:"event_data"`
}

// EncodeEvent 将DomainEvent编码为字节数组
func EncodeEvent(event entity.DomainEvent) ([]byte, error) {
	// 创建事件元数据
	meta := EventMeta{
		AggregateID:   event.GetAggregateID(),
		AggregateType: event.GetAggregateType(),
		EventType:     event.GetEventType(),
		Version:       event.GetVersion(),
		CreatedAt:     event.GetCreatedAt(),
	}

	// 序列化事件本身
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}
	meta.EventData = data

	// 序列化元数据
	metaData, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event metadata: %w", err)
	}

	return metaData, nil
}

// DecodeEvent 将字节数组解码为DomainEvent
func DecodeEvent(data []byte) (entity.DomainEvent, error) {
	// 解析元数据
	var meta EventMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("反序列化事件元数据失败: %w", err)
	}

	// 构建基础事件对象
	event := &entity.BaseEvent{
		Id:            meta.EventID,
		AggregateID:   meta.AggregateID,
		AggregateType: meta.AggregateType,
		EventType:     meta.EventType,
		Version:       meta.Version,
		Timestamp:     meta.CreatedAt,
		Payload:       meta.EventData,
	}

	return event, nil
}

// EncodeEventData 编码事件数据为map格式
func EncodeEventData(event entity.DomainEvent) (map[string]interface{}, error) {
	// 序列化事件为JSON
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	// 解析为map
	var eventData map[string]interface{}
	if err := json.Unmarshal(data, &eventData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event to map: %w", err)
	}

	return eventData, nil
}

// DecodeEventFromMap 从map中解码事件
func DecodeEventFromMap(eventData map[string]interface{}) entity.DomainEvent {
	// 创建基础事件对象
	event := &entity.BaseEvent{
		Id:            utils.ToString(eventData["id"]),
		AggregateID:   utils.ToString(eventData["aggregate_id"]),
		EventType:     utils.ToString(eventData["event_type"]),
		AggregateType: utils.ToString(eventData["aggregate_type"]),
		Timestamp:     utils.ParseTime(eventData["timestamp"]),
		Version:       utils.ToInt(eventData["version"]),
		Payload:       eventData,
	}

	return event
}
