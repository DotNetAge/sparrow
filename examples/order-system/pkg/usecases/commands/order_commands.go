package commands

// CreateOrderCommand创建订单命令
type CreateOrderCommand struct {
    CustomerID      string
    Items           []OrderItem
    ShippingAddress Address
}

type OrderItem struct {
    ProductID string
    Name      string
    Price     float64
    Quantity  int
}

type Address struct {
    Street  string
    City    string
    State   string
    ZipCode string
    Country string
}

// ConfirmOrderCommand确认订单命令
type ConfirmOrderCommand struct {
    OrderID string
}

// CancelOrderCommand取消订单命令
type CancelOrderCommand struct {
    OrderID string
}

// ShipOrderCommand发货订单命令
type ShipOrderCommand struct {
    OrderID string
}

// DeliverOrderCommand确认收货命令
type DeliverOrderCommand struct {
    OrderID string
}package commands
