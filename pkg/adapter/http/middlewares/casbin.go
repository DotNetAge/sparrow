package middlewares

import (
	"net/http"

	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

// RBACMiddleware 创建一个 Casbin 中间件
func RBACMiddleware(e *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取 Casbin subject（用户名）：
		//    优先从 request.Context 读取 JWTAuthMiddleware 写入的 Principal
		//    （auth.Current，JWT 签名校验后的可信身份），仅当 request.Context
		//    无 Principal 时，才回退兼容直接 c.Set("username") 的旧调用方。
		username := ""
		if p := auth.Current(c.Request.Context()); p != nil {
			username = p.Username
		} else if v, exists := c.Get("username"); exists {
			username, _ = v.(string)
		}
		if username == "" {
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
