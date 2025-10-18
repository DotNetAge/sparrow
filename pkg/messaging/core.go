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
