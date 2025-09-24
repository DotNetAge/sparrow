package entity

import (
	"encoding/json"
	"time"
)

// Snapshot 快照接口
type Snapshot interface {
	GetAggregateID() string
	GetAggregateType() string  
	GetVersion() int
	GetCreatedAt() time.Time
	GetState() interface{}     // 聚合状态
	Marshal() ([]byte, error)  // 序列化
	Unmarshal(data []byte) error // 反序列化
}

// SnapshotData 泛型快照实现
type SnapshotData[T any] struct {
	AggregateID   string    `json:"aggregate_id"`
	AggregateType string    `json:"aggregate_type"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	State         T         `json:"state"`
}

// SnapshotData方法实现
func (s *SnapshotData[T]) GetAggregateID() string { return s.AggregateID }
func (s *SnapshotData[T]) GetAggregateType() string { return s.AggregateType }
func (s *SnapshotData[T]) GetVersion() int { return s.Version }
func (s *SnapshotData[T]) GetCreatedAt() time.Time { return s.CreatedAt }
func (s *SnapshotData[T]) GetState() interface{} { return s.State }
func (s *SnapshotData[T]) Marshal() ([]byte, error) { return json.Marshal(s) }
func (s *SnapshotData[T]) Unmarshal(data []byte) error { return json.Unmarshal(data, s) }

// NewSnapshotData 创建新的泛型快照
func NewSnapshotData[T any](aggregateID, aggregateType string, version int, state T) *SnapshotData[T] {
	return &SnapshotData[T]{
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		Version:       version,
		CreatedAt:     time.Now(),
		State:         state,
	}
}