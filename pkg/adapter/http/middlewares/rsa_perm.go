package middlewares

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

// RSAClientAuthMiddleware 是一个基于RSA PSS签名的客户端验证中间件
// 它接收RSA公钥映射表，用于验证客户端的合法性
// 这是一个兼容旧接口的函数，内部使用新的实现
func RSAClientAuthMiddleware(clientPublicKeys map[string]*rsa.PublicKey) gin.HandlerFunc {
	// 创建默认配置
	config := auth.DefaultRSAAuthConfig()

	// 设置内存实现的公钥提供者
	config.PublicKeyProvider = auth.NewMemoryPublicKeyProvider(clientPublicKeys)
	// 设置内存实现的随机数存储
	config.NonceStore = auth.NewMemoryNonceStore()
	// 设置内存实现的签名缓存
	config.SignatureCache = auth.NewMemorySignatureCache()

	return ConfigurableRSAClientAuthMiddleware(config)
}

// ConfigurableRSAClientAuthMiddleware 是一个基于RSA PSS签名的客户端验证中间件
// 它接收一个配置结构体，支持灵活的配置
func ConfigurableRSAClientAuthMiddleware(config *auth.RSAAuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从请求头获取必要的验证信息
		clientID := c.GetHeader(config.ClientIDHeader)
		signature := c.GetHeader(config.ClientSignatureHeader)
		nonce := c.GetHeader(config.NonceHeader)
		timestamp := c.GetHeader(config.TimestampHeader)

		// 2. 验证必要的请求头是否存在
		if clientID == "" || signature == "" || nonce == "" || timestamp == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrMissingAuthInfo.Error()})
			c.Abort()
			return
		}

		// 3. 生成缓存键，用于性能优化
		cacheKey := clientID + ":" + signature + ":" + nonce + ":" + timestamp

		// 4. 检查缓存，如果命中则直接通过
		if config.SignatureCache != nil {
			valid, err := config.SignatureCache.Get(cacheKey)
			if err == nil && valid {
				// 缓存命中，直接通过
				c.Set("clientID", clientID)
				c.Next()
				return
			}
		}

		// 5. 验证时间戳
		timestampInt, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrInvalidTimestamp.Error()})
			c.Abort()
			return
		}

		// 验证时间戳是否在允许的时间窗口内
		now := time.Now().Unix()
		if math.Abs(float64(now-timestampInt)) > float64(config.TimeWindow) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrExpiredTimestamp.Error()})
			c.Abort()
			return
		}

		// 6. 验证随机数
		if config.NonceStore != nil {
			exists, err := config.NonceStore.Exists(nonce)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": auth.ErrInvalidNonce.Error()})
				c.Abort()
				return
			}
			if exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrDuplicateRequest.Error()})
				c.Abort()
				return
			}
		}

		// 7. 获取对应客户端的公钥
		publicKey, err := config.PublicKeyProvider.GetPublicKey(clientID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrClientNotFound.Error()})
			c.Abort()
			return
		}

		// 8. 读取请求体，用于验证
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": auth.ErrFailedToReadBody.Error()})
			c.Abort()
			return
		}
		// 重置请求体，以便后续处理
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))

		// 9. 构建要验证的数据（客户端ID + 随机数 + 时间戳 + 请求体）
		dataToVerify := clientID + nonce + timestamp + string(body)

		// 10. 解码Base64签名
		signatureBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrInvalidSignature.Error()})
			c.Abort()
			return
		}

		// 11. 使用RSA PSS算法验证签名
		hash := sha256.New()
		hash.Write([]byte(dataToVerify))
		digest := hash.Sum(nil)

		err = rsa.VerifyPSS(publicKey, crypto.SHA256, digest, signatureBytes, nil)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrFailedToVerifyRequest.Error()})
			c.Abort()
			return
		}

		// 12. 存储随机数，防止重放攻击
		if config.NonceStore != nil {
			expiration := time.Now().Add(time.Duration(config.TimeWindow) * time.Second)
			config.NonceStore.Set(nonce, expiration)
		}

		// 13. 缓存验证结果，提高性能
		if config.SignatureCache != nil {
			config.SignatureCache.Set(cacheKey, true, time.Duration(config.TimeWindow)*time.Second)
		}

		// 14. 验证通过，将客户端ID存入上下文
		c.Set("clientID", clientID)

		// 继续处理请求
		c.Next()
	}
}

// NewRSAClientAuthMiddleware 是一个工厂函数，用于创建RSAClientAuthMiddleware
// 它接收客户端ID和对应的PEM格式公钥字符串
func NewRSAClientAuthMiddleware(clientKeys map[string]string) (gin.HandlerFunc, error) {
	// 转换PEM格式公钥字符串为rsa.PublicKey对象
	publicKeys := make(map[string]*rsa.PublicKey)

	for clientID, pemKey := range clientKeys {
		key, err := parseRSAPublicKeyFromPEM([]byte(pemKey))
		if err != nil {
			return nil, errors.New("解析客户端公钥失败: " + clientID + ", " + err.Error())
		}
		publicKeys[clientID] = key
	}

	return RSAClientAuthMiddleware(publicKeys), nil
}

// NewConfigurableRSAClientAuthMiddlewareFromPEM 是一个工厂函数，用于创建ConfigurableRSAClientAuthMiddleware
// 它接收客户端ID和对应的PEM格式公钥字符串，并返回一个配置好的中间件
func NewConfigurableRSAClientAuthMiddlewareFromPEM(clientKeys map[string]string) (gin.HandlerFunc, error) {
	// 转换PEM格式公钥字符串为rsa.PublicKey对象
	publicKeys := make(map[string]*rsa.PublicKey)

	for clientID, pemKey := range clientKeys {
		key, err := parseRSAPublicKeyFromPEM([]byte(pemKey))
		if err != nil {
			return nil, errors.New("解析客户端公钥失败: " + clientID + ", " + err.Error())
		}
		publicKeys[clientID] = key
	}

	// 创建默认配置
	config := auth.DefaultRSAAuthConfig()
	// 设置内存实现的公钥提供者
	config.PublicKeyProvider = auth.NewMemoryPublicKeyProvider(publicKeys)
	// 设置内存实现的随机数存储
	config.NonceStore = auth.NewMemoryNonceStore()
	// 设置内存实现的签名缓存
	config.SignatureCache = auth.NewMemorySignatureCache()

	return ConfigurableRSAClientAuthMiddleware(config), nil
}

// parseRSAPublicKeyFromPEM 是一个辅助函数，用于从PEM格式字符串中解析RSA公钥
func parseRSAPublicKeyFromPEM(pemData []byte) (*rsa.PublicKey, error) {
	// 尝试解析标准PEM格式的RSA公钥
	block, _ := pem.Decode(pemData)
	if block != nil {
		// 尝试使用x509包解析标准格式的公钥
		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err == nil {
			if rsaKey, ok := publicKey.(*rsa.PublicKey); ok {
				return rsaKey, nil
			}
		}

		// 尝试使用ssh包解析
		publicKey, err = ssh.ParsePublicKey(block.Bytes)
		if err == nil {
			// 转换为RSA公钥
			if rsaPublicKey, ok := publicKey.(ssh.CryptoPublicKey); ok {
				if rsaKey, ok := rsaPublicKey.CryptoPublicKey().(*rsa.PublicKey); ok {
					return rsaKey, nil
				}
			}
		}
	}

	// 尝试直接使用ssh包解析
	publicKey, err := ssh.ParsePublicKey(pemData)
	if err == nil {
		if rsaPublicKey, ok := publicKey.(ssh.CryptoPublicKey); ok {
			if rsaKey, ok := rsaPublicKey.CryptoPublicKey().(*rsa.PublicKey); ok {
				return rsaKey, nil
			}
		}
	}

	return nil, errors.New("无法解析RSA公钥")
}
