package handlers

import (
	"net/http"

	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SessionHandler HTTP会话处理器
// 位于整洁架构的接口适配器层 - 处理HTTP请求

type SessionHandler struct {
	sessionService *usecase.SessionService
}

// NewSessionHandler 创建会话处理器
func NewSessionHandler(sessionService *usecase.SessionService) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
	}
}

// CreateSession 创建会话
// POST /sessions
func (h *SessionHandler) CreateSession(c *gin.Context) {
	id := uuid.New().String()
	session, err := h.sessionService.CreateSession(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

// GetSession 获取会话
// GET /sessions/:id
func (h *SessionHandler) GetSession(c *gin.Context) {
	id := c.Param("id")

	session, err := h.sessionService.GetSession(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// UpdateSessionData 更新会话数据
// PUT /sessions/:id/data
func (h *SessionHandler) UpdateSessionData(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Key   string      `json:"key" binding:"required"`
		Value interface{} `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.sessionService.UpdateSessionData(c.Request.Context(), id, req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "data updated"})
}

// DeleteSession 删除会话
// DELETE /sessions/:id
func (h *SessionHandler) DeleteSession(c *gin.Context) {
	id := c.Param("id")

	if err := h.sessionService.DeleteSession(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session deleted"})
}

// GetSessionData 获取会话特定数据
// GET /sessions/:id/data/:key
func (h *SessionHandler) GetSessionData(c *gin.Context) {
	id := c.Param("id")
	key := c.Param("key")

	value, exists, err := h.sessionService.GetSessionData(c.Request.Context(), id, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": key, "value": value})
}
