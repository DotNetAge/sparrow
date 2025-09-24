package handlers

import (
	"net/http"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"

	"github.com/gin-gonic/gin"
)

// HealthHandler 健康检查处理器
type HealthHandler struct {
	config *config.AppConfig
}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler(cfg *config.AppConfig) *HealthHandler {
	return &HealthHandler{
		config: cfg,
	}
}

// HealthCheck 健康检查接口
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"message":   h.config.Name + ":" + h.config.Version + " is running",
		"timestamp": time.Now().Unix(),
	})
}

// ReadinessCheck 就绪检查接口
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"message":   h.config.Name + ":" + h.config.Version + " is ready to serve requests",
		"timestamp": time.Now().Unix(),
	})
}
