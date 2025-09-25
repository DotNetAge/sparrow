package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/usecase"

	"github.com/redis/go-redis/v9"
)

// RedisEventStore Redis事件存储实现
type RedisEventStore struct {
	client *redis.Client
	prefix string
	logger *logger.Logger
}

var _ usecase.EventStore = (*RedisEventStore)(nil)

// NewRedisEventStore 创建Redis事件存储实例
func NewRedisEventStore(client *redis.Client, prefix string, logger *logger.Logger) (usecase.EventStore, error) {
	if prefix != "" && !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}

	return &RedisEventStore{
		client: client,
		prefix: prefix,
		logger: logger,
	}, nil
}

// Close 关闭Redis连接
func (s *RedisEventStore) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// SaveEvents 保存事件
func (s *RedisEventStore) SaveEvents(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	// 获取当前版本
	currentVersion, err := s.GetAggregateVersion(ctx, aggregateID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to get current version", "aggregate_id", aggregateID, "error", err)
		}
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if expectedVersion != -1 && currentVersion != expectedVersion {
		return &errs.EventStoreError{
			Type:      "concurrency_conflict",
			Message:   fmt.Sprintf("expected version %d, got %d", expectedVersion, currentVersion),
			Aggregate: aggregateID,
		}
	}

	// 使用事务保存事件
	pipe := s.client.Pipeline()
	// Redis pipeline不需要显式Close

	eventsKey := s.eventsKey(aggregateID)
	// snapshotKey变量已移除，因为未使用

	// 获取现有事件
	existingEvents, err := s.getEvents(ctx, aggregateID)
	if err != nil {
		return fmt.Errorf("failed to get existing events: %w", err)
	}

	// 追加新事件
	allEvents := append(existingEvents, events...)

	// 序列化事件
	eventsData := make([]interface{}, len(allEvents))
	for i, event := range allEvents {
		eventData, err := json.Marshal(event)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to marshal event", "aggregate_id", aggregateID, "event_type", event.GetEventType(), "error", err)
			}
			return fmt.Errorf("failed to marshal event: %w", err)
		}
		eventsData[i] = string(eventData)
	}

	// 保存事件列表
	pipe.Del(ctx, eventsKey)
	for _, eventData := range eventsData {
		pipe.RPush(ctx, eventsKey, eventData)
	}

	// 更新版本
	newVersion := currentVersion + len(events)
	pipe.Set(ctx, s.versionKey(aggregateID), newVersion, 0)

	// 设置事件过期时间（可选，默认永不过期）
	// pipe.Expire(ctx, eventsKey, 7*24*time.Hour)

	_, err = pipe.Exec(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to batch store events", "error", err)
		}
	}
	return err
}

// GetEvents 获取聚合的所有事件
func (s *RedisEventStore) GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	return s.getEvents(ctx, aggregateID)
}

// getEvents 内部获取事件方法
func (s *RedisEventStore) getEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	eventsKey := s.eventsKey(aggregateID)

	dataList, err := s.client.LRange(ctx, eventsKey, 0, -1).Result()
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to get events", "aggregate_id", aggregateID, "error", err)
		}
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	// 如果没有数据，返回空切片
	if len(dataList) == 0 {
		return []entity.DomainEvent{}, nil
	}

	return s.deserializeEvents(dataList)
}

// deserializeEvents 内部反序列化事件方法
func (s *RedisEventStore) deserializeEvents(dataList []string) ([]entity.DomainEvent, error) {
	events := make([]entity.DomainEvent, 0, len(dataList))
	for _, data := range dataList {
		// 为了保持兼容性，使用map解析事件数据
		var eventData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &eventData); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to unmarshal event", "error", err)
			}
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}

		// 使用通用函数创建事件对象
		domainEvent := DecodeEventFromMap(eventData)
		events = append(events, domainEvent)
	}

	return events, nil
}

// GetEventsFromVersion 从指定版本开始获取事件
func (s *RedisEventStore) GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error) {
	allEvents, err := s.getEvents(ctx, aggregateID)
	if err != nil {
		return nil, err
	}

	if fromVersion <= 1 {
		return allEvents, nil
	}

	if fromVersion > len(allEvents) {
		return []entity.DomainEvent{}, nil
	}

	return allEvents[fromVersion-1:], nil
}

// GetEventsByType 按事件类型获取事件
func (s *RedisEventStore) GetEventsByType(ctx context.Context, eventType string) ([]entity.DomainEvent, error) {
	// 扫描所有聚合的事件
	pattern := s.prefix + "events:*"
	keys, err := s.client.Keys(ctx, pattern).Result()
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to scan event keys", "pattern", pattern, "error", err)
		}
		return nil, fmt.Errorf("failed to scan event keys: %w", err)
	}

	var allEvents []entity.DomainEvent
	for _, key := range keys {
		dataList, err := s.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to get events from key", "key", key, "error", err)
			}
			continue // 跳过错误
		}

		for _, data := range dataList {
			// 为了保持兼容性，使用map解析事件数据
			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &eventData); err != nil {
				if s.logger != nil {
					s.logger.Error("Failed to unmarshal event data", "key", key, "error", err)
				}
				continue // 跳过错误
			}

			// 使用通用函数创建事件对象
			domainEvent := DecodeEventFromMap(eventData)

			if domainEvent.GetEventType() == eventType {
				allEvents = append(allEvents, domainEvent)
			}
		}
	}

	// 按时间排序
	// 由于Redis列表是有序的，这里不需要额外排序
	return allEvents, nil
}

// GetEventsByTimeRange 按时间范围获取事件
func (s *RedisEventStore) GetEventsByTimeRange(ctx context.Context, aggregateID string, fromTime, toTime time.Time) ([]entity.DomainEvent, error) {
	allEvents, err := s.getEvents(ctx, aggregateID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to get events", "aggregate_id", aggregateID, "error", err)
		}
		return nil, err
	}

	var filteredEvents []entity.DomainEvent
	for _, event := range allEvents {
		if event.GetCreatedAt().After(fromTime) && event.GetCreatedAt().Before(toTime) {
			filteredEvents = append(filteredEvents, event)
		}
	}

	return filteredEvents, nil
}

// GetEventsWithPagination 分页获取事件
func (s *RedisEventStore) GetEventsWithPagination(ctx context.Context, aggregateID string, limit, offset int) ([]entity.DomainEvent, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	eventsKey := s.eventsKey(aggregateID)

	start := offset
	end := offset + limit - 1

	dataList, err := s.client.LRange(ctx, eventsKey, int64(start), int64(end)).Result()
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to get events with pagination", "aggregate_id", aggregateID, "limit", limit, "offset", offset, "error", err)
		}
		return nil, fmt.Errorf("failed to get events with pagination: %w", err)
	}

	if len(dataList) == 0 {
		return []entity.DomainEvent{}, nil
	}

	events := make([]entity.DomainEvent, 0, len(dataList))
	for _, data := range dataList {
		var eventData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &eventData); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to unmarshal event", "aggregate_id", aggregateID, "error", err)
			}
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}

		domainEvent := DecodeEventFromMap(eventData)
		events = append(events, domainEvent)
	}

	return events, nil
}

// GetAggregateVersion 获取聚合版本
func (s *RedisEventStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int, error) {
	versionKey := s.versionKey(aggregateID)

	versionStr, err := s.client.Get(ctx, versionKey).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil // 没有事件，版本为0
		}
		if s.logger != nil {
			s.logger.Error("Failed to get aggregate version", "aggregate_id", aggregateID, "error", err)
		}
		return 0, fmt.Errorf("failed to get version: %w", err)
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Invalid version format", "aggregate_id", aggregateID, "version_str", versionStr, "error", err)
		}
		return 0, fmt.Errorf("invalid version format: %w", err)
	}

	return version, nil
}

// Load 从事件流加载聚合根状态
func (s *RedisEventStore) Load(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error {
	// 1. 尝试从快照恢复
	snapshot, version, err := s.GetLatestSnapshot(ctx, aggregateID)
	var events []entity.DomainEvent

	// 2. 获取快照之后的事件
	if err == nil && snapshot != nil {
		// 将快照数据应用到聚合根
		if err := aggregate.LoadFromSnapshot(snapshot); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to load from snapshot", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("failed to load from snapshot: %w", err)
		}
		// 获取快照版本之后的事件
		if version > 0 {
			var err error
			events, err = s.GetEventsFromVersion(ctx, aggregateID, version+1)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("Failed to get events from version", "aggregate_id", aggregateID, "from_version", version+1, "error", err)
				}
				return fmt.Errorf("failed to get events from version: %w", err)
			}
		}
	} else {
		// 没有快照或快照加载失败，从所有事件恢复
		var err error
		events, err = s.GetEvents(ctx, aggregateID)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to get events", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("failed to get events: %w", err)
		}
	}

	// 3. 应用事件到聚合根
	if len(events) > 0 {
		// 加载聚合 - 由于我们已经将events转换为[]entity.DomainEvent类型，这里可以直接使用
		if err := aggregate.LoadFromEvents(events); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to load aggregate from events", "aggregate_id", aggregateID, "events_count", len(events), "error", err)
			}
			return fmt.Errorf("failed to load aggregate from events: %w", err)
		}
	}

	return nil
}

// SaveSnapshot 保存快照
func (s *RedisEventStore) SaveSnapshot(ctx context.Context, aggregateID string, snapshot interface{}, version int) error {
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to marshal snapshot", "aggregate_id", aggregateID, "version", version, "error", err)
		}
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	snapshotKey := s.snapshotKey(aggregateID)

	// 使用Hash存储快照数据
	err = s.client.HSet(ctx, snapshotKey,
		"data", string(snapshotData),
		"version", version,
		"created_at", time.Now().Unix(),
	).Err()
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to store snapshot", "aggregate_id", aggregateID, "version", version, "error", err)
		}
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	return nil
}

// GetLatestSnapshot 获取最新快照
func (s *RedisEventStore) GetLatestSnapshot(ctx context.Context, aggregateID string) (interface{}, int, error) {
	snapshotKey := s.snapshotKey(aggregateID)

	data, err := s.client.HGetAll(ctx, snapshotKey).Result()
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to get snapshot", "aggregate_id", aggregateID, "error", err)
		}
		return nil, 0, fmt.Errorf("failed to get snapshot: %w", err)
	}

	if len(data) == 0 {
		return nil, 0, nil // 没有快照
	}

	snapshotData := data["data"]
	versionStr := data["version"]

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Invalid version format", "aggregate_id", aggregateID, "version_str", versionStr, "error", err)
		}
		return nil, 0, fmt.Errorf("invalid version format: %w", err)
	}

	var snapshot interface{}
	err = json.Unmarshal([]byte(snapshotData), &snapshot)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to unmarshal snapshot", "aggregate_id", aggregateID, "error", err)
		}
		return nil, 0, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return snapshot, version, nil
}

// SaveEventsBatch 批量保存事件
func (s *RedisEventStore) SaveEventsBatch(ctx context.Context, events map[string][]entity.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	pipe := s.client.Pipeline()
	// Redis pipeline不需要显式Close

	for aggregateID, eventList := range events {
		// 获取当前版本
		currentVersion, err := s.GetAggregateVersion(ctx, aggregateID)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to get current version", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("failed to get current version for aggregate %s: %w", aggregateID, err)
		}

		// 获取现有事件
		existingEvents, err := s.getEvents(ctx, aggregateID)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to get existing events", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("failed to get existing events: %w", err)
		}

		// 追加新事件
		allEvents := append(existingEvents, eventList...)

		// 序列化事件
		eventsData := make([]interface{}, len(allEvents))
		for i, event := range allEvents {
			eventData, err := json.Marshal(event)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("Failed to marshal event", "aggregate_id", aggregateID, "event_type", event.GetEventType(), "error", err)
				}
				return fmt.Errorf("failed to marshal event: %w", err)
			}
			eventsData[i] = string(eventData)
		}

		// 保存事件列表
		eventsKey := s.eventsKey(aggregateID)
		pipe.Del(ctx, eventsKey)
		for _, eventData := range eventsData {
			pipe.RPush(ctx, eventsKey, eventData)
		}

		// 更新版本
		newVersion := currentVersion + len(eventList)
		pipe.Set(ctx, s.versionKey(aggregateID), newVersion, 0)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && s.logger != nil {
		s.logger.Error("Failed to batch store events", "error", err)
	}
	return err
}

// 键生成方法
func (s *RedisEventStore) eventsKey(aggregateID string) string {
	return s.prefix + "events:" + aggregateID
}

func (s *RedisEventStore) snapshotKey(aggregateID string) string {
	return s.prefix + "snapshot:" + aggregateID
}

func (s *RedisEventStore) versionKey(aggregateID string) string {
	return s.prefix + "version:" + aggregateID
}
