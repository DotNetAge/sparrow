package handlers

import (
	"net/http"
	"os"
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
	name := os.Getenv("APP_NAME")
	if name == "" {
		name = h.config.Name
	}
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = h.config.Version
	}
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"message":   name + ":" + version + " is running",
		"name":      name,
		"version":   version,
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
