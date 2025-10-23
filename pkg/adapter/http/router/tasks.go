package router

// import (
// 	"github.com/DotNetAge/sparrow/pkg/adapter/http/handlers"
// 	"github.com/DotNetAge/sparrow/pkg/usecase"
// 	"github.com/gin-gonic/gin"
// )

// func RegisterTaskRoutes(engine *gin.Engine, taskSvc *usecase.TaskService) {
// 	taskHandler := handlers.NewTaskHandler(taskSvc)
// 	engine.POST("/tasks", taskHandler.CreateTask)
// 	engine.GET("/tasks/:id", taskHandler.GetTask)
// 	engine.GET("/tasks", taskHandler.ListTasks)
// 	engine.POST("/tasks/:id/execute", taskHandler.ExecuteTask)
// 	engine.DELETE("/tasks/:id", taskHandler.DeleteTask)
// 	engine.POST("/tasks/:id/cancel", taskHandler.CancelTask)
// 	engine.GET("/tasks/:id/stats", taskHandler.GetTaskStats)
// 	engine.GET("/tasks/pending", taskHandler.GetPendingTasks)
// }
