package queries

// GetOrderQuery获取订单查询
type GetOrderQuery struct {
    OrderID string
}

// ListOrdersQuery列出订单查询
type ListOrdersQuery struct {
    CustomerID string
    Status     string
    Page       int
    PageSize   int
}

// OrderViewModel订单视图模型
type OrderViewModel struct {
    ID              string
    CustomerID      string
    Items           []OrderItem
    TotalPrice      float64
    Status          string
    ShippingAddress Address
    CreatedAt       string
    UpdatedAt       string
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
}package queries
