package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenGenerator 定义了令牌生成器的接口
type TokenGenerator interface {
	// GrantToken 生成一个包含访问令牌和刷新令牌的 Token 结构体
	GrantToken(claims *UserClaims, scope string) (*Token, error)
	// GrantJWTToken 使用自定义 Claims 生成 JWT 令牌
	GrantJWTToken(claims *UserClaims) (string, error)
	// RefreshToken 刷新令牌，返回新的访问令牌和刷新令牌
	RefreshToken(token string, scope string) (*Token, error)
}

type tokenGenerator struct {
	secret     []byte
	expiresIn  time.Duration
	refreshExp time.Duration
}

// NewTokenGenerator 创建一个新的令牌生成器实例
func NewTokenGenerator(secret []byte, expiresIn time.Duration, refreshExp time.Duration) TokenGenerator {
	return &tokenGenerator{
		secret:     secret,
		expiresIn:  expiresIn,
		refreshExp: refreshExp,
	}
}

func (g *tokenGenerator) GrantToken(claims *UserClaims, scope string) (*Token, error) {
	// 实现令牌生成逻辑......
	// 1. 生成访问令牌
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(g.expiresIn))
	accessToken, err := g.GrantJWTToken(claims)
	if err != nil {
		return nil, err
	}

	// 2. 生成刷新令牌
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(g.refreshExp))
	refreshToken, err := g.GrantJWTToken(claims)
	if err != nil {
		return nil, err
	}

	return &Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Scope:        scope,
		ExpiresIn:    g.expiresIn,
	}, nil
}

// GrantJWTToken 使用自定义 Claims 生成 JWT 令牌
func (g *tokenGenerator) GrantJWTToken(claims *UserClaims) (string, error) {
	// 设置令牌的过期时间

	// 使用自定义 Claims 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 使用密钥对令牌进行签名
	tokenString, err := token.SignedString(g.secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (g *tokenGenerator) RefreshToken(tokenString string, scope string) (*Token, error) {
	// 解析刷新令牌
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return g.secret, nil
	})
	if err != nil {
		return nil, err
	}

	// 检查令牌是否有效
	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("无效的刷新令牌")
	}

	// 生成新令牌时更新签发时间
	claims.IssuedAt = jwt.NewNumericDate(time.Now())

	// 生成新的访问令牌和刷新令牌
	newToken, err := g.GrantToken(claims, scope)
	if err != nil {
		return nil, err
	}

	return newToken, nil
}
