package usecase

import (
	"context"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

/*
2025/10/18
事件存储的实现有两种模式，这关乎于具体实现的技术架构：
1. 基于数据库型的事件存储
2. 基于事件流（如JetStream或Kafka）的事件存储

##  基于数据库型事件存储
例如采用 Badger, Redis 等键值存储数据库，以及的关系型数据库（如MySQL, PostgreSQL）等。 此时就需要实现完整的事件存储接口。因为有以下有众多接口都是为了防止并发冲突，以及为了支持事件的批量操作。
这种架构下 EventStore 的读与写都是必须具备的。

##  基于事件流的事件存储
例如采用 JetStream 或 Kafka 等事件流平台。 此时就只需要实现 相关的“读” 方法。因为事件流存储将会被作为“唯一可靠的数据来源”。 而“写”操作则可以通过事件流平台的发布功能来实现。一切发布至事件流的事件，
都会被自动存储，从而就无需要担心写入事件流时出现的分布式并发问题。

因此，原有的EventStore接口属于针对全部场景而设计的接口，而当采用事件流存储时，推荐实现 EventReader 接口。
*/

// 基础事件存储能力（必须实现）
type BaseEventStore interface {
	SaveEvents(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error
	GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error)
	Load(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error
	Close() error
}

// 扩展查询能力（按需实现）
type EventQueryable interface {
	GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error)
	GetEventsByType(ctx context.Context, eventType string) ([]entity.DomainEvent, error)
	GetEventsByTimeRange(ctx context.Context, aggregateID string, fromTime, toTime time.Time) ([]entity.DomainEvent, error)
	GetEventsWithPagination(ctx context.Context, aggregateID string, limit, offset int) ([]entity.DomainEvent, error)
	GetAggregateVersion(ctx context.Context, aggregateID string) (int, error)
}

// 快照能力（按需实现）
type SnapshotSupport interface {
	SaveSnapshot(ctx context.Context, aggregateID string, snapshot interface{}, version int) error
	GetLatestSnapshot(ctx context.Context, aggregateID string) (interface{}, int, error)
}

// 批量操作能力（按需实现）
type BatchSupport interface {
	SaveEventsBatch(ctx context.Context, events map[string][]entity.DomainEvent) error
}

// 完整的EventStore接口（组合基础+扩展能力）
type EventStore interface {
	BaseEventStore
	EventQueryable
	SnapshotSupport
	BatchSupport
}
