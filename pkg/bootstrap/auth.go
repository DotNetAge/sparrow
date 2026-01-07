package bootstrap

import (
	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/gin-gonic/gin"
)

type Authorization struct {
	Tokens           auth.TokenGenerator
	Middlewares      []gin.HandlerFunc
	namedMiddlewares map[string][]gin.HandlerFunc
}

func NewAuthorization(tokens auth.TokenGenerator) *Authorization {
	a := &Authorization{
		Tokens:           tokens,
		Middlewares:      []gin.HandlerFunc{},
		namedMiddlewares: map[string][]gin.HandlerFunc{},
	}
	return a
}

// AddMiddlewares 增加中间件
func (a *Authorization) AddMiddlewares(middleware ...gin.HandlerFunc) {
	a.Middlewares = append(a.Middlewares, middleware...)
}

// AddMiddleware 增加命名中间件
func (a *Authorization) AddMiddleware(name string, middleware gin.HandlerFunc) {
	a.namedMiddlewares[name] = append(a.namedMiddlewares[name], middleware)
	a.Middlewares = append(a.Middlewares, middleware)
}

// GetMiddleware 获取命名中间件
func (a *Authorization) GetMiddleware(name string) []gin.HandlerFunc {
	return a.namedMiddlewares[name]
}
