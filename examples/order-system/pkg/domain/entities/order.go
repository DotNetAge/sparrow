package entities

import (
	"github.com/DotNetAge/sparrow/pkg/entity"
)

// OrderStatus订单状态
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// OrderItem订单项
type OrderItem struct {
	ProductID string
	Name      string
	Price     float64
	Quantity  int
}

// Order订单聚合根
type Order struct {
	entity.BaseAggregateRoot
	CustomerID      string
	Items           []OrderItem
	TotalPrice      float64
	Status          OrderStatus
	ShippingAddress Address
}

// NewOrder 创建新订单
func NewOrder(customerID string, items []OrderItem, shippingAddress Address) *Order {
	order := &Order{
		CustomerID:      customerID,
		Items:           items,
		Status:          OrderStatusPending,
		ShippingAddress: shippingAddress,
	}

	order.CalculateTotal()
	event := NewOrderCreated(order.GetID(), customerID, items, order.TotalPrice, shippingAddress)
	order.AddEvent(event)

	return order
}

// CalculateTotal计算订单总价
func (o *Order) CalculateTotal() {
	total := 0.0
	for _, item := range o.Items {
		total += item.Price * float64(item.Quantity)
	}
	o.TotalPrice = total
}

// Confirm确认订单
func (o *Order) Confirm() error {
	if o.Status != OrderStatusPending {
		return &OrderStatusError{CurrentStatus: o.Status, ExpectedStatus: OrderStatusPending}
	}

	o.Status = OrderStatusConfirmed
	event := NewOrderConfirmed(o.GetID())
	o.AddEvent(event)
	return nil
}

// Cancel取订单订单
func (o *Order) Cancel() error {
	if o.Status == OrderStatusDelivered || o.Status == OrderStatusCancelled {
		return &OrderStatusError{CurrentStatus: o.Status, ExpectedStatus: "not delivered or cancelled"}
	}

	o.Status = OrderStatusCancelled
	event := NewOrderCancelled(o.GetID())
	o.AddEvent(event)
	return nil
}

// Ship 发货
func (o *Order) Ship() error {
	if o.Status != OrderStatusConfirmed {
		return &OrderStatusError{CurrentStatus: o.Status, ExpectedStatus: OrderStatusConfirmed}
	}

	o.Status = OrderStatusShipped
	event := NewOrderShipped(o.GetID())
	o.AddEvent(event)
	return nil
}

// Deliver确认收货
func (o *Order) Deliver() error {
	if o.Status != OrderStatusShipped {
		return &OrderStatusError{CurrentStatus: o.Status, ExpectedStatus: OrderStatusShipped}
	}

	o.Status = OrderStatusDelivered
	event := NewOrderDelivered(o.GetID())
	o.AddEvent(event)
	return nil
}
