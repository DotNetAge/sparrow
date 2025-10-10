package middlewares

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRSAClientAuthMiddleware(t *testing.T) {
	// 设置测试环境
	gin.SetMode(gin.TestMode)

	// 生成RSA密钥对用于测试
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err, "生成RSA密钥对不应该返回错误")
	publicKey := &privateKey.PublicKey

	// 创建客户端公钥映射表
	clientPublicKeys := map[string]*rsa.PublicKey{
		"client-123": publicKey,
	}

	// 准备测试用例
	testCases := []struct {
		name             string
		clientID         string
		signature        string
		nonce            string
		timestamp        string
		expectedStatus   int
		expectedError    string
		hasClientContext bool
	}{{
		name:             "缺少必要的请求头",
		clientID:         "",
		signature:        "",
		nonce:            "",
		timestamp:        "",
		expectedStatus:   http.StatusUnauthorized,
		expectedError:    "缺少必要的客户端验证信息",
		hasClientContext: false,
	}, {
		name:             "未授权的客户端",
		clientID:         "unknown-client",
		signature:        "invalid-signature",
		nonce:            "random-nonce",
		timestamp:        "1234567890",
		expectedStatus:   http.StatusUnauthorized,
		expectedError:    "未授权的客户端",
		hasClientContext: false,
	}, {
		name:             "无效的签名格式",
		clientID:         "client-123",
		signature:        "invalid-base64-signature",
		nonce:            "random-nonce",
		timestamp:        "1234567890",
		expectedStatus:   http.StatusUnauthorized,
		expectedError:    "无效的签名格式",
		hasClientContext: false,
	}}

	// 运行测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试路由和处理器
			router := gin.New()
			router.Use(RSAClientAuthMiddleware(clientPublicKeys))
			router.GET("/test", func(c *gin.Context) {
				// 验证上下文是否包含客户端信息
				clientID, exists := c.Get("clientID")
				if tc.hasClientContext {
					assert.True(t, exists, "上下文应该包含clientID")
					assert.Equal(t, "client-123", clientID, "客户端ID应该正确")
				} else {
					// 如果不需要客户端上下文，这里不会执行到，因为中间件会中断请求
				}
				c.Status(http.StatusOK)
			})

			// 创建测试请求
			req, err := http.NewRequest(http.MethodGet, "/test", nil)
			assert.NoError(t, err, "创建测试请求不应该返回错误")

			// 设置请求头
			if tc.clientID != "" {
				req.Header.Set(ClientIDHeader, tc.clientID)
			}
			if tc.signature != "" {
				req.Header.Set(ClientSignatureHeader, tc.signature)
			}
			if tc.nonce != "" {
				req.Header.Set(NonceHeader, tc.nonce)
			}
			if tc.timestamp != "" {
				req.Header.Set(TimestampHeader, tc.timestamp)
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

func TestRSAClientAuthMiddleware_ValidSignature(t *testing.T) {
	// 设置测试环境
	gin.SetMode(gin.TestMode)

	// 生成RSA密钥对用于测试
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err, "生成RSA密钥对不应该返回错误")
	publicKey := &privateKey.PublicKey

	// 创建客户端公钥映射表
	clientPublicKeys := map[string]*rsa.PublicKey{
		"client-123": publicKey,
	}

	// 准备验证数据
	clientID := "client-123"
	nonce := "random-nonce-123"
	timestamp := "1234567890"
	dataToSign := clientID + nonce + timestamp

	// 使用RSA PSS算法签名，指定hash函数为SHA256
	hash := sha256.New()
	hash.Write([]byte(dataToSign))
	digest := hash.Sum(nil)
	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, digest, nil)
	assert.NoError(t, err, "签名不应该返回错误")

	// Base64编码签名
	encodedSignature := base64.StdEncoding.EncodeToString(signature)

	// 创建测试路由和处理器
	router := gin.New()
	router.Use(RSAClientAuthMiddleware(clientPublicKeys))
	router.GET("/test", func(c *gin.Context) {
		// 验证上下文是否包含客户端信息
		clientID, exists := c.Get("clientID")
		assert.True(t, exists, "上下文应该包含clientID")
		assert.Equal(t, "client-123", clientID, "客户端ID应该正确")
		c.Status(http.StatusOK)
	})

	// 创建测试请求
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	assert.NoError(t, err, "创建测试请求不应该返回错误")

	// 设置请求头
	req.Header.Set(ClientIDHeader, clientID)
	req.Header.Set(ClientSignatureHeader, encodedSignature)
	req.Header.Set(NonceHeader, nonce)
	req.Header.Set(TimestampHeader, timestamp)

	// 执行请求
	wrr := httptest.NewRecorder()
	router.ServeHTTP(wrr, req)

	// 验证响应状态码
	assert.Equal(t, http.StatusOK, wrr.Code, "响应状态码应该是200")
}

func TestRSAClientAuthMiddleware_InvalidSignature(t *testing.T) {
	// 设置测试环境
	gin.SetMode(gin.TestMode)

	// 生成两个不同的RSA密钥对用于测试
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err, "生成RSA密钥对不应该返回错误")
	// 使用不同的密钥对进行验证
	differentPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err, "生成第二个RSA密钥对不应该返回错误")
	publicKey := &differentPrivateKey.PublicKey

	// 创建客户端公钥映射表
	clientPublicKeys := map[string]*rsa.PublicKey{
		"client-123": publicKey,
	}

	// 准备验证数据
	clientID := "client-123"
	nonce := "random-nonce-123"
	timestamp := "1234567890"
	dataToSign := clientID + nonce + timestamp

	// 使用不同的私钥签名，指定hash函数为SHA256
	hash := sha256.New()
	hash.Write([]byte(dataToSign))
	digest := hash.Sum(nil)
	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, digest, nil)
	assert.NoError(t, err, "签名不应该返回错误")

	// Base64编码签名
	encodedSignature := base64.StdEncoding.EncodeToString(signature)

	// 创建测试路由和处理器
	router := gin.New()
	router.Use(RSAClientAuthMiddleware(clientPublicKeys))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 创建测试请求
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	assert.NoError(t, err, "创建测试请求不应该返回错误")

	// 设置请求头
	req.Header.Set(ClientIDHeader, clientID)
	req.Header.Set(ClientSignatureHeader, encodedSignature)
	req.Header.Set(NonceHeader, nonce)
	req.Header.Set(TimestampHeader, timestamp)

	// 执行请求
	wrr := httptest.NewRecorder()
	router.ServeHTTP(wrr, req)

	// 验证响应状态码和错误消息
	assert.Equal(t, http.StatusUnauthorized, wrr.Code, "响应状态码应该是401")
	assert.Contains(t, wrr.Body.String(), "客户端签名验证失败", "响应应该包含签名验证失败的错误消息")
}

func TestNewRSAClientAuthMiddleware(t *testing.T) {
	// 生成RSA密钥对用于测试
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err, "生成RSA密钥对不应该返回错误")

	// 将公钥转换为PEM格式
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	assert.NoError(t, err, "转换公钥不应该返回错误")
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// 创建客户端公钥映射表
	clientKeys := map[string]string{
		"client-123": string(publicKeyPEM),
	}

	// 测试正常情况
	middleware, err := NewRSAClientAuthMiddleware(clientKeys)
	assert.NoError(t, err, "创建中间件不应该返回错误")
	assert.NotNil(t, middleware, "中间件不应该为nil")

	// 测试无效的公钥
	invalidClientKeys := map[string]string{
		"client-456": "invalid-public-key",
	}
	middleware, err = NewRSAClientAuthMiddleware(invalidClientKeys)
	assert.Error(t, err, "使用无效的公钥应该返回错误")
	assert.Nil(t, middleware, "使用无效的公钥时中间件应该为nil")
}
