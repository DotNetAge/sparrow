package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type App struct {
	Config     *config.Config             // 全局配置
	Logger     *logger.Logger             // 全局日志
	Engine     *gin.Engine                // 全局路由引擎
	Bus        eventbus.EventBus          // 全局事件总线
	Publisher  *messaging.EventPublisher  // 全局消息发布器
	Subscriber *messaging.EventSubscriber // 全局消息订阅器
	Store      usecase.EventStore         // 全局事件存储
	Db         *Database                  // 全局数据库实例连接
	Tasks      *usecase.TaskService       // 全局任务服务
	Sessions   *usecase.SessionService    // 全局会话服务
}

type Option func(*App)

func NewApp(opts ...Option) *App {
	// 初始化配置
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("加载配置失败: %w", err))
	}

	log, err := logger.NewLogger(&cfg.Log)
	if err != nil {
		panic(fmt.Errorf("创建日志记录器失败: %w", err))
	}

	app := &App{
		Config: cfg,
		Logger: log,
		Engine: gin.Default(),
	}

	for _, opt := range opts {
		opt(app)
	}
	return app
}

func (app *App) Start() error {
	// 创建HTTP服务器
	srv := &http.Server{
		Addr:           app.Config.Server.Host + ":" + strconv.Itoa(app.Config.Server.Port),
		Handler:        app.Engine,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// 启动服务器
	go func() {
		app.Logger.Info("启动服务器", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.Logger.Fatal("服务器启动失败:", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.Logger.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		app.Logger.Fatal("服务器关闭失败:", zap.String("addr", srv.Addr), zap.Error(err))
	}
	// 释放资源
	app.CleanUp()
	app.Logger.Info("服务器已关闭")
	return nil
}

func (app *App) CleanUp() {
	// 进行清理操作，如关闭数据库连接、释放资源等
}
