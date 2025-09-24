package router

import (
	"github.com/DotNetAge/sparrow/pkg/adapter/http/handlers"
	"github.com/DotNetAge/sparrow/pkg/config"

	"github.com/gin-gonic/gin"
)

func RegisterHealthRoutes(config *config.AppConfig, engine *gin.Engine) {
	h := handlers.NewHealthHandler(config)
	// 健康检查端点
	engine.GET("/health", h.HealthCheck)
	// 存活检查端点
	engine.GET("/ready", h.ReadinessCheck)
}
