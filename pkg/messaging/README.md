# 领域事件模块使用指南

本模块提供了完整的领域事件发布和订阅机制，是实现事件驱动架构的核心组件。

## 目录结构

- `interfaces.go.tmpl` - 定义事件接口（Event, DomainEvent）
- `publisher.go.tmpl` - 实现事件发布器
- `subscriber.go.tmpl` - 实现事件订阅器
- `eventstore.go.tmpl` - 定义事件存储接口

## 核心接口说明

### Event 接口

基础事件接口，所有事件类型的统一抽象。

```go
type Event interface {
    GetEventID() string      // 事件唯一标识符
    GetEventType() string    // 事件类型
    GetCreatedAt() time.Time // 事件创建时间
}```

### DomainEvent 接口

领域事件接口，继承自Event接口，增加了聚合根相关属性。

```go
type DomainEvent interface {
    Event
    GetAggregateID() string   // 聚合根ID
    GetAggregateType() string // 聚合根类型
    GetVersion() int          // 事件版本，用于事件溯源
}```

### EventHandler 函数类型

事件处理函数，用于处理通用事件。

```go
type EventHandler func(ctx context.Context, event Event) error
```

### DomainEventHandler 函数类型

领域事件处理函数，专门用于处理领域事件。

```go
type DomainEventHandler func(ctx context.Context, event DomainEvent) error
```

## EventPublisher 组件

事件发布器负责发布领域事件，封装了EventBus和EventStore的操作。它实现了"服务名.聚合类.事件名"的主题命名规则，确保事件能够被正确路由到相应的订阅者。

### 使用方法

1. **初始化发布器**

```go
import "{{ .ModuleName }}/internal/entity/event"

// eventStore 是实现了 event.EventStore 接口的实例
// eventBus 是实现了 eventbus.EventBus 接口的实例
// serviceName 是当前服务的名称，用于构建事件主题
eventPublisher := event.NewEventPublisher(eventStore, eventBus, "order-service")
```

2. **发布聚合的所有事件**

```go
// 通过聚合根ID获取并发布所有事件
err := eventPublisher.PublishEvents(ctx, "aggregate-id")
if err != nil {
    // 处理错误
}
```

3. **发布聚合根中未提交的事件**

```go
// aggregate 是实现了 entity.AggregateRoot 接口的聚合根实例
err := eventPublisher.PublishUncommittedEvents(ctx, aggregate)
if err != nil {
    // 处理错误
}
```

## EventSubscriber 组件

事件订阅器负责订阅领域事件，封装了EventBus的订阅操作，提供更便捷的领域事件订阅能力。它支持根据事件类型和服务名称过滤事件，只接收符合条件的事件。

### 使用方法

1. **初始化订阅器**

```go
import "{{ .ModuleName }}/internal/entity/event"

// eventBus 是实现了 eventbus.EventBus 接口的实例
eventSubscriber := event.NewEventSubscriber(eventBus)
```

2. **订阅领域事件**

```go
// 订阅特定类型的领域事件
err := eventSubscriber.Subscribe("OrderCreatedEvent", func(ctx context.Context, evt event.DomainEvent) error {
    // 处理订单创建事件
    return nil
})
if err != nil {
    // 处理错误
}```

3. **带服务名称过滤的订阅**

```go
// 只订阅来自特定服务的事件
err := eventSubscriber.SubscribeWithFilter(
    "OrderCreatedEvent",
    "payment-service",
    func(ctx context.Context, evt event.DomainEvent) error {
        // 处理订单创建事件
        return nil
    }
)
if err != nil {
    // 处理错误
}```

4. **取消订阅**

```go
// 目前实现不支持根据ID取消单个订阅
// 如需停止接收事件，可以关闭整个订阅器```

7. **关闭订阅器**

```go
// 关闭订阅器，释放相关资源
err := eventSubscriber.Close()
if err != nil {
    // 处理错误
}
```

## 最佳实践

1. **事件命名规范**
   - 使用具体的、描述性的名称，如`OrderCreatedEvent`而不是`OrderEvent`
   - 采用过去时态表示事件已经发生
   - 遵循"服务名.聚合类.事件名"的主题命名规则，确保事件路由的一致性

2. **事件处理原则**
   - 事件处理器应该保持幂等性
   - 事件处理应该快速完成，避免长时间阻塞
   - 对于耗时操作，考虑在事件处理器中启动异步任务

3. **错误处理**
   - 发布事件失败时应记录日志并考虑重试策略
   - 事件处理器中的错误不应影响主业务流程

4. **服务间事件通信**
   - EventPublisher自动实现"服务名.聚合类.事件名"的主题命名规则，确保事件能够被正确路由
   - 使用明确的服务名称进行事件过滤，避免不必要的事件处理
   - 跨服务的关键业务流程考虑使用Saga模式确保一致性

## 与其他模块的集成

- 与EventBus集成：通过EventBus实现事件的实际传输，并遵循"服务名.聚合类.事件名"的主题命名规则
- 与EventStore集成：通过EventStore持久化和读取领域事件
- 与聚合根集成：聚合根生成事件，通过EventPublisher发布到事件总线

## 示例场景

### 订单创建场景

1. 订单服务创建订单，生成`OrderCreatedEvent`
2. 订单服务通过EventPublisher发布事件
3. 支付服务通过EventSubscriber订阅`OrderCreatedEvent`
4. 支付服务收到事件后，启动支付流程
5. 库存服务也订阅了`OrderCreatedEvent`，收到事件后更新库存

通过这种方式，各服务之间保持松耦合，同时能够响应业务事件进行协作。


```go

// 假设我们有以下领域事件处理器

func ExampleOrderCreatedHandler(ctx context.Context, domainEvent event.DomainEvent) error {
	// 处理订单创建事件
	fmt.Printf("处理订单创建事件: %s, 聚合ID: %s\n", 
		domainEvent.GetEventType(), 
		domainEvent.GetAggregateID())
	
	// 实际应用中，这里可以执行各种业务逻辑，如更新投影、触发其他领域事件等
	return nil
}

func ExamplePaymentCompletedHandler(ctx context.Context, domainEvent event.DomainEvent) error {
	// 处理支付完成事件
	fmt.Printf("处理支付完成事件: %s, 版本: %d\n", 
		domainEvent.GetEventType(), 
		domainEvent.GetVersion())
	return nil
}

// 初始化和使用EventSubscriber的示例

func SetupEventSubscribers(eventBus eventbus.EventBus) (*EventSubscriber, error) {
	// 创建事件订阅器
	subscriber := NewEventSubscriber(eventBus)
	
	// 订阅订单创建事件
	err := subscriber.Subscribe("OrderCreatedEvent", ExampleOrderCreatedHandler)
	if err != nil {
		return nil, fmt.Errorf("订阅订单创建事件失败: %w", err)
	}
	
	// 订阅特定服务的库存更新事件
	err = subscriber.SubscribeWithFilter("InventoryUpdatedEvent", "warehouse-service", func(ctx context.Context, domainEvent event.DomainEvent) error {
		fmt.Printf("收到来自warehouse-service的库存更新事件: %s\n", domainEvent.GetAggregateID())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("订阅库存更新事件失败: %w", err)
	}
	
	fmt.Println("事件订阅设置完成")
	return subscriber, nil
}

// 关闭订阅器的示例

func ShutdownSubscribers(subscriber *EventSubscriber) error {
	// 取消特定事件的订阅
	err := subscriber.Unsubscribe("OrderCreatedEvent")
	if err != nil {
		fmt.Printf("取消订阅订单创建事件出错: %v\n", err)
	}
	
	// 关闭所有订阅
	err = subscriber.Close()
	if err != nil {
		return fmt.Errorf("关闭事件订阅器失败: %w", err)
	}
	
	fmt.Println("事件订阅器已关闭")
	return nil
}
```