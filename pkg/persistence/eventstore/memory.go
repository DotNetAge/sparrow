package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// MemoryEventStore 内存事件存储实现
type MemoryEventStore struct {
	mutex     sync.RWMutex
	events    map[string][]EventMeta   // aggregateID -> events
	versions  map[string]int           // aggregateID -> current version
	snapshots map[string]*snapshotData // aggregateID -> snapshot
	logger    *logger.Logger
}

type snapshotData struct {
	Data    interface{}
	Version int
	Created time.Time
}

var _ usecase.EventStore = (*MemoryEventStore)(nil)

// NewMemoryEventStore 创建内存事件存储实例
func NewMemoryEventStore(logger *logger.Logger) usecase.EventStore {
	return &MemoryEventStore{
		events:    make(map[string][]EventMeta),
		versions:  make(map[string]int),
		snapshots: make(map[string]*snapshotData),
		logger:    logger,
	}
}

// SaveEvents 保存事件
func (s *MemoryEventStore) SaveEvents(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查当前版本
	currentVersion := s.versions[aggregateID]
	if expectedVersion != -1 && currentVersion != expectedVersion {
		return &errs.EventStoreError{
			Type:      "concurrency_conflict",
			Message:   fmt.Sprintf("expected version %d, got %d", expectedVersion, currentVersion),
			Aggregate: aggregateID,
		}
	}

	// 保存事件
	eventList, exists := s.events[aggregateID]
	if !exists {
		eventList = make([]EventMeta, 0)
	}

	for i, evt := range events {
		version := currentVersion + i + 1

		// 序列化事件
		eventData, err := json.Marshal(evt)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to marshal event", "aggregate_id", aggregateID, "event_type", evt.GetEventType(), "error", err)
			}
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		// 创建事件元数据
		eventMeta := EventMeta{
			AggregateID:   aggregateID,
			AggregateType: evt.GetAggregateType(),
			EventType:     evt.GetEventType(),
			Version:       version,
			CreatedAt:     evt.GetCreatedAt(),
			EventData:     eventData,
		}

		eventList = append(eventList, eventMeta)
	}

	// 更新事件列表和版本
	s.events[aggregateID] = eventList
	s.versions[aggregateID] = currentVersion + len(events)

	return nil
}

// GetEvents 获取指定聚合根的所有事件
func (s *MemoryEventStore) GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.getEventsInternal(aggregateID, 0)
}

// GetEventsFromVersion 从指定版本开始获取事件
func (s *MemoryEventStore) GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.getEventsInternal(aggregateID, fromVersion)
}

// getEventsInternal 内部方法：获取事件
func (s *MemoryEventStore) getEventsInternal(aggregateID string, fromVersion int) ([]entity.DomainEvent, error) {
	eventList, exists := s.events[aggregateID]
	if !exists {
		return []entity.DomainEvent{}, nil
	}

	// 过滤版本
	result := make([]entity.DomainEvent, 0)
	for _, eventMeta := range eventList {
		if eventMeta.Version >= fromVersion {
			event, err := DecodeEvent(eventMeta.EventData)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event: %w", err)
			}
			result = append(result, event)
		}
	}

	return result, nil
}

// GetEventsByType 按类型获取事件
func (s *MemoryEventStore) GetEventsByType(ctx context.Context, eventType string) ([]entity.DomainEvent, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]entity.DomainEvent, 0)
	for _, eventList := range s.events {
		for _, eventMeta := range eventList {
			if eventMeta.EventType == eventType {
				event, err := DecodeEvent(eventMeta.EventData)
				if err != nil {
					return nil, fmt.Errorf("failed to decode event: %w", err)
				}
				result = append(result, event)
			}
		}
	}

	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].GetCreatedAt().Before(result[j].GetCreatedAt())
	})

	return result, nil
}

// Load 加载聚合根
func (s *MemoryEventStore) Load(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 检查是否有快照
	snapshot, exists := s.snapshots[aggregateID]
	if exists {
		// 从快照加载
		if err := aggregate.LoadFromSnapshot(snapshot.Data); err != nil {
			return fmt.Errorf("failed to load from snapshot: %w", err)
		}
		// 应用快照之后的事件
		events, err := s.getEventsInternal(aggregateID, snapshot.Version+1)
		if err != nil {
			return err
		}
		return aggregate.LoadFromEvents(events)
	}

	// 从所有事件加载
	events, err := s.getEventsInternal(aggregateID, 0)
	if err != nil {
		return err
	}
	return aggregate.LoadFromEvents(events)
}

// GetEventsByTimeRange 按时间范围获取事件
func (s *MemoryEventStore) GetEventsByTimeRange(ctx context.Context, aggregateID string, fromTime, toTime time.Time) ([]entity.DomainEvent, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	eventList, exists := s.events[aggregateID]
	if !exists {
		return []entity.DomainEvent{}, nil
	}

	result := make([]entity.DomainEvent, 0)
	for _, eventMeta := range eventList {
		if (eventMeta.CreatedAt.After(fromTime) || eventMeta.CreatedAt.Equal(fromTime)) &&
			(eventMeta.CreatedAt.Before(toTime) || eventMeta.CreatedAt.Equal(toTime)) {
			event, err := DecodeEvent(eventMeta.EventData)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event: %w", err)
			}
			result = append(result, event)
		}
	}

	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].GetCreatedAt().Before(result[j].GetCreatedAt())
	})

	return result, nil
}

// GetEventsWithPagination 分页获取事件
func (s *MemoryEventStore) GetEventsWithPagination(ctx context.Context, aggregateID string, limit, offset int) ([]entity.DomainEvent, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	eventList, exists := s.events[aggregateID]
	if !exists {
		return []entity.DomainEvent{}, nil
	}

	// 按版本排序
	sortedEvents := make([]EventMeta, len(eventList))
	copy(sortedEvents, eventList)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Version < sortedEvents[j].Version
	})

	// 应用分页
	if offset >= len(sortedEvents) {
		return []entity.DomainEvent{}, nil
	}

	end := offset + limit
	if end > len(sortedEvents) {
		end = len(sortedEvents)
	}

	result := make([]entity.DomainEvent, 0, end-offset)
	for i := offset; i < end; i++ {
		event, err := DecodeEvent(sortedEvents[i].EventData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}
		result = append(result, event)
	}

	return result, nil
}

// GetAggregateVersion 获取聚合根当前版本
func (s *MemoryEventStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.versions[aggregateID], nil
}

// SaveSnapshot 保存快照
func (s *MemoryEventStore) SaveSnapshot(ctx context.Context, aggregateID string, snapshot interface{}, version int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查版本是否有效
	currentVersion := s.versions[aggregateID]
	if version > currentVersion {
		return &errs.EventStoreError{
			Type:      "invalid_version",
			Message:   fmt.Sprintf("snapshot version %d exceeds current version %d", version, currentVersion),
			Aggregate: aggregateID,
		}
	}

	// 保存快照
	s.snapshots[aggregateID] = &snapshotData{
		Data:    snapshot,
		Version: version,
		Created: time.Now(),
	}

	return nil
}

// GetLatestSnapshot 获取最新快照
func (s *MemoryEventStore) GetLatestSnapshot(ctx context.Context, aggregateID string) (interface{}, int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	snapshot, exists := s.snapshots[aggregateID]
	if !exists {
		return nil, 0, nil
	}

	return snapshot.Data, snapshot.Version, nil
}

// SaveEventsBatch 批量保存事件
func (s *MemoryEventStore) SaveEventsBatch(ctx context.Context, events map[string][]entity.DomainEvent) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for aggregateID, evtList := range events {
		// 使用 -1 表示不检查版本
		if err := s.saveEventsInternal(ctx, aggregateID, evtList, -1); err != nil {
			return err
		}
	}

	return nil
}

// saveEventsInternal 内部方法：保存事件（不加锁）
func (s *MemoryEventStore) saveEventsInternal(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	// 检查当前版本
	currentVersion := s.versions[aggregateID]
	if expectedVersion != -1 && currentVersion != expectedVersion {
		return &errs.EventStoreError{
			Type:      "concurrency_conflict",
			Message:   fmt.Sprintf("expected version %d, got %d", expectedVersion, currentVersion),
			Aggregate: aggregateID,
		}
	}

	// 保存事件
	eventList, exists := s.events[aggregateID]
	if !exists {
		eventList = make([]EventMeta, 0)
	}

	for i, evt := range events {
		version := currentVersion + i + 1

		// 序列化事件
		eventData, err := json.Marshal(evt)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to marshal event", "aggregate_id", aggregateID, "event_type", evt.GetEventType(), "error", err)
			}
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		// 创建事件元数据
		eventMeta := EventMeta{
			AggregateID:   aggregateID,
			AggregateType: evt.GetAggregateType(),
			EventType:     evt.GetEventType(),
			Version:       version,
			CreatedAt:     evt.GetCreatedAt(),
			EventData:     eventData,
		}

		eventList = append(eventList, eventMeta)
	}

	// 更新事件列表和版本
	s.events[aggregateID] = eventList
	s.versions[aggregateID] = currentVersion + len(events)

	return nil
}

// Close 关闭事件存储（内存实现无操作）
func (s *MemoryEventStore) Close() error {
	return nil
}
