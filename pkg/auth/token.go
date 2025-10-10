package auth

import "time"

// Token 令牌结构体, 用于返回访问令牌和刷新令牌
type Token struct {
	AccessToken  string        `json:"access_token"`  // AccessToken 资源访问令牌
	RefreshToken string        `json:"refresh_token"` // RefreshToken 刷新令牌
	Scope        string        `json:"scope"`         // Scope 授权范围
	ExpiresIn    time.Duration `json:"expires_in"`    // ExpiresIn 过期时限
}
