package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"

	"github.com/jmoiron/sqlx"
)

// PostgreSQLEventStore PostgreSQL事件存储实现
type PostgreSQLEventStore struct {
	db     *sqlx.DB
	logger *logger.Logger
}

// NewPostgreSQLEventStore 创建PostgreSQL事件存储实例
func NewPostgreSQLEventStore(db *sqlx.DB, logger *logger.Logger) (*PostgreSQLEventStore, error) {
	if err := createEventStoreTables(db); err != nil {
		return nil, fmt.Errorf("failed to create event store tables: %w", err)
	}

	return &PostgreSQLEventStore{db: db, logger: logger}, nil
}

// createEventStoreTables 创建事件存储所需的表
func createEventStoreTables(db *sqlx.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS events (
			id BIGSERIAL PRIMARY KEY,
			aggregate_id VARCHAR(255) NOT NULL,
			aggregate_type VARCHAR(255) NOT NULL,
			event_type VARCHAR(255) NOT NULL,
			event_data JSONB NOT NULL,
			version INTEGER NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			metadata JSONB,
			CONSTRAINT events_aggregate_version UNIQUE (aggregate_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_aggregate_id ON events(aggregate_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id BIGSERIAL PRIMARY KEY,
			aggregate_id VARCHAR(255) NOT NULL UNIQUE,
			aggregate_type VARCHAR(255) NOT NULL,
			snapshot_data JSONB NOT NULL,
			version INTEGER NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_aggregate_id ON snapshots(aggregate_id)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// SaveEvents 保存事件
func (s *PostgreSQLEventStore) SaveEvents(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			if s.logger != nil {
				s.logger.Error("failed to rollback transaction", "error", err)
			}
		}
	}()

	// 检查当前版本
	var currentVersion int
	query := `SELECT COALESCE(MAX(version), 0) FROM events WHERE aggregate_id = $1`
	err = tx.GetContext(ctx, &currentVersion, query, aggregateID)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if expectedVersion != -1 && currentVersion != expectedVersion {
		return &errs.EventStoreError{
			Type:      "concurrency_conflict",
			Message:   fmt.Sprintf("expected version %d, got %d", expectedVersion, currentVersion),
			Aggregate: aggregateID,
		}
	}

	// 插入事件
	insertQuery := `
		INSERT INTO events (aggregate_id, aggregate_type, event_type, event_data, version, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	for i, event := range events {
		eventData, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		version := currentVersion + i + 1
		_, err = tx.ExecContext(ctx, insertQuery,
			aggregateID,
			event.GetAggregateType(),
			event.GetEventType(),
			eventData,
			version,
			json.RawMessage(`{}`), // 空metadata
		)
		if err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetEvents 获取聚合的所有事件
func (s *PostgreSQLEventStore) GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	query := `
		SELECT event_data FROM events 
		WHERE aggregate_id = $1 
		ORDER BY version ASC
	`

	var eventsData []json.RawMessage
	err := s.db.SelectContext(ctx, &eventsData, query, aggregateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	return s.deserializeEvents(eventsData)
}

// GetEventsFromVersion 从指定版本开始获取事件
func (s *PostgreSQLEventStore) GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error) {
	query := `
		SELECT event_data FROM events 
		WHERE aggregate_id = $1 AND version >= $2
		ORDER BY version ASC
	`

	var eventsData []json.RawMessage
	err := s.db.SelectContext(ctx, &eventsData, query, aggregateID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get events from version: %w", err)
	}

	return s.deserializeEvents(eventsData)
}

// GetEventsByType 按事件类型获取事件
func (s *PostgreSQLEventStore) GetEventsByType(ctx context.Context, eventType string) ([]entity.DomainEvent, error) {
	query := `
		SELECT event_data FROM events 
		WHERE event_type = $1
		ORDER BY created_at ASC
	`

	var eventsData []json.RawMessage
	err := s.db.SelectContext(ctx, &eventsData, query, eventType)
	if err != nil {
		return nil, fmt.Errorf("failed to get events by type: %w", err)
	}

	return s.deserializeEvents(eventsData)
}

// GetEventsByTimeRange 按时间范围获取事件
func (s *PostgreSQLEventStore) GetEventsByTimeRange(ctx context.Context, aggregateID string, fromTime, toTime time.Time) ([]entity.DomainEvent, error) {
	query := `
		SELECT event_data FROM events 
		WHERE aggregate_id = $1 AND created_at BETWEEN $2 AND $3
		ORDER BY created_at ASC
	`

	var eventsData []json.RawMessage
	err := s.db.SelectContext(ctx, &eventsData, query, aggregateID, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get events by time range: %w", err)
	}

	return s.deserializeEvents(eventsData)
}

// GetEventsWithPagination 分页获取事件
func (s *PostgreSQLEventStore) GetEventsWithPagination(ctx context.Context, aggregateID string, limit, offset int) ([]entity.DomainEvent, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT event_data FROM events 
		WHERE aggregate_id = $1
		ORDER BY version ASC
		LIMIT $2 OFFSET $3
	`

	var eventsData []json.RawMessage
	err := s.db.SelectContext(ctx, &eventsData, query, aggregateID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get events with pagination: %w", err)
	}

	return s.deserializeEvents(eventsData)
}

// GetAggregateVersion 获取聚合版本
func (s *PostgreSQLEventStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int, error) {
	query := `SELECT COALESCE(MAX(version), 0) FROM events WHERE aggregate_id = $1`

	var version int
	err := s.db.GetContext(ctx, &version, query, aggregateID)
	if err != nil {
		return 0, fmt.Errorf("failed to get aggregate version: %w", err)
	}

	return version, nil
}

// Load 从事件流加载聚合根状态
func (s *PostgreSQLEventStore) Load(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error {
	// 1. 尝试从快照恢复
	snapshot, version, err := s.GetLatestSnapshot(ctx, aggregateID)
	var events []entity.DomainEvent

	// 2. 获取快照之后的事件
	if err == nil && snapshot != nil {
		// 将快照数据应用到聚合根
		if err := aggregate.LoadFromSnapshot(snapshot); err != nil {
			return fmt.Errorf("failed to load from snapshot: %w", err)
		}
		// 获取快照版本之后的事件
		if version > 0 {
			var err error
			events, err = s.GetEventsFromVersion(ctx, aggregateID, version+1)
			if err != nil {
				return fmt.Errorf("failed to get events from version: %w", err)
			}
		}
	} else {
		// 没有快照或快照加载失败，从所有事件恢复
		var err error
		events, err = s.GetEvents(ctx, aggregateID)
		if err != nil {
			return fmt.Errorf("failed to get events: %w", err)
		}
	}

	// 3. 应用事件到聚合根
	if len(events) > 0 {
		if err := aggregate.LoadFromEvents(events); err != nil {
			return fmt.Errorf("failed to load from events: %w", err)
		}
	}

	return nil
}

// SaveSnapshot 保存快照
func (s *PostgreSQLEventStore) SaveSnapshot(ctx context.Context, aggregateID string, snapshot interface{}, version int) error {
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	query := `
		INSERT INTO snapshots (aggregate_id, aggregate_type, snapshot_data, version)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (aggregate_id) DO UPDATE SET
			snapshot_data = EXCLUDED.snapshot_data,
			version = EXCLUDED.version,
			updated_at = NOW()
	`

	_, err = s.db.ExecContext(ctx, query,
		aggregateID,
		"aggregate", // 需要从快照中获取类型
		snapshotData,
		version,
	)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	return nil
}

// GetLatestSnapshot 获取最新快照
func (s *PostgreSQLEventStore) GetLatestSnapshot(ctx context.Context, aggregateID string) (interface{}, int, error) {
	query := `
		SELECT snapshot_data, version FROM snapshots 
		WHERE aggregate_id = $1
	`

	var snapshotData json.RawMessage
	var version int
	err := s.db.QueryRowContext(ctx, query, aggregateID).Scan(&snapshotData, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, nil // 没有快照
		}
		return nil, 0, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	var snapshot interface{}
	err = json.Unmarshal(snapshotData, &snapshot)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return snapshot, version, nil
}

// SaveEventsBatch 批量保存事件
func (s *PostgreSQLEventStore) SaveEventsBatch(ctx context.Context, events map[string][]entity.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			if s.logger != nil {
				s.logger.Error("failed to rollback transaction", "error", err)
			}
		}
	}()

	insertQuery := `
		INSERT INTO events (aggregate_id, aggregate_type, event_type, event_data, version, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	for aggregateID, eventList := range events {
		// 获取当前版本
		var currentVersion int
		versionQuery := `SELECT COALESCE(MAX(version), 0) FROM events WHERE aggregate_id = $1`
		err = tx.GetContext(ctx, &currentVersion, versionQuery, aggregateID)
		if err != nil {
			return fmt.Errorf("failed to get current version for aggregate %s: %w", aggregateID, err)
		}

		// 插入事件
		for i, event := range eventList {
			eventData, err := json.Marshal(event)
			if err != nil {
				return fmt.Errorf("failed to marshal event: %w", err)
			}

			version := currentVersion + i + 1
			_, err = tx.ExecContext(ctx, insertQuery,
				aggregateID,
				event.GetAggregateType(),
				event.GetEventType(),
				eventData,
				version,
				json.RawMessage(`{}`),
			)
			if err != nil {
				return fmt.Errorf("failed to insert event: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// deserializeEvents 反序列化事件数据
func (s *PostgreSQLEventStore) deserializeEvents(eventsData []json.RawMessage) ([]entity.DomainEvent, error) {
	// 创建结果切片
	events := make([]entity.DomainEvent, 0, len(eventsData))

	for _, data := range eventsData {
		// 为了保持兼容性，使用map解析事件数据
		var eventData map[string]interface{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
		}

		// 使用通用函数创建事件对象
	domainEvent := DecodeEventFromMap(eventData)
	events = append(events, domainEvent)
	}

	return events, nil
}
