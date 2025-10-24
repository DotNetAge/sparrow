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

	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Authorization struct {
	Tokens      auth.TokenGenerator
	Middlewares []gin.HandlerFunc
}

type App struct {
	Name         string
	Config       *config.Config // 全局配置
	Logger       *logger.Logger // 全局日志
	Engine       *gin.Engine    // 全局路由引擎
	Container    *Container     // 全局服务容器
	Debug        bool           // 是否开启调试模式
	SubProcesses []usecase.GracefulClose
	Subscribers  messaging.Subscribers
	Auth         Authorization
}

var (
	AppName    = ""
	AppVersion = "v0.0.1"
)

type Option func(*App)

func NewApp(opts ...Option) *App {
	// 初始化配置
	cfg, err := config.Load(AppName)
	if err != nil {
		panic(fmt.Errorf("加载配置失败: %w", err))
	}

	log, err := logger.NewLogger(&cfg.Log)
	if err != nil {
		panic(fmt.Errorf("创建日志记录器失败: %w", err))
	}

	if cfg.App.Name != "" {
		AppName = cfg.App.Name
	}

	if cfg.App.Version != "" {
		AppVersion = cfg.App.Version
	}

	r := gin.Default()
	// r.Use(cors.New(cors.Config{
	// 	AllowOrigins:     cfg.CORS.AllowOrigins,
	// 	AllowMethods:     cfg.CORS.AllowMethods,
	// 	AllowHeaders:     cfg.CORS.AllowHeaders,
	// 	AllowCredentials: cfg.CORS.AllowCredentials,
	// 	MaxAge:           time.Duration(cfg.CORS.MaxAgeHours) * time.Hour,
	// }), gzip.Gzip(gzip.DefaultCompression))

	app := &App{
		Name:         AppName,
		Config:       cfg,
		Logger:       log,
		Engine:       r,
		Container:    NewContainer(),
		SubProcesses: []usecase.GracefulClose{},
	}

	for _, opt := range opts {
		opt(app)
	}
	return app
}

// NeedCleanup 添加需要清理的子进程
func (app *App) NeedCleanup(process usecase.GracefulClose) {
	app.SubProcesses = append(app.SubProcesses, process)
}

// GetEventBus 获取事件总线实例
func (app *App) GetEventBus() eventbus.EventBus {
	var bus eventbus.EventBus
	if err := app.Container.ResolveInstance(&bus); err != nil {
		panic(fmt.Errorf("解析事件总线失败: %w", err))
	}
	return bus
}

// GetEventStore 获取事件存储实例
func (app *App) GetEventStore() usecase.EventStore {
	var store usecase.EventStore
	if err := app.Container.ResolveInstance(&store); err != nil {
		panic(fmt.Errorf("解析事件存储失败: %w", err))
	}
	return store
}

// GetPub 获取事件发布实例(基于数据实现)
func (app *App) GetPub() *messaging.EventPublisher {
	var pub *messaging.EventPublisher
	if err := app.Container.ResolveInstance(&pub); err != nil {
		panic(fmt.Errorf("解析事件发布器失败: %w", err))
	}
	return pub
}

// func (app *App) GetTasks() *usecase.TaskService {
// 	var tasks *usecase.TaskService
// 	if err := app.Container.ResolveInstance(&tasks); err != nil {
// 		panic(fmt.Errorf("解析任务服务失败: %w", err))
// 	}
// 	return tasks
// }

func (app *App) GetSessions() *usecase.SessionService {
	var sessions *usecase.SessionService
	if err := app.Container.ResolveInstance(&sessions); err != nil {
		panic(fmt.Errorf("解析会话服务失败: %w", err))
	}
	return sessions
}

func (app *App) getNamedRepo(name string) usecase.Repository[any] {
	var repo usecase.Repository[any]
	if err := app.Container.ResolveByName(name+"Repo", &repo); err != nil {
		panic(fmt.Errorf("解析命名仓库失败: %w", err))
	}
	return repo
}

// GetNamedRepo 获取命名仓库实例
// name: 仓库名称，对应容器注册的名称（如 "user"）
func GetNamedRepo[T any](app *App, name string) (usecase.Repository[T], error) {
	repo := app.getNamedRepo(name)
	typedRepo, ok := repo.(usecase.Repository[T])
	if !ok {
		app.Logger.Error("repo type mismatch", zap.String("name", name))
		return nil, fmt.Errorf("repo type mismatch")
	}
	return typedRepo, nil
}

func (app *App) Use(opts ...Option) *App {
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
	// 优雅关闭所有订阅器
	for _, subscriber := range app.SubProcesses {
		// 为每个订阅器创建5秒的超时上下文
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := subscriber.Close(ctx); err != nil {
			app.Logger.Error("清理资源失败", "error", zap.Error(err))
		} else {
			app.Logger.Info("资源已成功清理")
		}
	}
}

func (app *App) NatsConn() *nats.Conn {
	var cnn *nats.Conn
	if err := app.Container.ResolveInstance(&cnn); err != nil {
		app.Logger.Error("解释 NatsConn 连接失败")
		return nil
	}
	return cnn
}

func (app *App) StreamPub(appTypes ...string) messaging.StreamPublisher {
	return messaging.NewJetStreamPublisher(app.NatsConn(), app.Name, appTypes, app.Logger)
}

func (app *App) StreamReader(appType string) messaging.StreamReader {
	return messaging.NewJetStreamReader(app.NatsConn(), app.Name, appType, app.Logger)
}
