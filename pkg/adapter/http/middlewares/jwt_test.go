package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/auth"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestJWTAuthMiddleware(t *testing.T) {
	// 设置测试环境
	gin.SetMode(gin.TestMode)

	// 准备测试密钥和令牌生成器
	secret := []byte("test-secret")
	expiresIn := time.Hour
	refreshExp := time.Hour * 24
	generator := auth.NewTokenGenerator(secret, expiresIn, refreshExp)

	// 创建有效的UserClaims
	claims := auth.NewUserClaims("test-issuer", "user-123", "test-user", []string{"admin", "user"})

	// 生成有效的JWT令牌
	validToken, err := generator.GrantJWTToken(claims)
	assert.NoError(t, err, "生成有效令牌不应该返回错误")

	// 准备测试用例
	testCases := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedError  string
		hasUserContext bool
	}{{
		name:           "没有Authorization头",
		authHeader:     "",
		expectedStatus: http.StatusUnauthorized,
		expectedError:  "请求头中缺少Authorization字段",
		hasUserContext: false,
	}, {
		name:           "格式错误的Authorization头",
		authHeader:     "InvalidFormat token123",
		expectedStatus: http.StatusUnauthorized,
		expectedError:  "Authorization字段格式错误，应为 Bearer \\u003ctoken\\u003e",
		hasUserContext: false,
	}, {
		name:           "无效的令牌",
		authHeader:     "Bearer invalid.token.here",
		expectedStatus: http.StatusUnauthorized,
		expectedError:  "令牌格式错误 (Malformed)",
		hasUserContext: false,
	}, {
		name:           "过期的令牌",
		authHeader:     generateExpiredToken(secret),
		expectedStatus: http.StatusUnauthorized,
		expectedError:  "令牌已过期或尚未生效 (Expired/Not Valid Yet)",
		hasUserContext: false,
	}, {
		name:           "有效的令牌",
		authHeader:     "Bearer " + validToken,
		expectedStatus: http.StatusOK,
		expectedError:  "",
		hasUserContext: true,
	}}

	// 运行测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试路由和处理器
			router := gin.New()
			router.Use(JWTAuthMiddleware(secret))
			router.GET("/test", func(c *gin.Context) {
				// 验证上下文是否包含用户信息
					currentUser := auth.Current(c.Request.Context())
				if tc.hasUserContext {
					assert.True(t, currentUser != nil, "上下文应该包含currentUser")
					assert.Equal(t, "test-user", currentUser.Username, "用户名应该正确")
					assert.Equal(t, "user-123", currentUser.UserID, "用户ID应该正确")
					assert.Equal(t, []string{"admin", "user"}, currentUser.Roles, "角色应该正确")
				} else {
					// 如果不需要用户上下文，这里不会执行到，因为中间件会中断请求
				}

				c.Status(http.StatusOK)
			})

			// 创建测试请求
			req, err := http.NewRequest(http.MethodGet, "/test", nil)
			assert.NoError(t, err, "创建测试请求不应该返回错误")

			// 设置Authorization头
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			// 执行请求
			wrr := httptest.NewRecorder()
			router.ServeHTTP(wrr, req)

			// 验证响应状态码
			assert.Equal(t, tc.expectedStatus, wrr.Code, "响应状态码应该符合预期")

			// 验证错误消息
			if tc.expectedError != "" {
				assert.Contains(t, wrr.Body.String(), tc.expectedError, "响应应该包含预期的错误消息")
			}
		})
	}
}

// generateExpiredToken 生成一个已经过期的令牌用于测试
func generateExpiredToken(secret []byte) string {
	// 创建过期的claims
	claims := &auth.UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // 1小时前过期
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		UserID: "user-123",
		Roles:  []string{"admin", "user"},
	}

	// 创建并签名令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(secret)

	return "Bearer " + tokenString
}

func TestJWTAuthMiddleware_WithWrongSigningMethod(t *testing.T) {
	// 设置测试环境
	gin.SetMode(gin.TestMode)

	// 准备测试密钥
	secret := []byte("test-secret")

	// 注释：这里我们直接手动构造一个使用RS256签名方法的JWT字符串
	// 这足以触发签名方法不匹配的错误
	wrongMethodToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJ0ZXN0LWlzc3VlciIsInN1YiI6InRlc3QtdXNlciIsImV4cCI6MTYxMDAwMDAwMCwibmJmIjoxNjAwOTk2NDAwLCJpYXQiOjE2MDA5OTY0MDAsInVzZXJfaWQiOiJ1c2VyLTEyMyIsInJvbGVzIjpbImFkbWluIiwidXNlciJdfQ.invalid_signature"

	// 创建测试路由和处理器
	router := gin.New()
	router.Use(JWTAuthMiddleware(secret))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 创建测试请求
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+wrongMethodToken)

	// 执行请求
	wrr := httptest.NewRecorder()
	router.ServeHTTP(wrr, req)

	// 验证响应
	assert.Equal(t, http.StatusUnauthorized, wrr.Code, "响应状态码应该是401")
	assert.Contains(t, wrr.Body.String(), "令牌格式错误 (Malformed)", "响应应该包含无效令牌的错误消息")
}
