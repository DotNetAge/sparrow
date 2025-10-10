package auth

import "context"

type (
	UserKey struct{}
)

type Principal struct {
	Username string
	UserID   string
	Roles    []string
}

func NewPrincipal(username string, userID string, roles []string) *Principal {
	return &Principal{
		Username: username,
		UserID:   userID,
		Roles:    roles,
	}
}

func Current(c context.Context) *Principal {
	if p, ok := c.Value(UserKey{}).(*Principal); ok {
		return p
	}
	return nil
}
