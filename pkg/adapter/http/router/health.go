package router

import (
	"purchase-service/pkg/adapter/http/handlers"
	"purchase-service/pkg/config"

	"github.com/gin-gonic/gin"
)

func RegisterHealthRoutes(config *config.AppConfig, engine *gin.Engine) {
	h := handlers.NewHealthHandler(config)
	// 健康检查端点
	engine.GET("/health", h.HealthCheck)
	// 存活检查端点
	engine.GET("/ready", h.ReadinessCheck)
}
