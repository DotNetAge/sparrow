package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	jwt.RegisteredClaims
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"` // Roles 用户的角色
}

func NewUserClaims(issuer string, userID string, userName string, roles []string) *UserClaims {
	return &UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userName,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
		Roles:  roles,
	}
}
