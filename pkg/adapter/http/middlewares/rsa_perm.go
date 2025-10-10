package middlewares

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

const (
	// ClientSignatureHeader 客户端签名的请求头
	ClientSignatureHeader = "X-Client-Signature"
	// ClientIDHeader 客户端ID的请求头
	ClientIDHeader = "X-Client-ID"
	// NonceHeader 随机数的请求头，用于防止重放攻击
	NonceHeader = "X-Nonce"
	// TimestampHeader 请求时间戳的请求头
	TimestampHeader = "X-Timestamp"
)

// RSAClientAuthMiddleware 是一个基于RSA PSS签名的客户端验证中间件
// 它接收RSA公钥映射表，用于验证客户端的合法性
func RSAClientAuthMiddleware(clientPublicKeys map[string]*rsa.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从请求头获取必要的验证信息
		clientID := c.GetHeader(ClientIDHeader)
		signature := c.GetHeader(ClientSignatureHeader)
		nonce := c.GetHeader(NonceHeader)
		timestamp := c.GetHeader(TimestampHeader)

		// 2. 验证必要的请求头是否存在
		if clientID == "" || signature == "" || nonce == "" || timestamp == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少必要的客户端验证信息"})
			c.Abort()
			return
		}

		// 3. 获取对应客户端的公钥
		publicKey, exists := clientPublicKeys[clientID]
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权的客户端"})
			c.Abort()
			return
		}

		// 4. 构建要验证的数据（客户端ID + 随机数 + 时间戳）
		dataToVerify := clientID + nonce + timestamp

		// 5. 解码Base64签名
		signatureBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的签名格式"})
			c.Abort()
			return
		}

		// 6. 使用RSA PSS算法验证签名
		hash := sha256.New()
		hash.Write([]byte(dataToVerify))
		digest := hash.Sum(nil)

		err = rsa.VerifyPSS(publicKey, crypto.SHA256, digest, signatureBytes, nil)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "客户端签名验证失败"})
			c.Abort()
			return
		}

		// 7. 验证通过，将客户端ID存入上下文
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
