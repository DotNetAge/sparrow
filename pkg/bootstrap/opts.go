package bootstrap

import (
	"strconv"
	"time"

	"github.com/DotNetAge/sparrow/pkg/adapter/http/middlewares"
	"github.com/DotNetAge/sparrow/pkg/adapter/http/router"
	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"

	// "github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/persistence/eventstore"
	"github.com/DotNetAge/sparrow/pkg/persistence/repo"
	"github.com/DotNetAge/sparrow/pkg/projection"
	"github.com/DotNetAge/sparrow/pkg/tasks"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"
	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

// ServerPort 配置服务器端口
func ServerPort(port int) Option {
	return func(o *App) {
		o.Config.Server.Port = port
	}
}

// ServerHost 配置服务器主机
func ServerHost(host string) Option {
	return func(o *App) {
		o.Config.Server.Host = host
	}
}

// SQLDB 配置SQL数据库连接
func SQLDB() Option {
	return func(o *App) {
	}
}

// RedisDB 配置Redis数据库连接
func RedisDB() Option {
	return func(app *App) {
		app.Container.Register(func() *redis.Client {
			return newRedis(&app.Config.Redis)
		})
	}
}

// Nats 配置NATS连接
func Nats() Option {
	return func(app *App) {
		app.Container.Register(func() *nats.Conn {
			cnn, err := nats.Connect(app.Config.NATS.URL)
			if err != nil {
				app.Logger.Error("连接NATS失败", "error", err)
				panic(err)
			}
			return cnn
		})
	}
}

// NatStreamBus 配置NATS JetStream事件总线
//
// 此方法会自动配置Nats连接实例与一个用于订阅本服务的事件总线。用于本服务的投影器处理服务内的领域事件。
func NatStreamBus() Option {
	return func(app *App) {
		app.Container.Register(func() *nats.Conn {
			cnn, err := nats.Connect(app.Config.NATS.URL)
			if err != nil {
				app.Logger.Error("连接NATS失败", "error", err)
				panic(err)
			}
			return cnn
		})
		// 订阅本服务的事件总线
		bus := messaging.NewJetStreamBus(app.NatsConn(), app.Name, app.Name, app.Logger)
		app.NeedCleanup(bus.(usecase.GracefulClose))
		app.Subscribers = bus.(messaging.Subscribers)
	}
}

// BadgerStore 配置Badger数据库连接
func BadgerDB() Option {
	return func(o *App) {
		o.Container.Register(func() *badger.DB {
			if o.Debug {
				opts := badger.DefaultOptions("").WithInMemory(true)
				db, err := badger.Open(opts)
				if err != nil {
					o.Logger.Error("创建BadgerDB的内存实例失败", "error", err)
					panic(err)
				}
				return db
			} else {
				// 当前为可应对每年10万行数据的吞吐量优化配置
				opts := badger.DefaultOptions(o.Config.Badger.DataDir).
					WithValueLogFileSize(256 << 20). // 256 MB
					WithValueThreshold(2 * 1024).    // 2 KB
					WithMemTableSize(64 << 20).      // 64 MB
					WithNumMemtables(2).
					WithCompression(options.ZSTD) // 或 badger.Snappy
					// 这是一个针对2万行数据的优化配置
					// WithValueLogFileSize(64 << 20). // 64 MB
					// WithValueThreshold(1024).       // 1 KB: 小于该值的 value 存在 LSM
					// WithNumMemtables(2).
					// WithMemTableSize(64 << 20).   // 64 MB
					// WithCompression(options.ZSTD) // 或 badger.Snappy

				db, err := badger.Open(opts)
				if err != nil {
					o.Logger.Error("创建BadgerDB实例失败", "error", err)
					panic(err)
				}
				return db
			}
		})
	}
}

// BadgerStore 使用Badger作为事件存储
func BadgerStore() Option {
	return func(o *App) {
		o.Container.Register(func() usecase.EventStore {
			store, err := eventstore.NewBadgerEventStore(o.Config.Badger.EventStoreDir, o.Logger)
			if err != nil {
				o.Logger.Error("创建Badger事件存储失败", "error", err)
				panic(err)
			}
			return store
		})
	}
}

func newRedis(config *config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     config.Host + ":" + strconv.Itoa(config.Port),
		Password: config.Password,
		DB:       config.DB,
	})
}

// RedisStore 使用Redis作为事件存储
func RedisStore() Option {
	return func(o *App) {
		o.Container.Register(func() usecase.EventStore {
			var redisClient *redis.Client
			if err := o.Container.ResolveInstance(&redisClient); err != nil {
				o.Logger.Error("解析Redis客户端失败", "error", err)
				panic(err)
			}
			store, err := eventstore.NewRedisEventStore(redisClient, o.Config.App.Name+":", o.Logger)
			if err != nil {
				o.Logger.Error("创建Redis事件存储失败", "error", err)
				panic(err)
			}
			return store
		})
	}
}

// SQLStore 使用SQL作为事件存储
func SQLStore() Option {
	return func(o *App) {
	}
}

// MemBus 使用内存作为事件总线
func MemBus() Option {
	return func(o *App) {
		o.Container.Register(eventbus.NewMemoryEventBus)
	}
}

// Tasks 使用任务系统
// 任务系统使用内存作为存储
// 向后兼容：无参数时使用默认配置（并发模式）
// 新功能：支持配置执行模式、工作协程数等
func Tasks(opts ...TaskOption) Option {
	return func(o *App) {
		// 默认配置，保持向后兼容
		config := &TaskConfig{
			WorkerCount:        5,
			MaxConcurrentTasks: 10,
			ExecutionMode:      tasks.ExecutionModeConcurrent, // 默认并发模式
			Logger:             o.Logger,
		}

		// 应用用户配置
		for _, opt := range opts {
			opt(config)
		}

		// 根据执行模式调整工作协程数
		// 顺序和流水线模式只需要1个工作协程，避免资源浪费
		actualWorkerCount := config.WorkerCount
		if config.ExecutionMode == tasks.ExecutionModeSequential ||
			config.ExecutionMode == tasks.ExecutionModePipeline {
			actualWorkerCount = 1
			if config.WorkerCount > 1 && config.Logger != nil {
				config.Logger.Warn("顺序/流水线模式下只能使用1个工作协程，已自动调整",
					"requested", config.WorkerCount, "actual", actualWorkerCount)
			}
		}

		// 创建单一调度器
		scheduler := tasks.NewMemoryTaskScheduler(
			tasks.WithLogger(config.Logger),
			tasks.WithWorkerCount(actualWorkerCount),
			tasks.WithMaxConcurrentTasks(config.MaxConcurrentTasks),
		)

		// 设置执行模式
		if err := scheduler.SetExecutionMode(config.ExecutionMode); err != nil {
			o.Logger.Error("设置执行模式失败", "mode", config.ExecutionMode, "error", err)
		}

		o.Scheduler = tasks.NewSchedulerWrapper(scheduler, DefaultRetryConfig())
		o.NeedCleanup(o.Scheduler.Instance.(usecase.GracefulClose))
	}
}

// TaskConfig 任务调度器配置
type TaskConfig struct {
	WorkerCount        int
	MaxConcurrentTasks int
	ExecutionMode      tasks.ExecutionMode
	Logger             *logger.Logger
}

// TaskOption 任务配置选项
type TaskOption func(*TaskConfig)

// WithWorkerCount 设置工作协程数
func WithWorkerCount(count int) TaskOption {
	return func(c *TaskConfig) {
		c.WorkerCount = count
	}
}

// WithMaxConcurrentTasks 设置最大并发任务数
func WithMaxConcurrentTasks(max int) TaskOption {
	return func(c *TaskConfig) {
		c.MaxConcurrentTasks = max
	}
}

// WithSequentialMode 设置为顺序执行模式
func WithSequentialMode() TaskOption {
	return func(c *TaskConfig) {
		c.ExecutionMode = tasks.ExecutionModeSequential
	}
}

// WithConcurrentMode 设置为并发执行模式
func WithConcurrentMode() TaskOption {
	return func(c *TaskConfig) {
		c.ExecutionMode = tasks.ExecutionModeConcurrent
	}
}

// WithPipelineMode 设置为流水线执行模式
func WithPipelineMode() TaskOption {
	return func(c *TaskConfig) {
		c.ExecutionMode = tasks.ExecutionModePipeline
	}
}

// HybridTasks 使用混合任务系统
// 支持并发、顺序、流水线三种执行策略，根据任务类型自动选择
func HybridTasks(opts ...tasks.HybridSchedulerOption) Option {
	return func(o *App) {
		// 默认配置
		defaultOpts := []tasks.HybridSchedulerOption{
			tasks.WithHybridLogger(o.Logger),
			tasks.WithHybridWorkerCount(5, 1, 1), // 并发5个，顺序1个，流水线1个
			tasks.WithHybridMaxConcurrentTasks(10),
		}

		// 合并用户配置
		allOpts := append(defaultOpts, opts...)

		o.Scheduler = tasks.NewSchedulerWrapper(tasks.NewHybridTaskScheduler(allOpts...), DefaultRetryConfig())
		o.NeedCleanup(o.Scheduler.Instance.(usecase.GracefulClose))
	}
}

// AdvancedTasks 高级任务系统配置
// 提供更灵活的任务系统配置选项，使用Option模式
func AdvancedTasks(opts ...AdvancedTaskOption) Option {
	return func(o *App) {
		// 默认配置
		config := &AdvancedTaskConfig{
			ConcurrentWorkers:         5,
			MaxConcurrentTasks:        10,
			TaskPolicies:              make(map[string]tasks.TaskExecutionPolicy),
			Logger:                    o.Logger,
			EnableSequentialExecution: false, // 默认不启用顺序执行
			EnablePipelineExecution:   false, // 默认不启用流水线执行
		}

		// 应用用户配置
		for _, opt := range opts {
			opt(config)
		}

		// 根据配置确定工作协程数
		sequentialWorkers := 0
		pipelineWorkers := 0

		if config.EnableSequentialExecution {
			sequentialWorkers = 1 // 顺序执行只需要1个工作协程
		}

		if config.EnablePipelineExecution {
			pipelineWorkers = 1 // 流水线执行只需要1个工作协程
		}

		// 创建混合调度器
		scheduler := tasks.NewHybridTaskScheduler(
			tasks.WithHybridLogger(config.Logger),
			tasks.WithHybridWorkerCount(config.ConcurrentWorkers, sequentialWorkers, pipelineWorkers),
			tasks.WithHybridMaxConcurrentTasks(config.MaxConcurrentTasks),
		)

		// 注册任务类型策略
		for taskType, policy := range config.TaskPolicies {
			if err := scheduler.RegisterTaskPolicy(taskType, policy); err != nil {
				o.Logger.Warn("注册任务策略失败", "taskType", taskType, "policy", policy, "error", err)
			}
		}

		o.Scheduler = tasks.NewSchedulerWrapper(scheduler, DefaultRetryConfig())
		o.NeedCleanup(o.Scheduler.Instance.(usecase.GracefulClose))
	}
}

// AdvancedTaskConfig 高级任务配置
type AdvancedTaskConfig struct {
	ConcurrentWorkers  int
	MaxConcurrentTasks int
	TaskPolicies       map[string]tasks.TaskExecutionPolicy
	Logger             *logger.Logger

	// 执行模式配置 - 新增更清晰的配置
	EnableSequentialExecution bool // 是否启用顺序执行模式
	EnablePipelineExecution   bool // 是否启用流水线执行模式
}

// AdvancedTaskOption 高级任务配置选项
type AdvancedTaskOption func(*AdvancedTaskConfig)

// WithConcurrentWorkers 设置并发工作协程数
func WithConcurrentWorkers(count int) AdvancedTaskOption {
	return func(c *AdvancedTaskConfig) {
		c.ConcurrentWorkers = count
	}
}

// EnableSequentialExecution 启用顺序执行模式
// 顺序执行模式下，任务将严格按照提交顺序串行执行，同时只会有一个任务在运行
func EnableSequentialExecution() AdvancedTaskOption {
	return func(c *AdvancedTaskConfig) {
		c.EnableSequentialExecution = true
	}
}

// EnablePipelineExecution 启用流水线执行模式
// 流水线执行模式下，任务将按阶段串行执行，适用于需要分阶段处理的任务
func EnablePipelineExecution() AdvancedTaskOption {
	return func(c *AdvancedTaskConfig) {
		c.EnablePipelineExecution = true
	}
}

// WithAdvancedMaxConcurrentTasks 设置最大并发任务数（高级任务系统）
func WithAdvancedMaxConcurrentTasks(max int) AdvancedTaskOption {
	return func(c *AdvancedTaskConfig) {
		c.MaxConcurrentTasks = max
	}
}

// WithSequentialType 指定任务类型为顺序执行
func WithSequentialType(taskTypes ...string) AdvancedTaskOption {
	return func(c *AdvancedTaskConfig) {
		if c.TaskPolicies == nil {
			c.TaskPolicies = make(map[string]tasks.TaskExecutionPolicy)
		}
		for _, taskType := range taskTypes {
			c.TaskPolicies[taskType] = tasks.PolicySequential
		}
	}
}

// WithPipelineType 指定任务类型为流水线执行
func WithPipelineType(taskTypes ...string) AdvancedTaskOption {
	return func(c *AdvancedTaskConfig) {
		if c.TaskPolicies == nil {
			c.TaskPolicies = make(map[string]tasks.TaskExecutionPolicy)
		}
		for _, taskType := range taskTypes {
			c.TaskPolicies[taskType] = tasks.PolicyPipeline
		}
	}
}

// ===== 重试配置选项 =====

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *config.RetryConfig {
	return &config.RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Enabled:           true,
	}
}

// WithRetry 配置任务重试能力
// 这是生产系统的核心能力，默认启用
func WithRetry(opts ...RetryOption) Option {
	return func(app *App) {
		config := DefaultRetryConfig()
		for _, opt := range opts {
			opt(config)
		}
		// 保存重试配置到App结构体
		// app.retryConfig = config

		// 确保使用支持重试的调度器
		if app.Scheduler == nil {
			// 如果没有调度器，创建支持重试的混合调度器
			app.Scheduler = &tasks.SchedulerWrapper{
				Instance: tasks.NewHybridTaskScheduler(
					tasks.WithHybridLogger(app.Logger),
					tasks.WithHybridWorkerCount(5, 1, 1),
					tasks.WithHybridMaxConcurrentTasks(10),
				),
				RetryConfig: config,
			}
			app.NeedCleanup(app.Scheduler.Instance.(usecase.GracefulClose))
		} else {
			app.Scheduler.RetryConfig = config
		}
	}
}

// RetryOption 重试配置选项
type RetryOption func(*config.RetryConfig)

// WithMaxRetries 设置最大重试次数
func WithMaxRetries(count int) RetryOption {
	return func(c *config.RetryConfig) {
		c.MaxRetries = count
	}
}

// WithInitialBackoff 设置初始退避时间
func WithInitialBackoff(duration time.Duration) RetryOption {
	return func(c *config.RetryConfig) {
		c.InitialBackoff = duration
	}
}

// WithMaxBackoff 设置最大退避时间
func WithMaxBackoff(duration time.Duration) RetryOption {
	return func(c *config.RetryConfig) {
		c.MaxBackoff = duration
	}
}

// WithExponentialBackoff 设置指数退避策略
func WithExponentialBackoff(multiplier float64) RetryOption {
	return func(c *config.RetryConfig) {
		c.BackoffMultiplier = multiplier
	}
}

// WithLinearBackoff 设置线性退避策略
func WithLinearBackoff() RetryOption {
	return func(c *config.RetryConfig) {
		c.BackoffMultiplier = 1.0
	}
}

// WithFixedBackoff 设置固定退避策略
func WithFixedBackoff() RetryOption {
	return func(c *config.RetryConfig) {
		c.BackoffMultiplier = 1.0
		c.MaxBackoff = c.InitialBackoff
	}
}

// DisableRetry 禁用重试（不推荐用于生产环境）
func DisableRetry() RetryOption {
	return func(c *config.RetryConfig) {
		c.Enabled = false
	}
}

// UseSession 使用会话系统
// 会话系统使用内存作为存储
func Sessions(expire time.Duration) Option {
	return func(o *App) {
		o.Container.RegisterNamed("sessionRepo", repo.NewMemoryRepository[*entity.Session])
		o.Container.Register(func() *usecase.SessionService {
			var repo usecase.Repository[*entity.Session]
			if err := o.Container.ResolveByName("SessionRepo", &repo); err != nil {
				o.Logger.Error("解析会话存储库失败", "error", err)
				panic(err)
			}
			return usecase.NewSessionService(repo, expire, o.Logger)
		})

		router.RegisterSessionRoutes(o.Engine, o.GetSessions())
	}
}

func HealthCheck() Option {
	return func(o *App) {
		router.RegisterHealthRoutes(&o.Config.App, o.Engine)
	}
}

func BadgerRepo[T entity.Entity]() Option {
	return func(o *App) {
		var db *badger.DB
		if err := o.Container.ResolveInstance(&db); err != nil {
			o.Logger.Panic("解析Badger数据库实例失败", "error", err)
		}
		name := utils.GetShotTypeName[T]()
		repoName := utils.Pascal(name + "Repo")
		prefix := utils.Snake(name)
		o.Container.RegisterNamed(repoName, func() usecase.Repository[T] {
			return repo.NewBadgerRepository[T](db, prefix)
		})
	}
}

func RedisRepo[T entity.Entity]() Option {
	return func(o *App) {
		var redisClient *redis.Client
		if err := o.Container.ResolveInstance(&redisClient); err != nil {
			o.Logger.Fatal("解析Redis客户端失败", "error", err)
		}
		name := utils.GetShotTypeName[T]()
		repoName := utils.Pascal(name + "Repo")
		prefix := utils.Snake(name)
		o.Container.RegisterNamed(repoName, func() usecase.Repository[T] {
			return repo.NewRedisRepository[T](redisClient, prefix, 0)
		})
	}
}

func Middlewares(middleware ...gin.HandlerFunc) Option {
	return func(o *App) {
		o.Engine.Use(middleware...)
	}
}

func WithJWT(expire time.Duration, refreshExp time.Duration) Option {

	return func(app *App) {
		app.Auth = Authorization{
			Tokens: auth.NewTokenGenerator([]byte(app.Config.App.Secret), expire, refreshExp),
			Middlewares: []gin.HandlerFunc{
				middlewares.JWTAuthMiddleware([]byte(app.Config.App.Secret)),
			},
		}
	}
}

func DebugMode(debug bool) Option {
	return func(o *App) {
		o.Debug = debug
	}
}
func WithName(name string) Option {
	return func(o *App) {
		o.Name = name
	}
}

// WithProjection 使用全量投影
//
// 此方法用于注册一个全量投影，该投影会从NATS流中读取事件并更新索引。
func WithProjection() Option {
	return func(o *App) {
		o.Container.Register(func() *projection.FullProjection {
			return projection.NewFullProjection(
				projection.NewJetStreamEventReader(o.NatsConn(), o.Name, o.Logger),
				projection.NewJetStreamIndexer(o.NatsConn(), o.Name, o.Logger),
			)
		})
	}
}
