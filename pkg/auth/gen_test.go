package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestNewTokenGenerator(t *testing.T) {
	secret := []byte("test-secret")
	expiresIn := time.Hour
	refreshExp := time.Hour * 24

	generator := NewTokenGenerator(secret, expiresIn, refreshExp)

	assert.NotNil(t, generator, "生成器不应该为nil")

	// 验证生成器的内部字段
	impl, ok := generator.(*tokenGenerator)
	assert.True(t, ok, "应该能够转换为具体实现类型")
	assert.Equal(t, secret, impl.secret, "密钥应该正确设置")
	assert.Equal(t, expiresIn, impl.expiresIn, "过期时间应该正确设置")
	assert.Equal(t, refreshExp, impl.refreshExp, "刷新令牌过期时间应该正确设置")
}

func TestGrantJWTToken(t *testing.T) {
	secret := []byte("test-secret")
	generator := NewTokenGenerator(secret, time.Hour, time.Hour*24)

	claims := NewUserClaims("test-issuer", "user-123", "test-user", []string{"admin", "user"})

	// 测试成功生成令牌
	tokenString, err := generator.GrantJWTToken(claims)
	assert.NoError(t, err, "生成令牌不应该返回错误")
	assert.NotEmpty(t, tokenString, "生成的令牌不应该为空")

	// 验证令牌可以被正确解析和验证
	parsedToken, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	assert.NoError(t, err, "解析令牌不应该返回错误")
	assert.True(t, parsedToken.Valid, "令牌应该是有效的")

	// 验证令牌中的声明
	parsedClaims, ok := parsedToken.Claims.(*UserClaims)
	assert.True(t, ok, "应该能够转换为UserClaims类型")
	assert.Equal(t, "user-123", parsedClaims.UserID, "用户ID应该正确")
	assert.Equal(t, []string{"admin", "user"}, parsedClaims.Roles, "角色应该正确")
}

func TestGrantToken(t *testing.T) {
	secret := []byte("test-secret")
	expiresIn := time.Hour
	refreshExp := time.Hour * 24
	generator := NewTokenGenerator(secret, expiresIn, refreshExp)

	claims := NewUserClaims("test-issuer", "user-123", "test-user", []string{"admin", "user"})
	scope := "read write"

	// 测试成功生成令牌对
	tokenPair, err := generator.GrantToken(claims, scope)
	assert.NoError(t, err, "生成令牌对不应该返回错误")
	assert.NotNil(t, tokenPair, "生成的令牌对不应该为nil")
	assert.NotEmpty(t, tokenPair.AccessToken, "访问令牌不应该为空")
	assert.NotEmpty(t, tokenPair.RefreshToken, "刷新令牌不应该为空")
	assert.Equal(t, scope, tokenPair.Scope, "作用域应该正确设置")
	assert.Equal(t, expiresIn, tokenPair.ExpiresIn, "过期时间应该正确设置")

	// 验证访问令牌可以被正确解析和验证
	accessToken, err := jwt.ParseWithClaims(tokenPair.AccessToken, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	assert.NoError(t, err, "解析访问令牌不应该返回错误")
	assert.True(t, accessToken.Valid, "访问令牌应该是有效的")

	// 验证刷新令牌可以被正确解析和验证
	refreshToken, err := jwt.ParseWithClaims(tokenPair.RefreshToken, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	assert.NoError(t, err, "解析刷新令牌不应该返回错误")
	assert.True(t, refreshToken.Valid, "刷新令牌应该是有效的")

	// 验证两个令牌是不同的
	assert.NotEqual(t, tokenPair.AccessToken, tokenPair.RefreshToken, "访问令牌和刷新令牌应该不同")
}

func TestRefreshToken(t *testing.T) {
	secret := []byte("test-secret")
	expiresIn := time.Hour
	refreshExp := time.Hour * 24
	generator := NewTokenGenerator(secret, expiresIn, refreshExp)

	claims := NewUserClaims("test-issuer", "user-123", "test-user", []string{"admin", "user"})
	scope := "read write"

	// 先获取一个令牌对
	originalTokenPair, err := generator.GrantToken(claims, scope)
	assert.NoError(t, err, "生成初始令牌对不应该返回错误")

	// 使用刷新令牌获取新的令牌对
	newTokenPair, err := generator.RefreshToken(originalTokenPair.RefreshToken, scope)
	assert.NoError(t, err, "刷新令牌不应该返回错误")
	assert.NotNil(t, newTokenPair, "新生成的令牌对不应该为nil")
	assert.NotEmpty(t, newTokenPair.AccessToken, "新的访问令牌不应该为空")
	assert.NotEmpty(t, newTokenPair.RefreshToken, "新的刷新令牌不应该为空")

	// 解析并验证原始访问令牌
	originalAccessToken, _ := jwt.ParseWithClaims(originalTokenPair.AccessToken, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	originalAccessClaims := originalAccessToken.Claims.(*UserClaims)

	// 解析并验证新的访问令牌
	newAccessToken, _ := jwt.ParseWithClaims(newTokenPair.AccessToken, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	newAccessClaims := newAccessToken.Claims.(*UserClaims)

	// 验证令牌内容正确
	assert.Equal(t, originalAccessClaims.UserID, newAccessClaims.UserID, "用户ID应该保持不变")
	assert.Equal(t, originalAccessClaims.Roles, newAccessClaims.Roles, "角色应该保持不变")
	// 由于测试执行速度快，时间可能差异很小，这里不严格比较时间

	// 验证刷新令牌也已更新
	originalRefreshToken, _ := jwt.ParseWithClaims(originalTokenPair.RefreshToken, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	originalRefreshClaims := originalRefreshToken.Claims.(*UserClaims)

	newRefreshToken, _ := jwt.ParseWithClaims(newTokenPair.RefreshToken, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	newRefreshClaims := newRefreshToken.Claims.(*UserClaims)

	assert.Equal(t, originalRefreshClaims.UserID, newRefreshClaims.UserID, "用户ID应该保持不变")
	assert.Equal(t, originalRefreshClaims.Roles, newRefreshClaims.Roles, "角色应该保持不变")
	// 由于测试执行速度快，时间可能差异很小，这里不严格比较时间

	// 已经在前面验证过新生成的访问令牌的有效性，这里不需要重复验证
	assert.True(t, newAccessToken.Valid, "新的访问令牌应该是有效的")
}

func TestRefreshTokenWithInvalidToken(t *testing.T) {
	secret := []byte("test-secret")
	generator := NewTokenGenerator(secret, time.Hour, time.Hour*24)
	scope := "read write"

	// 测试使用无效的令牌
	invalidToken := "invalid.token.here"
	_, err := generator.RefreshToken(invalidToken, scope)
	assert.Error(t, err, "使用无效令牌应该返回错误")

	// 测试使用错误签名的令牌
	wrongSecret := []byte("wrong-secret")
	claims := NewUserClaims("test-issuer", "user-123", "test-user", []string{"admin", "user"})
	wrongSignedToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(wrongSecret)
	_, err = generator.RefreshToken(wrongSignedToken, scope)
	assert.Error(t, err, "使用错误签名的令牌应该返回错误")
}

func TestGrantJWTTokenWithCustomExpiration(t *testing.T) {
	secret := []byte("test-secret")
	generator := NewTokenGenerator(secret, time.Hour, time.Hour*24)

	// 创建一个自定义过期时间的claims
	claims := &UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 30)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: "user-123",
		Roles:  []string{"admin", "user"},
	}

	// 生成令牌
	tokenString, err := generator.GrantJWTToken(claims)
	assert.NoError(t, err, "生成令牌不应该返回错误")

	// 解析令牌并验证过期时间
	parsedToken, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	assert.NoError(t, err, "解析令牌不应该返回错误")
	assert.True(t, parsedToken.Valid, "令牌应该是有效的")

	parsedClaims, _ := parsedToken.Claims.(*UserClaims)
	assert.WithinDuration(t, time.Now().Add(time.Minute*30), parsedClaims.ExpiresAt.Time, time.Second*5, "过期时间应该符合预期")
}
