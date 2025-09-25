package usecase

import (
	"context"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// EventStore 事件存储接口
type EventStore interface {
	// 基础操作
	SaveEvents(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error
	GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error)
	GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error)
	GetEventsByType(ctx context.Context, eventType string) ([]entity.DomainEvent, error)

	// 聚合根加载操作
	Load(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error

	// 增强功能
	GetEventsByTimeRange(ctx context.Context, aggregateID string, fromTime, toTime time.Time) ([]entity.DomainEvent, error)
	GetEventsWithPagination(ctx context.Context, aggregateID string, limit, offset int) ([]entity.DomainEvent, error)
	GetAggregateVersion(ctx context.Context, aggregateID string) (int, error)

	// 快照支持
	SaveSnapshot(ctx context.Context, aggregateID string, snapshot interface{}, version int) error
	GetLatestSnapshot(ctx context.Context, aggregateID string) (interface{}, int, error)

	// 批量操作
	SaveEventsBatch(ctx context.Context, events map[string][]entity.DomainEvent) error
	Close() error
}
