package auth

import (
	"crypto/rsa"
	"time"
)

// NonceStore 是一个用于存储和验证随机数的接口
// 用于防止重放攻击

type NonceStore interface {
	// Set 存储一个随机数，带有过期时间
	Set(nonce string, expiration time.Time) error
	// Exists 检查随机数是否存在且未过期
	Exists(nonce string) (bool, error)
}

// PublicKeyProvider 是一个用于获取RSA公钥的接口
// 根据客户端ID返回对应的RSA公钥

type PublicKeyProvider interface {
	// GetPublicKey 根据客户端ID获取RSA公钥
	GetPublicKey(clientID string) (*rsa.PublicKey, error)
}

// SignatureCache 是一个用于缓存签名验证结果的接口
// 用于提高性能

type SignatureCache interface {
	// Get 获取签名验证结果
	Get(key string) (bool, error)
	// Set 设置签名验证结果，带有过期时间
	Set(key string, value bool, expiration time.Duration) error
}

// RSAAuthConfig 是RSA认证的配置结构体
// 包含所有必要的配置参数

type RSAAuthConfig struct {
	// ClientIDHeader 客户端ID的请求头
	ClientIDHeader string
	// ClientSignatureHeader 客户端签名的请求头
	ClientSignatureHeader string
	// NonceHeader 随机数的请求头
	NonceHeader string
	// TimestampHeader 请求时间戳的请求头
	TimestampHeader string
	// TimeWindow 允许的时间窗口，单位为秒
	TimeWindow int64
	// NonceStore 随机数存储实现
	NonceStore NonceStore
	// SignatureCache 签名缓存实现
	SignatureCache SignatureCache
	// PublicKeyProvider 公钥提供者实现
	PublicKeyProvider PublicKeyProvider
}

// DefaultRSAAuthConfig 返回默认的RSA认证配置
func DefaultRSAAuthConfig() *RSAAuthConfig {
	return &RSAAuthConfig{
		ClientIDHeader:        "X-Client-ID",
		ClientSignatureHeader: "X-Client-Signature",
		NonceHeader:           "X-Nonce",
		TimestampHeader:       "X-Timestamp",
		TimeWindow:            300, // 5分钟
	}
}
