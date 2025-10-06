package messaging

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/utils"
)

// EventSubscriber 事件订阅器
// 负责订阅事件总线上的领域事件，根据不同事件类型、聚合类型和服务名称将其分发给对应的处理器
// 负责对事件总线的事件类型进行主题(Topic)编码，格式为：服务名.聚合类型.事件类型
type EventSubscriber[T entity.DomainEvent] struct {
	eventBus    eventbus.EventBus
	logger      *logger.Logger
	serviceName string // 本服务名称
	aggType     string // 聚合类型
	eventType   string // 事件类型的字符串表示
	handlers    map[string]DomainEventHandler[T]
	mu          sync.RWMutex
}

// Init 初始化事件订阅器
// serviceName: 本服务名称，用于过滤事件
// aggType: 聚合类型
// bus: 事件总线实例
func (s *EventSubscriber[T]) Init(serviceName string, aggregateType string, bus eventbus.EventBus, logger *logger.Logger) {

	if bus == nil {
		panic("事件总线未进行初始化")
	}

	if serviceName == "" {
		panic("服务名称不能为空")
	}

	if aggregateType == "" {
		s.aggType = "*"
	} else {
		s.aggType = getRealTypeName(aggregateType)
	}

	s.serviceName = serviceName
	s.eventBus = bus
	s.logger = logger
	s.eventType = getRealTypeName(utils.GetTypeName[T]())
	s.handlers = make(map[string]DomainEventHandler[T])
}
func getRealTypeName(name string) string {
	if strings.Contains(name, ".") {
		segements := strings.Split(name, ".")
		return segements[len(segements)-1]
	} else {
		return name
	}
}

// Subscribe 订阅指定类型的领域事件,默认使用初始化时的服务名
// eventType: 事件类型
// handler: 领域事件处理器
func (s *EventSubscriber[T]) AddHandler(handler DomainEventHandler[T]) error {
	return s.AddServiceHandler(s.serviceName, handler)
}

// AddServiceHandler 订阅指定类型的领域事件，仅处理指定服务上的领域事件
// serviceName: 服务名称过滤器，为空时不过滤
// handler: 领域事件处理器
func (s *EventSubscriber[T]) AddServiceHandler(serviceName string, handler DomainEventHandler[T]) error {
	if handler == nil {
		s.logger.Error("领域事件处理器不能为空")
		return fmt.Errorf("领域事件处理器不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	subject := s.encodeEventType(serviceName)
	// 注册到事件总线

	// 创建包装处理器，用于从通用事件中提取领域事件数据并实现过滤
	wrappedHandler := func(ctx context.Context, evt eventbus.Event) error {
		// 从EventBus接收到的是通用事件，需要从中提取领域事件数据

		// 检查是否指定了服务名称过滤器
		if serviceName != "" {
			// 从完整事件类型名称中解析服务名
			// 格式为：服务名.聚合名.事件名
			subjectParts := strings.Split(subject, ".")
			if len(subjectParts) >= 1 {
				// 比较解析出的服务名与过滤器
				if subjectParts[0] != serviceName {
					// 服务名称不匹配，跳过此事件
					return nil
				}
				// 比较解析出的聚合类型与过滤器,支持通配符*
				if subjectParts[1] != s.aggType && s.aggType != "*" {
					// 聚合类型不匹配，跳过此事件
					return nil
				}
			}
		}

		// 从Payload中提取领域事件数据
		payload := evt.Payload.(map[string]interface{})
		eventData, ok := payload["eventData"]
		if !ok {
			s.logger.Error("事件数据中缺少领域事件数据")
			return fmt.Errorf("事件数据中缺少领域事件数据")
		}

		// 将事件数据转换为DomainEvent
		domainEvent, ok := eventData.(T)
		if !ok {
			s.logger.Error("无效的领域事件数据格式")
			return fmt.Errorf("无效的领域事件数据格式")
		}

		// 调用实际的领域事件处理器
		return handler(ctx, domainEvent)
	}
	err := s.eventBus.Sub(subject, wrappedHandler)

	if err != nil {
		s.logger.Error("订阅事件 %s 失败: %w", subject, err)
		return fmt.Errorf("订阅事件 %s 失败: %w", subject, err)
	}

	s.handlers[subject] = handler
	s.logger.Info("成功订阅事件 %s", subject)
	return nil
}

// encodeEventType 编码事件的最终主题名称，格式为：服务名.聚合名.事件名
func (s *EventSubscriber[T]) encodeEventType(serviceName string) string {
	svcName := serviceName
	if svcName == "" {
		svcName = s.serviceName
	}
	return fmt.Sprintf("%s.%s.%s", svcName, s.aggType, s.eventType)
}

// Unsubscribe 取消订阅指定服务名称的领域事件
// serviceName: 服务名称过滤器
func (s *EventSubscriber[T]) Unsubscribe(serviceName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从事件总线中取消订阅
	subject := s.encodeEventType(serviceName)
	err := s.eventBus.Unsub(subject)
	if err != nil {
		return fmt.Errorf("取消订阅事件 %s 失败: %w", subject, err)
	}
	// 从处理器映射中移除
	delete(s.handlers, subject)
	s.logger.Info("成功取消订阅事件 %s", subject)
	return nil
}

// DomainEventHandler 领域事件处理器接口
type DomainEventHandler[T entity.DomainEvent] func(ctx context.Context, event T) error
