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
	// 复制 claims，避免修改调用方持有的对象（副作用）。
	// 访问令牌与刷新令牌通过 TokenType claim 区分（access/refresh）。
	now := time.Now()

	// 1. 生成访问令牌
	access := *claims
	access.TokenType = "access"
	access.ExpiresAt = jwt.NewNumericDate(now.Add(g.expiresIn))
	accessToken, err := g.GrantJWTToken(&access)
	if err != nil {
		return nil, err
	}

	// 2. 生成刷新令牌
	refresh := *claims
	refresh.TokenType = "refresh"
	refresh.ExpiresAt = jwt.NewNumericDate(now.Add(g.refreshExp))
	refreshToken, err := g.GrantJWTToken(&refresh)
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

	// 仅允许 refresh 类型令牌刷新：显式标记为 "access" 的令牌一律拒绝，
	// 防止访问令牌被当作刷新令牌使用（token_type 为空视为旧版令牌，兼容放行）。
	if claims.TokenType == "access" {
		return nil, fmt.Errorf("访问令牌不能用于刷新")
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
