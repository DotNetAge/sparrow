package messaging

import (
	"context"
	"fmt"
	"purchase-service/pkg/entity"
	"purchase-service/pkg/eventbus"
	"purchase-service/pkg/usecase"
)

// EventPublisher 事件发布器 - 连接事件存储与投影
// 负责从事件存储中读取事件并发布到事件总线，供投影更新器消费
// 增强功能：支持领域事件到通用事件的转换，实现主题命名约束

type EventPublisher struct {
	eventStore  usecase.EventStore
	eventBus    eventbus.EventBus
	serviceName string // 服务名称，用于构建完整的事件主题
}

// NewEventPublisher 创建事件发布器
func NewEventPublisher(eventStore usecase.EventStore, eventBus eventbus.EventBus, serviceName string) *EventPublisher {
	return &EventPublisher{
		eventStore:  eventStore,
		eventBus:    eventBus,
		serviceName: serviceName,
	}
}

// PublishEvents 发布聚合的事件到事件总线
func (p *EventPublisher) PublishEvents(ctx context.Context, aggregateID string) error {
	events, err := p.eventStore.GetEvents(ctx, aggregateID)
	if err != nil {
		return fmt.Errorf("failed to get events from store: %w", err)
	}

	// 发布所有事件到事件总线
	for _, event := range events {
		if err := p.publishEvent(ctx, event); err != nil {
			return fmt.Errorf("failed to publish event %s: %w", event.GetEventType(), err)
		}
	}

	return nil
}

// PublishUncommittedEvents 发布未提交的事件（在保存后立即发布）
func (p *EventPublisher) PublishUncommittedEvents(ctx context.Context, aggregate entity.AggregateRoot) error {
	// 获取未提交的事件
	events := aggregate.GetUncommittedEvents()

	if len(events) == 0 {
		return nil
	}

	// 发布事件到事件总线
	for _, event := range events {
		if err := p.publishEvent(ctx, event); err != nil {
			return fmt.Errorf("failed to publish uncommitted event %s: %w", event.GetEventType(), err)
		}
	}

	return nil
}

// publishEvent 内部方法：发布单个事件并处理主题命名约束
// 实现 Service.Aggregate.EventType 格式的主题命名
// 将领域事件转换为包含完整主题名称的通用事件
func (p *EventPublisher) publishEvent(ctx context.Context, domainEvent entity.DomainEvent) error {
	// 构建完整的事件主题：服务名.聚合名.事件名
	serviceName := p.serviceName
	if serviceName == "" {
		serviceName = "default"
	}

	// 按照"服务名.聚合名.事件名"的格式生成完整的事件类型名称
	fullEventType := fmt.Sprintf("%s.%s.%s", serviceName, domainEvent.GetAggregateType(), domainEvent.GetEventType())

	// 创建一个新的通用事件，用于在事件总线上传输
	// 这里通过组合领域事件数据，确保事件总线上的事件包含完整的主题信息
	genericEvent := &entity.GenericEvent{
		Id:        domainEvent.GetEventID(),
		EventType: fullEventType,
		Timestamp: domainEvent.GetCreatedAt(),
		Payload: map[string]interface{}{
			"aggregateID":   domainEvent.GetAggregateID(),
			"aggregateType": domainEvent.GetAggregateType(),
			"version":       domainEvent.GetVersion(),
			"eventData":     domainEvent,
		},
	}

	// 将包含完整主题信息的通用事件发布到事件总线
	return p.eventBus.Pub(ctx, genericEvent)
}
