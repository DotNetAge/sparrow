package routers

import (
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/adapters/handlers"
    "github.com/gin-gonic/gin"
)

// SetupRouter设置路由
func SetupRouter(orderHandler *handlers.OrderHandler) *gin.Engine {
    r := gin.Default()
    
    //检查
    r.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "status": "ok",
            "service": "order-system",
        })
    })
    
    // 订单相关路由
    orderGroup := r.Group("/api/v1/orders")
    {
        orderGroup.POST("", orderHandler.CreateOrder)
        orderGroup.POST("/:id/confirm", orderHandler.ConfirmOrder)
        orderGroup.POST("/:id/cancel", orderHandler.CancelOrder)
        orderGroup.POST("/:id/ship", orderHandler.ShipOrder)
        orderGroup.POST("/:id/deliver", orderHandler.DeliverOrder)
        orderGroup.GET("/:id", orderHandler.GetOrder)
        orderGroup.GET("", orderHandler.ListOrders)
    }
    
    return r
}package routers
