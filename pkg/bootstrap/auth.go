package bootstrap

import (
	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/gin-gonic/gin"
)

type Authorization struct {
	Tokens           auth.TokenGenerator
	Middlewares      []gin.HandlerFunc
	namedMiddlewares map[string]gin.HandlerFunc
}

func NewAuthorization(tokens auth.TokenGenerator) *Authorization {
	a := &Authorization{
		Tokens:           tokens,
		Middlewares:      []gin.HandlerFunc{},
		namedMiddlewares: map[string]gin.HandlerFunc{},
	}
	return a
}

// AddMiddlewares 增加中间件
func (a *Authorization) AddMiddlewares(middleware ...gin.HandlerFunc) {
	a.Middlewares = append(a.Middlewares, middleware...)
}

// AddMiddleware 增加命名中间件
func (a *Authorization) AddMiddleware(name string, middleware gin.HandlerFunc) {
	a.namedMiddlewares[name] = middleware
	a.Middlewares = append(a.Middlewares, middleware)
}

// GetMiddleware 获取命名中间件
// 若未注册该名称的中间件则 panic（fail-fast），避免 group.Use(nil) 在请求到达时
// 才崩溃（此类错误属于启动期配置错误，应在进程启动时暴露）。
func (a *Authorization) GetMiddleware(name string) gin.HandlerFunc {
	mw, ok := a.namedMiddlewares[name]
	if !ok {
		panic("未注册的命名中间件: " + name + "（请先通过 WithJWT/WithRSA/WithRBAC 或 AddMiddleware 注册）")
	}
	return mw
}
