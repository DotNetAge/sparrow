package middlewares

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/gin-gonic/gin"
)

// 生成RSA密钥对用于测试
func generateRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// 生成签名用于测试
func generateSignature(privateKey *rsa.PrivateKey, data string) (string, error) {
	hash := sha256.New()
	hash.Write([]byte(data))
	digest := hash.Sum(nil)

	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, digest, nil)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// 测试正常情况下的认证
func TestRSAClientAuthMiddleware_Normal(t *testing.T) {
	// 生成RSA密钥对
	privateKey, publicKey, err := generateRSAKeyPair()
	if err != nil {
		t.Fatalf("生成RSA密钥对失败: %v", err)
	}

	// 设置测试数据
	clientID := "test_client"
	nonce := "test_nonce_123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := "test_body"
	dataToSign := clientID + nonce + timestamp + body

	// 生成签名
	signature, err := generateSignature(privateKey, dataToSign)
	if err != nil {
		t.Fatalf("生成签名失败: %v", err)
	}

	// 创建中间件
	publicKeys := map[string]*rsa.PublicKey{
		clientID: publicKey,
	}
	middleware := RSAClientAuthMiddleware(publicKeys)

	// 创建测试路由
	r := gin.Default()
	r.POST("/test", middleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 创建测试请求
	req, err := http.NewRequest("POST", "/test", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("X-Client-ID", clientID)
	req.Header.Set("X-Client-Signature", signature)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证结果
	if w.Code != http.StatusOK {
		t.Fatalf("预期状态码200，实际得到%d: %s", w.Code, w.Body.String())
	}
}

// 测试缺少必要请求头的情况
func TestRSAClientAuthMiddleware_MissingHeaders(t *testing.T) {
	// 生成RSA密钥对
	_, publicKey, err := generateRSAKeyPair()
	if err != nil {
		t.Fatalf("生成RSA密钥对失败: %v", err)
	}

	// 创建中间件
	publicKeys := map[string]*rsa.PublicKey{
		"test_client": publicKey,
	}
	middleware := RSAClientAuthMiddleware(publicKeys)

	// 创建测试路由
	r := gin.Default()
	r.POST("/test", middleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 创建测试请求（缺少必要请求头）
	req, err := http.NewRequest("POST", "/test", strings.NewReader("test_body"))
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}

	// 只设置部分请求头
	req.Header.Set("X-Client-ID", "test_client")
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证结果
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("预期状态码401，实际得到%d: %s", w.Code, w.Body.String())
	}
}

// 测试无效客户端ID的情况
func TestRSAClientAuthMiddleware_InvalidClientID(t *testing.T) {
	// 生成RSA密钥对
	privateKey, _, err := generateRSAKeyPair()
	if err != nil {
		t.Fatalf("生成RSA密钥对失败: %v", err)
	}

	// 设置测试数据
	clientID := "invalid_client"
	nonce := "test_nonce_123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := "test_body"
	dataToSign := clientID + nonce + timestamp + body

	// 生成签名
	signature, err := generateSignature(privateKey, dataToSign)
	if err != nil {
		t.Fatalf("生成签名失败: %v", err)
	}

	// 创建中间件（使用不同的客户端ID）
	publicKeys := map[string]*rsa.PublicKey{
		"valid_client": &privateKey.PublicKey,
	}
	middleware := RSAClientAuthMiddleware(publicKeys)

	// 创建测试路由
	r := gin.Default()
	r.POST("/test", middleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 创建测试请求
	req, err := http.NewRequest("POST", "/test", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("X-Client-ID", clientID)
	req.Header.Set("X-Client-Signature", signature)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证结果
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("预期状态码401，实际得到%d: %s", w.Code, w.Body.String())
	}
}

// 测试无效签名的情况
func TestRSAClientAuthMiddleware_InvalidSignature(t *testing.T) {
	// 生成RSA密钥对
	_, publicKey, err := generateRSAKeyPair()
	if err != nil {
		t.Fatalf("生成RSA密钥对失败: %v", err)
	}

	// 设置测试数据
	clientID := "test_client"
	nonce := "test_nonce_123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := "test_body"

	// 创建中间件
	publicKeys := map[string]*rsa.PublicKey{
		clientID: publicKey,
	}
	middleware := RSAClientAuthMiddleware(publicKeys)

	// 创建测试路由
	r := gin.Default()
	r.POST("/test", middleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 创建测试请求（使用无效签名）
	req, err := http.NewRequest("POST", "/test", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}

	// 设置请求头（使用无效签名）
	req.Header.Set("X-Client-ID", clientID)
	req.Header.Set("X-Client-Signature", "invalid_signature")
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证结果
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("预期状态码401，实际得到%d: %s", w.Code, w.Body.String())
	}
}

// 测试过期时间戳的情况
func TestRSAClientAuthMiddleware_ExpiredTimestamp(t *testing.T) {
	// 生成RSA密钥对
	privateKey, publicKey, err := generateRSAKeyPair()
	if err != nil {
		t.Fatalf("生成RSA密钥对失败: %v", err)
	}

	// 设置测试数据（使用过期的时间戳）
	clientID := "test_client"
	nonce := "test_nonce_123"
	expiredTimestamp := strconv.FormatInt(time.Now().Unix()-3600, 10) // 1小时前
	body := "test_body"
	dataToSign := clientID + nonce + expiredTimestamp + body

	// 生成签名
	signature, err := generateSignature(privateKey, dataToSign)
	if err != nil {
		t.Fatalf("生成签名失败: %v", err)
	}

	// 创建中间件
	publicKeys := map[string]*rsa.PublicKey{
		clientID: publicKey,
	}
	middleware := RSAClientAuthMiddleware(publicKeys)

	// 创建测试路由
	r := gin.Default()
	r.POST("/test", middleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 创建测试请求
	req, err := http.NewRequest("POST", "/test", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("X-Client-ID", clientID)
	req.Header.Set("X-Client-Signature", signature)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Timestamp", expiredTimestamp)
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证结果
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("预期状态码401，实际得到%d: %s", w.Code, w.Body.String())
	}
}

// 测试可配置中间件的功能
func TestConfigurableRSAClientAuthMiddleware(t *testing.T) {
	// 生成RSA密钥对
	privateKey, publicKey, err := generateRSAKeyPair()
	if err != nil {
		t.Fatalf("生成RSA密钥对失败: %v", err)
	}

	// 设置测试数据
	clientID := "test_client"
	nonce := "test_nonce_456"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := "test_body"
	dataToSign := clientID + nonce + timestamp + body

	// 生成签名
	signature, err := generateSignature(privateKey, dataToSign)
	if err != nil {
		t.Fatalf("生成签名失败: %v", err)
	}

	// 创建自定义配置
	config := auth.DefaultRSAAuthConfig()
	config.TimeWindow = 600 // 10分钟
	config.PublicKeyProvider = auth.NewMemoryPublicKeyProvider(map[string]*rsa.PublicKey{
		clientID: publicKey,
	})
	config.NonceStore = auth.NewMemoryNonceStore()
	config.SignatureCache = auth.NewMemorySignatureCache()

	// 创建可配置中间件
	middleware := ConfigurableRSAClientAuthMiddleware(config)

	// 创建测试路由
	r := gin.Default()
	r.POST("/test", middleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 创建测试请求
	req, err := http.NewRequest("POST", "/test", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("X-Client-ID", clientID)
	req.Header.Set("X-Client-Signature", signature)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证结果
	if w.Code != http.StatusOK {
		t.Fatalf("预期状态码200，实际得到%d: %s", w.Code, w.Body.String())
	}
}

// 测试重复请求（重放攻击）的情况
func TestRSAClientAuthMiddleware_ReplayAttack(t *testing.T) {
	// 生成RSA密钥对
	privateKey, publicKey, err := generateRSAKeyPair()
	if err != nil {
		t.Fatalf("生成RSA密钥对失败: %v", err)
	}

	// 设置测试数据
	clientID := "test_client"
	nonce := "test_nonce_789"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := "test_body"
	dataToSign := clientID + nonce + timestamp + body

	// 生成签名
	signature, err := generateSignature(privateKey, dataToSign)
	if err != nil {
		t.Fatalf("生成签名失败: %v", err)
	}

	// 创建自定义配置，禁用缓存以测试重放攻击
	config := auth.DefaultRSAAuthConfig()
	config.PublicKeyProvider = auth.NewMemoryPublicKeyProvider(map[string]*rsa.PublicKey{
		clientID: publicKey,
	})
	config.NonceStore = auth.NewMemoryNonceStore()
	// 禁用缓存，确保每次请求都执行完整的验证流程
	config.SignatureCache = nil

	// 创建中间件
	middleware := ConfigurableRSAClientAuthMiddleware(config)

	// 创建测试路由
	r := gin.Default()
	r.POST("/test", middleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 第一次请求（应该成功）
	req1, err := http.NewRequest("POST", "/test", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}
	req1.Header.Set("X-Client-ID", clientID)
	req1.Header.Set("X-Client-Signature", signature)
	req1.Header.Set("X-Nonce", nonce)
	req1.Header.Set("X-Timestamp", timestamp)
	req1.Header.Set("Content-Type", "application/json")

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("第一次请求预期状态码200，实际得到%d: %s", w1.Code, w1.Body.String())
	}

	// 第二次请求（使用相同的nonce，应该失败）
	req2, err := http.NewRequest("POST", "/test", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建测试请求失败: %v", err)
	}
	req2.Header.Set("X-Client-ID", clientID)
	req2.Header.Set("X-Client-Signature", signature)
	req2.Header.Set("X-Nonce", nonce)
	req2.Header.Set("X-Timestamp", timestamp)
	req2.Header.Set("Content-Type", "application/json")

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// 验证结果
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("第二次请求预期状态码401，实际得到%d: %s", w2.Code, w2.Body.String())
	}
}
