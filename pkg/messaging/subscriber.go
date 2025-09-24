package messaging

import (
	"context"
	"fmt"
	"purchase-service/pkg/adapter/event/handlers"
	"purchase-service/pkg/entity"
	"purchase-service/pkg/eventbus"
	"strings"
	"sync"
)

// EventSubscriber 事件订阅器 - 封装事件总线以简化领域事件的订阅
// 负责订阅事件总线上的领域事件，并将其分发给对应的处理器
// 增强功能：支持领域事件过滤，自动处理订阅管理
type EventSubscriber struct {
	eventBus    eventbus.EventBus
	subscribers map[string][]*subscriberInfo
	mu          sync.RWMutex
}

// NewEventSubscriber 创建事件订阅器
func NewEventSubscriber(eventBus eventbus.EventBus) *EventSubscriber {
	return &EventSubscriber{
		eventBus:    eventBus,
		subscribers: make(map[string][]*subscriberInfo),
	}
}

// Subscribe 订阅指定类型的领域事件
// eventType: 事件类型
// handler: 领域事件处理器
func (s *EventSubscriber) Subscribe(eventType string, handler handlers.DomainEventHandler) error {
	return s.SubscribeWithFilter(eventType, "", handler)
}

// SubscribeWithFilter 订阅指定类型的领域事件，支持按服务名称过滤
// eventType: 事件类型
// serviceName: 服务名称过滤器，为空时不过滤
// handler: 领域事件处理器
func (s *EventSubscriber) SubscribeWithFilter(eventType string, serviceName string, handler handlers.DomainEventHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建包装处理器，用于从通用事件中提取领域事件数据并实现过滤
	wrappedHandler := func(ctx context.Context, evt entity.Event) error {
		// 从EventBus接收到的是通用事件，需要从中提取领域事件数据
		genericEvent, ok := evt.(*entity.GenericEvent)
		if !ok {
			return fmt.Errorf("event %s is not a GenericEvent", evt.GetEventType())
		}

		// 检查是否指定了服务名称过滤器
		if serviceName != "" {
			// 从完整事件类型名称中解析服务名
			// 格式为：服务名.聚合名.事件名
			fullEventType := evt.GetEventType()
			eventTypeParts := strings.Split(fullEventType, ".")
			if len(eventTypeParts) >= 1 {
				// 比较解析出的服务名与过滤器
				if eventTypeParts[0] != serviceName {
					// 服务名称不匹配，跳过此事件
					return nil
				}
			}
		}

		// 从Payload中提取领域事件数据
		payload := genericEvent.Payload.(map[string]interface{})
		eventData, ok := payload["eventData"]
		if !ok {
			return fmt.Errorf("missing domain event data in payload")
		}

		// 将事件数据转换为DomainEvent
		domainEvent, ok := eventData.(entity.DomainEvent)
		if !ok {
			return fmt.Errorf("invalid domain event data format")
		}

		// 调用实际的领域事件处理器
		return handler(ctx, domainEvent)
	}

	// 注册到事件总线
	var err error
	_, exists := s.subscribers[eventType]
	if !exists {
		err = s.eventBus.Sub(eventType, wrappedHandler)
		s.subscribers[eventType] = []*subscriberInfo{}
	}

	if err != nil {
		return fmt.Errorf("failed to subscribe to event %s: %w", eventType, err)
	}

	// 记录订阅信息
	s.subscribers[eventType] = append(s.subscribers[eventType], &subscriberInfo{
		handler:     handler,
		eventType:   eventType,
		serviceName: serviceName,
	})

	return nil
}

func (s *EventSubscriber) Unsubscribe(eventType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取该事件类型的所有订阅信息
	_, exists := s.subscribers[eventType]
	if !exists {
		return nil // 没有找到订阅信息，直接返回
	}

	// 从事件总线中取消订阅
	err := s.eventBus.Unsub(eventType)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from event %s: %w", eventType, err)
	}

	// 从记录中移除
	delete(s.subscribers, eventType)

	return nil
}

// Close 关闭订阅器，清理所有订阅

func (s *EventSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清空订阅记录
	s.subscribers = make(map[string][]*subscriberInfo)

	// 注意：实际的事件总线关闭由外部管理
	// EventSubscriber不负责关闭底层的EventBus

	return nil
}

// subscriberInfo 存储订阅信息
// 记录领域事件处理器及其相关配置
// 用于管理订阅关系和实现订阅取消功能
type subscriberInfo struct {
	// handler: 领域事件处理器函数
	handler handlers.DomainEventHandler
	// eventType: 订阅的事件类型
	eventType string
	// serviceName: 服务名称过滤器，用于过滤特定服务的事件
	serviceName string
}
