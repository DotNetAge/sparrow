package projections

import (
    "context"
    "encoding/json"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/domain/entities"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/queries"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/repositories"
    "github.com/DotNetAge/sparrow/pkg/eventbus"
    "github.com/DotNetAge/sparrow/pkg/messaging"
)

// OrderProjection订单投影
type OrderProjection struct {
    orderRepo repositories.OrderRepository
    eventBus  eventbus.EventBus
}

// NewOrderProjection创建订单投影
func NewOrderProjection(orderRepo repositories.OrderRepository, eventBus eventbus.EventBus) *OrderProjection {
    return &OrderProjection{
        orderRepo: orderRepo,
        eventBus:  eventBus,
    }
}

// Start启动投影订阅
func (p *OrderProjection) Start(ctx context.Context) error {
    //订订单创建事件
    subscriber := messaging.NewEventSubscriber[*entities.OrderCreated]("order-service", "Order", "OrderCreated", p.eventBus, nil)
    if err := subscriber.AddHandler(p.handleOrderCreated); err != nil {
        return err
    }
    
    // 订阅订单确认事件
    subscriber2 := messaging.NewEventSubscriber[*entities.OrderConfirmed]("order-service", "Order", "OrderConfirmed", p.eventBus, nil)
    if err := subscriber2.AddHandler(p.handleOrderConfirmed); err != nil {
        return err
    }
    
    // 订阅订单取消事件
    subscriber3 := messaging.NewEventSubscriber[*entities.OrderCancelled]("order-service", "Order", "OrderCancelled", p.eventBus, nil)
    if err := subscriber3.AddHandler(p.handleOrderCancelled); err != nil {
        return err
    }
    
    // 订阅订单发货事件
    subscriber4 := messaging.NewEventSubscriber[*entities.OrderShipped]("order-service", "Order", "OrderShipped", p.eventBus, nil)
    if err := subscriber4.AddHandler(p.handleOrderShipped); err != nil {
        return err
    }
    
    // 订阅订单送达事件
    subscriber5 := messaging.NewEventSubscriber[*entities.OrderDelivered]("order-service", "Order", "OrderDelivered", p.eventBus, nil)
    if err := subscriber5.AddHandler(p.handleOrderDelivered); err != nil {
        return err
    }
    
    return nil
}

// handleOrderCreated处理订单创建事件
func (p *OrderProjection) handleOrderCreated(ctx context.Context, event *entities.OrderCreated) error {
    // 创建订单视图模型
    orderView := &queries.OrderViewModel{
        ID:         event.GetAggregateID(),
        CustomerID: event.Payload.CustomerID,
        Items:      convertEventItems(event.Payload.Items),
        TotalPrice: event.Payload.TotalPrice,
        Status:     string(entities.OrderStatusPending),
        ShippingAddress: queries.Address{
            Street:  event.Payload.ShippingAddress.Street,
            City:    event.Payload.ShippingAddress.City,
            State:   event.Payload.ShippingAddress.State,
            ZipCode: event.Payload.ShippingAddress.ZipCode,
            Country: event.Payload.ShippingAddress.Country,
        },
        CreatedAt: event.GetCreatedAt().Format("2006-01-02 15:04:05"),
        UpdatedAt: event.GetCreatedAt().Format("2006-01-02 15:04:05"),
    }
    
    // 保存到读模型仓储
    //这里简化处理，实际应该保存到专门的读模型存储
    return nil
}

// handleOrderConfirmed处理订单确认事件
func (p *OrderProjection) handleOrderConfirmed(ctx context.Context, event *entities.OrderConfirmed) error {
    // 更新订单状态为已确认
    return p.updateOrderStatus(event.GetAggregateID(), string(entities.OrderStatusConfirmed))
}

// handleOrderCancelled处理订单取消事件
func (p *OrderProjection) handleOrderCancelled(ctx context.Context, event *entities.OrderCancelled) error {
    // 更新订单状态为已取消
    return p.updateOrderStatus(event.GetAggregateID(), string(entities.OrderStatusCancelled))
}

// handleOrderShipped处理订单发货事件
func (p *OrderProjection) handleOrderShipped(ctx context.Context, event *entities.OrderShipped) error {
    // 更新订单状态为已发货
    return p.updateOrderStatus(event.GetAggregateID(), string(entities.OrderStatusShipped))
}

// handleOrderDelivered处理订单送达事件
func (p *OrderProjection) handleOrderDelivered(ctx context.Context, event *entities.OrderDelivered) error {
    // 更新订单状态为已送达
    return p.updateOrderStatus(event.GetAggregateID(), string(entities.OrderStatusDelivered))
}

// updateOrderStatus更新订单状态
func (p *OrderProjection) updateOrderStatus(orderID string, status string) error {
    // 这里应该更新读模型存储中的订单状态
    //简化处理，实际实现中应该有专门的读模型仓储
    return nil
}

// convertEventItems转换事件中的订单项
func convertEventItems(items []entities.OrderItem) []queries.OrderItem {
    var result []queries.OrderItem
    for _, item := range items {
        result = append(result, queries.OrderItem{
            ProductID: item.ProductID,
            Name:      item.Name,
            Price:     item.Price,
            Quantity:  item.Quantity,
        })
    }
    return result
}package projections
