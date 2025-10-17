package http

import (
	h "net/http"

	"github.com/gin-gonic/gin"
)

// ERROR 通用错误响应
func ERROR(c *gin.Context, statusCode int, message string) {
	c.JSON(h.StatusOK, gin.H{"code": statusCode, "msg": message})
}

// BAD_REQ 400 错误响应
func BAD_REQ(c *gin.Context, message string) {
	c.JSON(h.StatusOK, gin.H{"code": h.StatusBadRequest, "msg": message})
}

// NOT_FOUND 404 错误响应
func NOT_FOUND(c *gin.Context, message string) {
	c.JSON(h.StatusOK, gin.H{"code": h.StatusNotFound, "msg": message})
}

// OK 200 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(h.StatusOK, gin.H{"code": 0, "msg": "success", "data": data})
}

// UNAUTH 401 未认证错误响应
func UNAUTH(c *gin.Context, message string) {
	c.JSON(h.StatusOK, gin.H{"code": h.StatusUnauthorized, "msg": message})
}

// DATA 200 数据成功响应
func DATA(c *gin.Context, data interface{}) {
	c.JSON(h.StatusOK, gin.H{"code": 0, "msg": "success", "data": data})
}

// PAGE_DATA 200 分页数据成功响应
func PAGE_DATA(c *gin.Context, data interface{}, total int, page, size int) {
	c.JSON(h.StatusOK, gin.H{
		"code":  0,
		"msg":   "success",
		"data":  data,
		"total": total,
		"page":  page,
		"size":  size,
	})
}
