package handlers

import (
    "net/http"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/commands"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/queries"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/services"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

// OrderHandler订单HTTP处理器
type OrderHandler struct {
    orderService *services.OrderApplicationService
}

// NewOrderHandler创建订单处理器
func NewOrderHandler(orderService *services.OrderApplicationService) *OrderHandler {
    return &OrderHandler{
        orderService: orderService,
    }
}

// CreateOrder创建订单
func (h *OrderHandler) CreateOrder(c *gin.Context) {
    var req CreateOrderRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    cmd := commands.CreateOrderCommand{
        CustomerID: req.CustomerID,
        Items:      convertRequestItems(req.Items),
        ShippingAddress: commands.Address{
            Street:  req.ShippingAddress.Street,
            City:    req.ShippingAddress.City,
            State:   req.ShippingAddress.State,
            ZipCode: req.ShippingAddress.ZipCode,
            Country: req.ShippingAddress.Country,
        },
    }
    
    orderID, err := h.orderService.CreateOrder(c.Request.Context(), cmd)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{
        "order_id": orderID,
        "message":  "Order created successfully",
    })
}

// ConfirmOrder确认订单
func (h *OrderHandler) ConfirmOrder(c *gin.Context) {
    var req OrderIDRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    cmd := commands.ConfirmOrderCommand{
        OrderID: req.OrderID,
    }
    
    if err := h.orderService.ConfirmOrder(c.Request.Context(), cmd); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "Order confirmed successfully",
    })
}

// CancelOrder取消订单
func (h *OrderHandler) CancelOrder(c *gin.Context) {
    var req OrderIDRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    cmd := commands.CancelOrderCommand{
        OrderID: req.OrderID,
    }
    
    if err := h.orderService.CancelOrder(c.Request.Context(), cmd); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "Order cancelled successfully",
    })
}

// ShipOrder发货订单
func (h *OrderHandler) ShipOrder(c *gin.Context) {
    var req OrderIDRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    cmd := commands.ShipOrderCommand{
        OrderID: req.OrderID,
    }
    
    if err := h.orderService.ShipOrder(c.Request.Context(), cmd); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "Order shipped successfully",
    })
}

// DeliverOrder确认收货
func (h *OrderHandler) DeliverOrder(c *gin.Context) {
    var req OrderIDRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    cmd := commands.DeliverOrderCommand{
        OrderID: req.OrderID,
    }
    
    if err := h.orderService.DeliverOrder(c.Request.Context(), cmd); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "Order delivered successfully",
    })
}

// GetOrder获取订单
func (h *OrderHandler) GetOrder(c *gin.Context) {
    orderID := c.Param("id")
    if orderID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "order id is required"})
        return
    }
    
    query := queries.GetOrderQuery{
        OrderID: orderID,
    }
    
    order, err := h.orderService.GetOrder(c.Request.Context(), query)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
        return
    }
    
    c.JSON(http.StatusOK, order)
}

// ListOrders列出订单
func (h *OrderHandler) ListOrders(c *gin.Context) {
    customerID := c.Query("customer_id")
    status := c.Query("status")
    
    query := queries.ListOrdersQuery{
        CustomerID: customerID,
        Status:     status,
        Page:       1,
        PageSize:   10,
    }
    
    orders, err := h.orderService.ListOrders(c.Request.Context(), query)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "orders": orders,
        "count":  len(orders),
    })
}

// CreateOrderRequest创建订单请求
type CreateOrderRequest struct {
    CustomerID      string           `json:"customer_id" binding:"required"`
    Items           []OrderItem      `json:"items" binding:"required"`
    ShippingAddress Address         `json:"shipping_address" binding:"required"`
}

type OrderItem struct {
    ProductID string  `json:"product_id" binding:"required"`
    Name      string  `json:"name" binding:"required"`
    Price     float64 `json:"price" binding:"required"`
    Quantity  int     `json:"quantity" binding:"required"`
}

type Address struct {
    Street  string `json:"street" binding:"required"`
    City    string `json:"city" binding:"required"`
    State   string `json:"state"`
    ZipCode string `json:"zip_code" binding:"required"`
    Country string `json:"country" binding:"required"`
}

// OrderIDRequest订单ID请求
type OrderIDRequest struct {
    OrderID string `json:"order_id" binding:"required"`
}

// convertRequestItems转换请求中的订单项
func convertRequestItems(items []OrderItem) []commands.OrderItem {
    var result []commands.OrderItem
    for _, item := range items {
        result = append(result, commands.OrderItem{
            ProductID: item.ProductID,
            Name:      item.Name,
            Price:     item.Price,
            Quantity:  item.Quantity,
        })
    }
    return result
}package handlers
