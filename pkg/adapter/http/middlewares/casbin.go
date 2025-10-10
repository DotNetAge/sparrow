package middlewares

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

// NewCasbinMiddleware 创建一个 Casbin 中间件
func RBACMiddleware(e *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从上下文获取用户名 (假设 JWT 中间件已经将用户名存入了 "username")
		username, exists := c.Get("username")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			c.Abort()
			return
		}

		// 2. 获取资源 (URI) 和动作 (HTTP 方法)
		obj := c.Request.URL.Path
		act := c.Request.Method

		// 3. 执行权限检查
		ok, err := e.Enforce(username, obj, act)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "权限检查失败"})
			c.Abort()
			return
		}

		if ok {
			// 权限通过，继续处理请求
			c.Next()
		} else {
			// 权限拒绝
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			c.Abort()
		}
	}
}
