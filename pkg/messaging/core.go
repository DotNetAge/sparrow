package messaging

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// 泛型约束：限制为实现了entity.DomainEvent的类型
type DomainEventConstraint interface {
	entity.DomainEvent
}

// StreamPublisher 事件流发布器接口
// 定义事件流系统的发布能力，所有事件流发布器（如JetStream/Kafka）需实现此接口
type StreamPublisher interface {
	// Publish 发布领域事件到事件流（发布即存储）
	// event：待发布的领域事件（需实现entity.DomainEvent）
	Publish(ctx context.Context, event entity.DomainEvent) error

	// 批量发布事件
	PublishEvents(ctx context.Context, events []entity.DomainEvent) error
}

// StreamSubscriber 事件流订阅器接口
// 定义事件流系统的订阅能力，支持订阅多种事件类型并处理
// StreamSubscriber 事件流订阅器的抽象接口
type StreamSubscriber interface {
	// // 订阅指定事件类型
	// Subscribe(ctx context.Context, eventType string, handler interface{}) error
	// 启动订阅（开始接收事件，长时运行）
	Start(ctx context.Context) error
	// 停止订阅（优雅关闭）
	Stop(ctx context.Context) error
}

// DomainEventHandler 领域事件处理器函数类型
// 针对特定事件类型T的处理逻辑
type DomainEventHandler[T DomainEventConstraint] func(ctx context.Context, event T) error

// 事件流专用的事件读取器接口
type StreamReader interface {
	// 获取聚合根的所有事件（事件流的核心读操作）
	GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error)
	// 将事件重放至聚合根（封装“获取事件→应用到聚合根”的逻辑）
	// Replay 方法是可以允许传入的aggregateID不存在，而不引发错误。相当于Unexists的情况，多用于添加新聚合根时的初始化
	Replay(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error
	// 事件流场景常用的“按版本/时间范围重放”（可选，但实用）
	ReplayFromVersion(ctx context.Context, aggregateID string, fromVersion int, aggregate entity.AggregateRoot) error
	// 按事件流的物理偏移量重放（比按版本更高效）
	ReplayFromOffset(ctx context.Context, aggregateID string, offset uint64, aggregate entity.AggregateRoot) error
}
