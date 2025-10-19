package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/usecase"

	"github.com/dgraph-io/badger/v4"
)

// BadgerEventStore BadgerDB事件存储实现
type BadgerEventStore struct {
	db     *badger.DB
	logger *logger.Logger
}

var _ usecase.EventStore = (*BadgerEventStore)(nil)

// NewBadgerEventStore 创建BadgerDB事件存储实例
// TODO: 错误地更了事件与的版本，这个行为在事件存储中应该是不允许的，但由于非流式处理必然会面对并发冲突，因此，可以使用
// v1.1.18 之后对 DomainEvent 新增的 SetVersion 对事件的版本进行更改；
func NewBadgerEventStore(dbPath string, log *logger.Logger) (usecase.EventStore, error) {
	opts := badger.DefaultOptions(dbPath)
	opts.SyncWrites = true
	opts.Logger = nil // 禁用日志输出

	db, err := badger.Open(opts)
	if err != nil {
		if log != nil {
			log.Error("Failed to open badger database", "error", err)
		}
		return nil, fmt.Errorf("failed to open badger database: %w", err)
	}

	return &BadgerEventStore{db: db, logger: log}, nil
}

// SaveEventsBatch 批量保存事件
func (s *BadgerEventStore) SaveEventsBatch(ctx context.Context, events map[string][]entity.DomainEvent) error {
	return s.db.Update(func(txn *badger.Txn) error {
		for aggregateID, evtList := range events {
			if err := s.SaveEvents(ctx, aggregateID, evtList, -1); err != nil {
				return err
			}
		}
		return nil
	})
}

// Close 关闭数据库连接
func (s *BadgerEventStore) Close() error {
	return s.db.Close()
}

// SaveEvents 保存事件
func (s *BadgerEventStore) SaveEvents(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// 检查当前版本
		currentVersion, err := s.getCurrentVersion(txn, aggregateID)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to get current version", "aggregate_id", aggregateID, "error", err)
			}
			return errs.NewEventStoreError("version_check", "failed to get current version", aggregateID, err)
		}

		if expectedVersion != -1 && currentVersion != expectedVersion {
			return errs.NewConcurrencyConflictError(aggregateID, expectedVersion, currentVersion)
		}

		// 保存事件
		for i, evt := range events {
			version := currentVersion + i + 1

			eventData, err := json.Marshal(evt)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("序列化事件失败", "aggregate_id", aggregateID, "event_type", evt.GetEventType(), "error", err)
				}
				return errs.NewEventStoreError("event_marshal", "序列化事件失败", aggregateID, err)
			}

			eventKey := s.getEventKey(aggregateID, version)

			eventMeta := EventMeta{
				EventID:       evt.GetEventID(),
				AggregateID:   aggregateID,
				AggregateType: evt.GetAggregateType(),
				EventType:     evt.GetEventType(),
				Version:       version,
				CreatedAt:     time.Now(),
				EventData:     eventData,
			}

			metaData, err := json.Marshal(eventMeta)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("序列化事件元数据失败", "aggregate_id", aggregateID, "error", err)
				}
				return errs.NewEventStoreError("metadata_marshal", "序列化事件元数据失败", aggregateID, err)
			}

			if err := txn.Set(eventKey, metaData); err != nil {
				if s.logger != nil {
					s.logger.Error("保存事件失败", "aggregate_id", aggregateID, "version", version, "error", err)
				}
				return errs.NewEventStoreError("event_save", "保存事件失败", aggregateID, err)
			}
		}

		// 更新版本号
		versionKey := s.getVersionKey(aggregateID)
		newVersion := currentVersion + len(events)
		if err := txn.Set(versionKey, []byte(strconv.Itoa(newVersion))); err != nil {
			if s.logger != nil {
				s.logger.Error("更新版本号失败", "aggregate_id", aggregateID, "new_version", newVersion, "error", err)
			}
			return errs.NewEventStoreError("version_update", "更新版本号失败", aggregateID, err)
		}

		return nil
	})
}

// GetEvents 获取聚合的所有事件
func (s *BadgerEventStore) GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	return s.GetEventsFromVersion(ctx, aggregateID, 1)
}

// GetEventsFromVersion 从指定版本开始获取事件
func (s *BadgerEventStore) GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error) {
	var events []entity.DomainEvent

	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(fmt.Sprintf("event:%s:", aggregateID))
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			// 解析版本号
			key := string(item.Key())
			version, err := s.parseVersionFromKey(key)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("解析事件版本号失败", "key", key, "error", err)
				}
				continue
			}

			if version < fromVersion {
				continue
			}

			var metaData []byte
			err = item.Value(func(val []byte) error {
				metaData = append([]byte{}, val...)
				return nil
			})
			if err != nil {
				if s.logger != nil {
					s.logger.Error("获取事件值失败", "aggregate_id", aggregateID, "error", err)
				}
				return fmt.Errorf("获取事件值失败: %w", err)
			}

			event, err := s.deserializeEvent(metaData)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("反序列化事件失败", "aggregate_id", aggregateID, "error", err)
				}
				return fmt.Errorf("反序列化事件失败: %w", err)
			}

			events = append(events, event)
		}

		return nil
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("获取事件失败", "aggregate_id", aggregateID, "error", err)
		}
		return nil, fmt.Errorf("获取事件失败: %w", err)
	}

	return events, nil
}

// GetEventsByType 按事件类型获取事件
func (s *BadgerEventStore) GetEventsByType(ctx context.Context, eventType string) ([]entity.DomainEvent, error) {
	var events []entity.DomainEvent

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("event:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var metaData []byte
			err := item.Value(func(val []byte) error {
				metaData = append([]byte{}, val...)
				return nil
			})
			if err != nil {
				if s.logger != nil {
					s.logger.Error("获取事件值失败", "error", err)
				}
				return fmt.Errorf("获取事件值失败: %w", err)
			}

			// 先解析EventMeta以获取事件类型信息
			var eventMeta EventMeta
			if err := json.Unmarshal(metaData, &eventMeta); err != nil {
				if s.logger != nil {
					s.logger.Error("反序列化事件元数据失败", "error", err)
				}
				continue // 跳过这个事件
			}

			if eventMeta.EventType == eventType {
				event, err := s.deserializeEvent(metaData)
				if err != nil {
					if s.logger != nil {
						s.logger.Error("反序列化事件失败", "error", err)
					}
					return fmt.Errorf("反序列化事件失败: %w", err)
				}
				events = append(events, event)
			}
		}

		return nil
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("获取事件失败", "event_type", eventType, "error", err)
		}
		return nil, fmt.Errorf("获取事件失败: %w", err)
	}

	return events, nil
}

// GetEventsByTimeRange 按时间范围获取事件
func (s *BadgerEventStore) GetEventsByTimeRange(ctx context.Context, aggregateID string, fromTime, toTime time.Time) ([]entity.DomainEvent, error) {
	var events []entity.DomainEvent

	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(fmt.Sprintf("event:%s:", aggregateID))
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			var metaData []byte
			err := item.Value(func(val []byte) error {
				metaData = append([]byte{}, val...)
				return nil
			})
			if err != nil {
				if s.logger != nil {
					s.logger.Error("获取事件值失败", "aggregate_id", aggregateID, "error", err)
				}
				return fmt.Errorf("获取事件值失败: %w", err)
			}

			// 先解析EventMeta以获取时间信息
			var eventMeta EventMeta
			if err := json.Unmarshal(metaData, &eventMeta); err != nil {
				if s.logger != nil {
					s.logger.Error("反序列化事件元数据失败", "aggregate_id", aggregateID, "error", err)
				}
				continue // 跳过这个事件
			}

			if eventMeta.CreatedAt.After(fromTime) && eventMeta.CreatedAt.Before(toTime) {
				event, err := s.deserializeEvent(metaData)
				if err != nil {
					if s.logger != nil {
						s.logger.Error("反序列化事件失败", "aggregate_id", aggregateID, "error", err)
					}
					return fmt.Errorf("反序列化事件失败: %w", err)
				}
				events = append(events, event)
			}
		}

		return nil
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("获取事件失败", "aggregate_id", aggregateID, "error", err)
		}
		return nil, fmt.Errorf("获取事件失败: %w", err)
	}

	return events, nil
}

// GetEventsWithPagination 分页获取事件
func (s *BadgerEventStore) GetEventsWithPagination(ctx context.Context, aggregateID string, limit, offset int) ([]entity.DomainEvent, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	var events []entity.DomainEvent
	var count int

	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(fmt.Sprintf("event:%s:", aggregateID))
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if count < offset {
				count++
				continue
			}

			if len(events) >= limit {
				break
			}

			item := it.Item()
			var metaData []byte
			err := item.Value(func(val []byte) error {
				metaData = append([]byte{}, val...)
				return nil
			})
			if err != nil {
				if s.logger != nil {
					s.logger.Error("获取事件值失败", "aggregate_id", aggregateID, "error", err)
				}
				return fmt.Errorf("获取事件值失败: %w", err)
			}

			event, err := s.deserializeEvent(metaData)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("反序列化事件失败", "aggregate_id", aggregateID, "error", err)
				}
				return fmt.Errorf("反序列化事件失败: %w", err)
			}
			events = append(events, event)
			count++
		}

		return nil
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("获取事件分页失败", "aggregate_id", aggregateID, "limit", limit, "offset", offset, "error", err)
		}
		return nil, fmt.Errorf("获取事件分页失败: %w", err)
	}

	return events, nil
}

// SaveSnapshot 保存快照
func (s *BadgerEventStore) SaveSnapshot(ctx context.Context, aggregateID string, snapshot interface{}, version int) error {
	return s.db.Update(func(txn *badger.Txn) error {
		snapshotData, err := json.Marshal(snapshot)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("序列化快照失败", "aggregate_id", aggregateID, "version", version, "error", err)
			}
			return fmt.Errorf("序列化快照失败: %w", err)
		}

		snapshotMeta := SnapshotMeta{
			AggregateID:   aggregateID,
			SnapshotData:  snapshotData,
			Version:       version,
			CreatedAt:     time.Now(),
			AggregateType: "unknown", // 可以从快照数据推断
		}

		metaData, err := json.Marshal(snapshotMeta)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("序列化快照元数据失败", "aggregate_id", aggregateID, "version", version, "error", err)
			}
			return fmt.Errorf("序列化快照元数据失败: %w", err)
		}

		snapshotKey := s.getSnapshotKey(aggregateID)
		if err := txn.Set(snapshotKey, metaData); err != nil {
			if s.logger != nil {
				s.logger.Error("保存快照失败", "aggregate_id", aggregateID, "version", version, "error", err)
			}
			return fmt.Errorf("保存快照失败: %w", err)
		}

		return nil
	})
}

// GetLatestSnapshot 获取最新快照
func (s *BadgerEventStore) GetLatestSnapshot(ctx context.Context, aggregateID string) (interface{}, int, error) {
	var snapshotMeta SnapshotMeta
	var snapshot interface{}

	err := s.db.View(func(txn *badger.Txn) error {
		snapshotKey := s.getSnapshotKey(aggregateID)
		item, err := txn.Get(snapshotKey)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // 快照不存在
			}
			if s.logger != nil {
				s.logger.Error("获取快照失败", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("获取快照失败: %w", err)
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &snapshotMeta)
		})
		if err != nil {
			if s.logger != nil {
				s.logger.Error("反序列化快照元数据失败", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("反序列化快照元数据失败: %w", err)
		}

		err = json.Unmarshal(snapshotMeta.SnapshotData, &snapshot)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("反序列化快照失败", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("反序列化快照失败: %w", err)
		}

		return nil
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("获取最新快照失败", "aggregate_id", aggregateID, "error", err)
		}
		return nil, 0, fmt.Errorf("获取最新快照失败: %w", err)
	}

	if snapshot == nil {
		return nil, 0, nil // 快照不存在
	}

	return snapshot, snapshotMeta.Version, nil
}

// GetAggregateVersion 获取聚合版本号
func (s *BadgerEventStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int, error) {
	var version int

	err := s.db.View(func(txn *badger.Txn) error {
		var err error
		version, err = s.getCurrentVersion(txn, aggregateID)
		return err
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("获取聚合版本失败", "aggregate_id", aggregateID, "error", err)
		}
		return 0, fmt.Errorf("获取聚合版本失败: %w", err)
	}

	return version, nil
}

// Load 从事件流加载聚合根状态
func (s *BadgerEventStore) Load(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error {
	// 1. 尝试从快照恢复
	snapshot, version, err := s.GetLatestSnapshot(ctx, aggregateID)
	var events []entity.DomainEvent

	// 2. 获取快照之后的事件
	if err == nil && snapshot != nil {
		// 将快照数据应用到聚合根
		if err := aggregate.LoadFromSnapshot(snapshot); err != nil {
			if s.logger != nil {
				s.logger.Error("从快照加载聚合根失败", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("从快照加载聚合根失败: %w", err)
		}
		// 获取快照版本之后的事件
		if version > 0 {
			var err error
			events, err = s.GetEventsFromVersion(ctx, aggregateID, version+1)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("获取事件分页失败", "aggregate_id", aggregateID, "from_version", version+1, "error", err)
				}
				return fmt.Errorf("获取事件分页失败: %w", err)
			}
		}
	} else {
		// 没有快照或快照加载失败，从所有事件恢复
		var err error
		events, err = s.GetEvents(ctx, aggregateID)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("获取事件分页失败", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("获取事件分页失败: %w", err)
		}
	}

	// 3. 应用事件到聚合根
	if len(events) > 0 {
		if err := aggregate.LoadFromEvents(events); err != nil {
			if s.logger != nil {
				s.logger.Error("从事件加载聚合根失败", "aggregate_id", aggregateID, "events_count", len(events), "error", err)
			}
			return fmt.Errorf("从事件加载聚合根失败: %w", err)
		}
	}

	return nil
}

// 辅助方法

// getCurrentVersion 获取当前版本号
func (s *BadgerEventStore) getCurrentVersion(txn *badger.Txn, aggregateID string) (int, error) {
	versionKey := s.getVersionKey(aggregateID)
	item, err := txn.Get(versionKey)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return 0, nil // 新聚合，版本为0
		}
		if s.logger != nil {
			s.logger.Error("获取聚合版本失败", "aggregate_id", aggregateID, "error", err)
		}
		return 0, fmt.Errorf("获取聚合版本失败: %w", err)
	}

	var version int
	err = item.Value(func(val []byte) error {
		v, err := strconv.Atoi(string(val))
		if err != nil {
			if s.logger != nil {
				s.logger.Error("无效的版本格式", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("无效的版本格式: %w", err)
		}
		version = v
		return nil
	})

	return version, err
}

// 键生成方法
func (s *BadgerEventStore) getEventKey(aggregateID string, version int) []byte {
	return []byte(fmt.Sprintf("event:%s:%d", aggregateID, version))
}

func (s *BadgerEventStore) getVersionKey(aggregateID string) []byte {
	return []byte(fmt.Sprintf("version:%s", aggregateID))
}

func (s *BadgerEventStore) getSnapshotKey(aggregateID string) []byte {
	return []byte(fmt.Sprintf("snapshot:%s", aggregateID))
}

func (s *BadgerEventStore) parseVersionFromKey(key string) (int, error) {
	// 格式: event:aggregateID:version
	parts := []byte(key)
	lastColon := -1
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon == -1 {
		return 0, fmt.Errorf("无效的键格式")
	}
	return strconv.Atoi(string(parts[lastColon+1:]))
}

// deserializeEvent 反序列化事件
func (s *BadgerEventStore) deserializeEvent(data []byte) (entity.DomainEvent, error) {
	// 使用通用的DecodeEvent函数替换自己实现的反序列化逻辑
	event, err := DecodeEvent(data)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("反序列化事件失败", "error", err)
		}
		return nil, fmt.Errorf("反序列化事件失败: %w", err)
	}
	return event, nil
}

// deserializeEvents 方法已移除，因为未被使用

// SnapshotMeta 快照元数据结构
type SnapshotMeta struct {
	AggregateID   string          `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	Version       int             `json:"version"`
	CreatedAt     time.Time       `json:"created_at"`
	SnapshotData  json.RawMessage `json:"snapshot_data"`
}

// GenericEvent 通用事件类型
type GenericEvent struct {
	Data map[string]interface{}
}

func (e *GenericEvent) GetEventType() string {
	if eventType, ok := e.Data["event_type"]; ok {
		return fmt.Sprintf("%v", eventType)
	}
	return "unknown"
}

func (e *GenericEvent) GetAggregateType() string {
	if aggType, ok := e.Data["aggregate_type"]; ok {
		return fmt.Sprintf("%v", aggType)
	}
	return "unknown"
}

func (e *GenericEvent) GetAggregateID() string {
	if aggID, ok := e.Data["aggregate_id"]; ok {
		return fmt.Sprintf("%v", aggID)
	}
	return "unknown"
}

func (e *GenericEvent) GetEventData() interface{} {
	return e.Data
}

func (e *GenericEvent) GetEventTime() time.Time {
	if eventTime, ok := e.Data["event_time"]; ok {
		if t, ok := eventTime.(string); ok {
			if parsed, err := time.Parse(time.RFC3339, t); err == nil {
				return parsed
			}
		}
	}
	return time.Now()
}
