package auth

import (
	"crypto/rsa"
	"sync"
	"time"
)

// MemoryNonceStore 是一个基于内存的NonceStore实现
// 使用map存储随机数和过期时间，并定期清理过期的随机数

type MemoryNonceStore struct {
	nonces map[string]time.Time
	mutex  sync.RWMutex
}

// NewMemoryNonceStore 创建一个新的MemoryNonceStore实例
func NewMemoryNonceStore() *MemoryNonceStore {
	store := &MemoryNonceStore{
		nonces: make(map[string]time.Time),
	}
	// 启动清理过期nonce的goroutine
	go store.cleanup()
	return store
}

// Set 存储一个随机数，带有过期时间
func (s *MemoryNonceStore) Set(nonce string, expiration time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.nonces[nonce] = expiration
	return nil
}

// Exists 检查随机数是否存在且未过期
func (s *MemoryNonceStore) Exists(nonce string) (bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	expiration, ok := s.nonces[nonce]
	if !ok {
		return false, nil
	}
	if time.Now().After(expiration) {
		return false, nil
	}
	return true, nil
}

// cleanup 定期清理过期的随机数
func (s *MemoryNonceStore) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mutex.Lock()
		now := time.Now()
		for nonce, expiration := range s.nonces {
			if now.After(expiration) {
				delete(s.nonces, nonce)
			}
		}
		s.mutex.Unlock()
	}
}

// MemoryPublicKeyProvider 是一个基于内存的PublicKeyProvider实现
// 使用map存储客户端ID到RSA公钥的映射

type MemoryPublicKeyProvider struct {
	keys  map[string]*rsa.PublicKey
	mutex sync.RWMutex
}

// NewMemoryPublicKeyProvider 创建一个新的MemoryPublicKeyProvider实例
func NewMemoryPublicKeyProvider(keys map[string]*rsa.PublicKey) *MemoryPublicKeyProvider {
	return &MemoryPublicKeyProvider{
		keys: keys,
	}
}

// GetPublicKey 根据客户端ID获取RSA公钥
func (p *MemoryPublicKeyProvider) GetPublicKey(clientID string) (*rsa.PublicKey, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	key, exists := p.keys[clientID]
	if !exists {
		return nil, ErrClientNotFound
	}
	return key, nil
}

// AddPublicKey 添加客户端ID和对应的RSA公钥
// 支持动态添加多个客户端ID
func (p *MemoryPublicKeyProvider) AddPublicKey(clientID string, publicKey *rsa.PublicKey) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.keys[clientID] = publicKey
}

// MemorySignatureCache 是一个基于内存的SignatureCache实现
// 使用map存储签名验证结果，并定期清理过期的结果

type MemorySignatureCache struct {
	cache map[string]cacheEntry
	mutex sync.RWMutex
}

type cacheEntry struct {
	valid      bool
	expiration time.Time
}

// NewMemorySignatureCache 创建一个新的MemorySignatureCache实例
func NewMemorySignatureCache() *MemorySignatureCache {
	cache := &MemorySignatureCache{
		cache: make(map[string]cacheEntry),
	}
	// 启动清理过期缓存的goroutine
	go cache.cleanup()
	return cache
}

// Get 获取签名验证结果
func (c *MemorySignatureCache) Get(key string) (bool, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	entry, ok := c.cache[key]
	if !ok {
		return false, nil
	}
	if time.Now().After(entry.expiration) {
		return false, nil
	}
	return entry.valid, nil
}

// Set 设置签名验证结果，带有过期时间
func (c *MemorySignatureCache) Set(key string, value bool, expiration time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[key] = cacheEntry{
		valid:      value,
		expiration: time.Now().Add(expiration),
	}
	return nil
}

// cleanup 定期清理过期的缓存条目
func (c *MemorySignatureCache) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mutex.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.After(entry.expiration) {
				delete(c.cache, key)
			}
		}
		c.mutex.Unlock()
	}
}
