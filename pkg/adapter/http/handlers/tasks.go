package handlers

import (
	"net/http"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/gin-gonic/gin"
)

// TaskHandler HTTP任务处理器
// 位于整洁架构的接口适配器层 - 处理HTTP请求

type TaskHandler struct {
	TaskService *usecase.TaskService
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(TaskService *usecase.TaskService) *TaskHandler {
	return &TaskHandler{
		TaskService: TaskService,
	}
}

// CreateTask 创建任务
// POST /tasks
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req struct {
		Type     string                 `json:"type" binding:"required"`
		Payload  map[string]interface{} `json:"payload"`
		Priority int                    `json:"priority"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.TaskService.CreateTask(c.Request.Context(), req.Type, req.Payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// GetTask 获取任务
// GET /tasks/:id
func (h *TaskHandler) GetTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.TaskService.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// ListTasks 列出任务
// GET /tasks
func (h *TaskHandler) ListTasks(c *gin.Context) {
	var req struct {
		Status *entity.TaskStatus `form:"status"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tasks, err := h.TaskService.ListTasks(c.Request.Context(), req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// ExecuteTask 执行任务
// POST /tasks/:id/execute
func (h *TaskHandler) ExecuteTask(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Async bool `json:"async"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Async {
		// 异步执行
		if err := h.TaskService.ExecuteTaskAsync(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"message": "task execution started"})
	} else {
		// 同步执行
		if err := h.TaskService.ExecuteTask(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "task executed successfully"})
	}
}

// DeleteTask 删除任务
// DELETE /tasks/:id
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.TaskService.DeleteTask(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "task deleted"})
}

// CancelTask 取消任务
// POST /tasks/:id/cancel
func (h *TaskHandler) CancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.TaskService.CancelTask(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "task cancelled"})
}

// GetTaskStats 获取任务统计
// GET /tasks/stats
func (h *TaskHandler) GetTaskStats(c *gin.Context) {
	stats, err := h.TaskService.GetTaskStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// GetPendingTasks 获取待处理任务
// GET /tasks/pending
func (h *TaskHandler) GetPendingTasks(c *gin.Context) {
	var req struct {
		Limit int `form:"limit,default=10"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tasks, err := h.TaskService.ListPendingTasks(c.Request.Context(), req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}
