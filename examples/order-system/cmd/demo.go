package main

import (
	"log"

	"github.com/DotNetAge/sparrow/examples/order-system/pkg/domain/entities"
	"github.com/gin-gonic/gin"
)

// 简化版本的订单系统示例
func main() {
	// 创建内存存储(使用简单的map)
	orders := make(map[string]*entities.Order)

	r := gin.Default()

	//康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "order-system-demo",
		})
	})

	// 创建订单
	r.POST("/api/v1/orders", func(c *gin.Context) {
		var req CreateOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// 创建地址
		address := entities.Address{
			Street:  req.ShippingAddress.Street,
			City:    req.ShippingAddress.City,
			State:   req.ShippingAddress.State,
			ZipCode: req.ShippingAddress.ZipCode,
			Country: req.ShippingAddress.Country,
		}

		// 创建订单项
		var items []entities.OrderItem
		for _, item := range req.Items {
			items = append(items, entities.OrderItem{
				ProductID: item.ProductID,
				Name:      item.Name,
				Price:     item.Price,
				Quantity:  item.Quantity,
			})
		}

		// 创建订单聚合根
		order := entities.NewOrder(req.CustomerID, items, address)

		// 保存到内存存储
		orders[order.GetID()] = order

		//标记事件为已提交
		order.MarkEventsAsCommitted()

		c.JSON(201, gin.H{
			"order_id": order.GetID(),
			"message":  "Order created successfully",
		})
	})

	// 获取订单
	r.GET("/api/v1/orders/:id", func(c *gin.Context) {
		orderID := c.Param("id")

		order, exists := orders[orderID]
		if !exists {
			c.JSON(404, gin.H{"error": "Order not found"})
			return
		}

		response := OrderResponse{
			ID:         order.GetID(),
			CustomerID: order.CustomerID,
			Items:      convertItems(order.Items),
			TotalPrice: order.TotalPrice,
			Status:     string(order.Status),
			ShippingAddress: AddressResponse{
				Street:  order.ShippingAddress.Street,
				City:    order.ShippingAddress.City,
				State:   order.ShippingAddress.State,
				ZipCode: order.ShippingAddress.ZipCode,
				Country: order.ShippingAddress.Country,
			},
			CreatedAt: order.GetCreatedAt().Format("2006-01-02 15:04:05"),
			UpdatedAt: order.GetUpdatedAt().Format("2006-01-02 15:04:05"),
		}

		c.JSON(200, response)
	})

	//列出所有订单
	r.GET("/api/v1/orders", func(c *gin.Context) {
		var responses []OrderResponse

		for _, order := range orders {
			response := OrderResponse{
				ID:         order.GetID(),
				CustomerID: order.CustomerID,
				Items:      convertItems(order.Items),
				TotalPrice: order.TotalPrice,
				Status:     string(order.Status),
				ShippingAddress: AddressResponse{
					Street:  order.ShippingAddress.Street,
					City:    order.ShippingAddress.City,
					State:   order.ShippingAddress.State,
					ZipCode: order.ShippingAddress.ZipCode,
					Country: order.ShippingAddress.Country,
				},
				CreatedAt: order.GetCreatedAt().Format("2006-01-02 15:04:05"),
				UpdatedAt: order.GetUpdatedAt().Format("2006-01-02 15:04:05"),
			}
			responses = append(responses, response)
		}

		c.JSON(200, gin.H{
			"orders": responses,
			"count":  len(responses),
		})
	})

	//确认订单
	r.POST("/api/v1/orders/:id/confirm", func(c *gin.Context) {
		orderID := c.Param("id")

		order, exists := orders[orderID]
		if !exists {
			c.JSON(404, gin.H{"error": "Order not found"})
			return
		}

		if err := order.Confirm(); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// 更新内存存储
		orders[order.GetID()] = order
		order.MarkEventsAsCommitted()

		c.JSON(200, gin.H{
			"message": "Order confirmed successfully",
		})
	})

	log.Println("Starting order system demo on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// 请求和响应结构体
type CreateOrderRequest struct {
	CustomerID      string         `json:"customer_id" binding:"required"`
	Items           []OrderItem    `json:"items" binding:"required"`
	ShippingAddress AddressRequest `json:"shipping_address" binding:"required"`
}

type OrderItem struct {
	ProductID string  `json:"product_id" binding:"required"`
	Name      string  `json:"name" binding:"required"`
	Price     float64 `json:"price" binding:"required"`
	Quantity  int     `json:"quantity" binding:"required"`
}

type AddressRequest struct {
	Street  string `json:"street" binding:"required"`
	City    string `json:"city" binding:"required"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code" binding:"required"`
	Country string `json:"country" binding:"required"`
}

type OrderResponse struct {
	ID              string          `json:"id"`
	CustomerID      string          `json:"customer_id"`
	Items           []OrderItem     `json:"items"`
	TotalPrice      float64         `json:"total_price"`
	Status          string          `json:"status"`
	ShippingAddress AddressResponse `json:"shipping_address"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

type AddressResponse struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

func convertItems(items []entities.OrderItem) []OrderItem {
	var result []OrderItem
	for _, item := range items {
		result = append(result, OrderItem{
			ProductID: item.ProductID,
			Name:      item.Name,
			Price:     item.Price,
			Quantity:  item.Quantity,
		})
	}
	return result
}
