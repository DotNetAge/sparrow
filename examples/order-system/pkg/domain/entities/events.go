package entities

import (
	"github.com/DotNetAge/sparrow/pkg/entity"
)

// OrderCreatedPayload订单创建事件负载
type OrderCreatedPayload struct {
	CustomerID      string
	Items           []OrderItem
	TotalPrice      float64
	ShippingAddress Address
}

// OrderCreated订单创建事件
type OrderCreated struct {
	entity.BaseEvent
	Payload OrderCreatedPayload
}

func NewOrderCreated(aggregateID string, customerID string, items []OrderItem, totalPrice float64, shippingAddress Address) *OrderCreated {
	return &OrderCreated{
		BaseEvent: *entity.NewBaseEvent(aggregateID, "OrderCreated", "Order", 0),
		Payload: OrderCreatedPayload{
			CustomerID:      customerID,
			Items:           items,
			TotalPrice:      totalPrice,
			ShippingAddress: shippingAddress,
		},
	}
}

// OrderConfirmed订单确认事件
type OrderConfirmed struct {
	entity.BaseEvent
}

func NewOrderConfirmed(aggregateID string) *OrderConfirmed {
	return &OrderConfirmed{
		BaseEvent: *entity.NewBaseEvent(aggregateID, "OrderConfirmed", "Order", 0),
	}
}

// OrderCancelled订单取消事件
type OrderCancelled struct {
	entity.BaseEvent
}

func NewOrderCancelled(aggregateID string) *OrderCancelled {
	return &OrderCancelled{
		BaseEvent: *entity.NewBaseEvent(aggregateID, "OrderCancelled", "Order", 0),
	}
}

// OrderShipped订单发货事件
type OrderShipped struct {
	entity.BaseEvent
}

func NewOrderShipped(aggregateID string) *OrderShipped {
	return &OrderShipped{
		BaseEvent: *entity.NewBaseEvent(aggregateID, "OrderShipped", "Order", 0),
	}
}

// OrderDelivered订单送达事件
type OrderDelivered struct {
	entity.BaseEvent
}

func NewOrderDelivered(aggregateID string) *OrderDelivered {
	return &OrderDelivered{
		BaseEvent: *entity.NewBaseEvent(aggregateID, "OrderDelivered", "Order", 0),
	}
}
