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
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/tasks"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	Scheduler    tasks.TaskScheduler
	retryCancel  context.CancelFunc // 用于取消重试goroutine的函数
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
// func (app *App) GetEventBus() eventbus.EventBus {
// 	var bus eventbus.EventBus
// 	if err := app.Container.ResolveInstance(&bus); err != nil {
// 		panic(fmt.Errorf("解析事件总线失败: %w", err))
// 	}
// 	return bus
// }

// // GetEventStore 获取事件存储实例
// func (app *App) GetEventStore() usecase.EventStore {
// 	var store usecase.EventStore
// 	if err := app.Container.ResolveInstance(&store); err != nil {
// 		panic(fmt.Errorf("解析事件存储失败: %w", err))
// 	}
// 	return store
// }

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

// GetNamedRepo 获取命名仓库实例
// name: 仓库名称，对应容器注册的名称（如 "user"）
func GetNamedRepo[T entity.Entity](app *App) (usecase.Repository[T], error) {
	var repo usecase.Repository[T]
	name := utils.GetShotTypeName[T]()
	if err := app.Container.ResolveByName(name+"Repo", &repo); err != nil {
		panic(fmt.Errorf("解析命名仓库失败[%s]: %w", name, err))
	}
	return repo, nil
}

func (app *App) Use(opts ...Option) *App {
	for _, opt := range opts {
		opt(app)
	}
	return app
}

func (app *App) Start() error {
	app.Logger.Info("启动相关子进程...")

	// 创建一个通道用于传递需要重试的子进程
	type retryItem struct {
		process usecase.Startable
		attempt int
	}
	retryChan := make(chan retryItem, len(app.SubProcesses))

	// 启动重试goroutine
	retryCtx, retryCancel := context.WithCancel(context.Background())
	app.retryCancel = retryCancel // 保存取消函数到App结构体

	go func() {
		for {
			select {
			case item := <-retryChan:
				// 指数退避重试
				backoffTime := time.Duration(1<<item.attempt) * time.Second
				if backoffTime > 1*time.Minute {
					backoffTime = 1 * time.Minute // 最大等待时间1分钟
				}

				app.Logger.Info("准备重试启动子进程",
					"attempt", item.attempt+1,
					"backoff", backoffTime,
				)

				// 等待退避时间
				select {
				case <-time.After(backoffTime):
					if err := item.process.Start(context.Background()); err != nil {
						app.Logger.Warn("子进程重试启动失败，将在稍后再次尝试",
							"attempt", item.attempt+1,
							"error", err,
						)
						// 继续重试，但限制最大重试次数
						if item.attempt < 10 { // 最多重试10次
							retryChan <- retryItem{
								process: item.process,
								attempt: item.attempt + 1,
							}
						} else {
							app.Logger.Error("子进程达到最大重试次数，放弃重试",
								"max_attempts", 10,
								"error", err,
							)
						}
					} else {
						app.Logger.Info("子进程重试启动成功",
							"attempt", item.attempt+1,
						)
					}
				case <-retryCtx.Done():
					return
				}
			case <-retryCtx.Done():
				return
			}
		}
	}()

	// 启动所有子进程
	for _, sub := range app.SubProcesses {
		needStart, ok := sub.(usecase.Startable)
		if !ok {
			continue
		}

		if err := needStart.Start(context.Background()); err != nil {
			app.Logger.Warn("启动子进程失败，将在后台重试", "error", err)
			// 将失败的进程添加到重试队列
			retryChan <- retryItem{
				process: needStart,
				attempt: 0,
			}
		} else {
			app.Logger.Info("子进程启动成功")
		}
	}

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
	// 取消重试goroutine
	if app.retryCancel != nil {
		app.retryCancel()
		app.Logger.Info("已取消所有子进程重试任务")
	}

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
	return messaging.NewJetStreamPublisher(
		app.NatsConn(),
		app.Name,
		appTypes,
		app.Logger,
		messaging.WithMaxAge(time.Duration(app.Config.NATS.MaxAge)*24*time.Hour), // 转换为天
	)
}

func (app *App) StreamReader(appType string) messaging.StreamReader {
	return messaging.NewJetStreamReader(app.NatsConn(), app.Name, appType, app.Logger)
}

// AddSub 添加一个订阅者
//
// 此方法用于添加一个订阅者到事件总线。订阅者会处理指定应用类型和事件类型的事件。
func (app *App) AddSub(appType, eventType string, handler messaging.DomainEventHandler[*entity.BaseEvent]) {
	app.Subscribers.AddHandler(appType, eventType, handler)
}

// NewHub 创建一个新的流处理中心
//
// 此方法用于增加一个订阅外部事件的流处理中心。
func (app *App) NewHub(serviceName string) *messaging.StreamHub {
	hub := messaging.NewStreamBus(app.NatsConn(), app.Name, serviceName, app.Logger)
	app.NeedCleanup(hub)
	return hub
}

func (app *App) RunTaskAt(at time.Time, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()
	task := tasks.NewTaskBuilder().
		WithID(taskId).
		ScheduleAt(at).
		WithHandler(handler).
		Build()
	app.Scheduler.Schedule(task)
	return taskId
}
func (app *App) RunTaskRecurring(interval time.Duration, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()
	task := tasks.NewTaskBuilder().
		WithID(taskId).
		ScheduleRecurring(interval).
		WithHandler(handler).
		Build()
	app.Scheduler.Schedule(task)
	return taskId
}

func (app *App) RunTask(handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()
	task := tasks.NewTaskBuilder().
		WithID(taskId).
		Immediate().
		WithHandler(handler).
		Build()
	app.Scheduler.Schedule(task)
	return taskId
}

// RunTypedTask 提交一个指定类型的任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// handler: 任务处理函数
// 返回: 任务ID
func (app *App) RunTypedTask(taskType string, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()
	task := tasks.NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		Immediate().
		WithHandler(handler).
		Build()
	app.Scheduler.Schedule(task)
	return taskId
}

// RunTypedTaskAt 提交一个指定类型的定时任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// at: 定时执行时间
// handler: 任务处理函数
// 返回: 任务ID
func (app *App) RunTypedTaskAt(taskType string, at time.Time, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()
	task := tasks.NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		ScheduleAt(at).
		WithHandler(handler).
		Build()
	app.Scheduler.Schedule(task)
	return taskId
}

// RunTypedTaskRecurring 提交一个指定类型的周期性任务（主要用于混合调度器）
// taskType: 任务类型，混合调度器会根据类型选择执行策略
// interval: 执行间隔
// handler: 任务处理函数
// 返回: 任务ID
func (app *App) RunTypedTaskRecurring(taskType string, interval time.Duration, handler func(ctx context.Context) error) string {
	taskId := uuid.New().String()
	task := tasks.NewTaskBuilder().
		WithID(taskId).
		WithType(taskType).
		ScheduleRecurring(interval).
		WithHandler(handler).
		Build()
	app.Scheduler.Schedule(task)
	return taskId
}
