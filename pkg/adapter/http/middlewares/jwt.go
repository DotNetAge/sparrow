package middlewares

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTAuthMiddleware 是一个函数工厂，它接收jwt密钥并返回一个Gin中间件
func JWTAuthMiddleware(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从请求头获取令牌
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请求头中缺少Authorization字段"})
			c.Abort()
			return
		}

		const bearerSchema = "Bearer "
		if len(authHeader) < len(bearerSchema) || authHeader[:len(bearerSchema)] != bearerSchema {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization字段格式错误，应为 Bearer <token>"})
			c.Abort()
			return
		}
		tokenString := authHeader[len(bearerSchema):]

		// 2. 解析和验证令牌，使用UserClaims而不是RegisteredClaims
		token, err := jwt.ParseWithClaims(tokenString, &auth.UserClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secret, nil
		})

		// 3. 检查错误 (v5版本的核心)
		if err != nil {
			// 使用 v5 版本提供的 Is... 辅助函数来判断错误类型
			switch {
			case errors.Is(err, jwt.ErrTokenMalformed):
				// 令牌格式不正确
				c.JSON(http.StatusUnauthorized, gin.H{"error": "令牌格式错误 (Malformed)"})
			case errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet):
				// 令牌过期或尚未生效
				c.JSON(http.StatusUnauthorized, gin.H{"error": "令牌已过期或尚未生效 (Expired/Not Valid Yet)"})
			case errors.Is(err, jwt.ErrSignatureInvalid):
				// 签名无效
				c.JSON(http.StatusUnauthorized, gin.H{"error": "令牌签名无效 (Invalid Signature)"})
			default:
				// 其他任何验证错误
				c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的令牌 (Invalid Token)"})
			}
			c.Abort()
			return
		}

		// 4. 验证通过，提取claims
		if claims, ok := token.Claims.(*auth.UserClaims); ok && token.Valid {
			// 除了用户名外，还提取用户ID和角色信息
			ctx := context.WithValue(c.Request.Context(), auth.UserKey{}, auth.NewPrincipal(claims.Subject, claims.UserID, claims.Roles))
			c.Request = c.Request.WithContext(ctx)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无法提取有效的用户信息"})
			c.Abort()
			return
		}

		c.Next()
	}
}
