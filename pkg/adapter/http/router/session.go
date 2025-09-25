package router

import (
	"github.com/DotNetAge/sparrow/pkg/adapter/http/handlers"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/gin-gonic/gin"
)

func RegisterSessionRoutes(engine *gin.Engine, sessionSvc *usecase.SessionService) {
	handler := handlers.NewSessionHandler(sessionSvc)
	engine.POST("/sessions", handler.CreateSession)
	engine.GET("/sessions/:id", handler.GetSession)
	engine.PUT("/sessions/:id", handler.UpdateSessionData)
	engine.GET("/sessions/:id/data", handler.GetSessionData)
	engine.DELETE("/sessions/:id", handler.DeleteSession)
}
