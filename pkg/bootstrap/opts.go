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
	sb "github.com/DotNetAge/sparrow/pkg/eventbus/nats"
	"github.com/DotNetAge/sparrow/pkg/eventbus/rabbitmq"
	redis_bus "github.com/DotNetAge/sparrow/pkg/eventbus/redis"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/persistence/eventstore"
	"github.com/DotNetAge/sparrow/pkg/persistence/repo"
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

// func(o *App) getConn() *nats.Conn {
// 		var cnn *nats.Conn
// 		if err := o.Container.ResolveInstance(&cnn); err != nil {
// 			o.Logger.Error("解析NATS连接失败", "error", err)
// 			panic(err)
// 		}
// 		return cnn

// 	}

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
		bus := messaging.NewJetStreamBus(app.NatsConn(), app.Name, app.Logger)
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

// Messaging 使用事件总线作为消息发布
func Messaging() Option {
	return func(a *App) {
		a.Container.Register(func() *messaging.EventPublisher {
			bus := a.GetEventBus()
			store := a.GetEventStore()
			if bus == nil || store == nil {
				a.Logger.Error("事件总线或事件存储未配置，无法创建事件发布者")
				panic("事件总线或事件存储未配置，无法创建事件发布者")
			}

			// appName := strings.ReplaceAll(a.Config.App.Name, "-", "_")
			return messaging.NewEventPublisher(store, bus, a.Name)
		})
	}
}

// NatsBus 使用NATS作为事件总线
func NatsBus() Option {
	return func(o *App) {
		o.Container.Register(func() eventbus.EventBus {
			eventBus, err := sb.NewNatsEventBus(&o.Config.NATS)
			if err != nil {
				o.Logger.Error("创建NATS事件总线失败", "error", err)
				panic(err)
			}
			return eventBus
		})
	}
}

func RedisBus() Option {
	return func(o *App) {
		o.Container.Register(func() eventbus.EventBus {
			eventBus, err := redis_bus.NewRedisEventBus(&o.Config.Redis)
			if err != nil {
				o.Logger.Error("创建Redis事件总线失败", "error", err)
				panic(err)
			}
			return eventBus
		})
	}
}

func RabbitMQBus() Option {
	return func(o *App) {
		o.Container.Register(func() eventbus.EventBus {
			eventBus, err := rabbitmq.NewRabbitMQEventBus(&o.Config.RabbitMQ)
			if err != nil {
				o.Logger.Error("创建RabbitMQ事件总线失败", "error", err)
				panic(err)
			}
			return eventBus
		})
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
func Tasks() Option {
	return func(o *App) {
		o.Scheduler = tasks.NewMemoryTaskScheduler(
			tasks.WithLogger(o.Logger),
		)
		o.NeedCleanup(o.Scheduler.(usecase.GracefulClose))
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

func RedisRepo[T entity.Entity](name string) Option {
	return func(o *App) {
		var redisClient *redis.Client
		if err := o.Container.ResolveInstance(&redisClient); err != nil {
			o.Logger.Fatal("解析Redis客户端失败", "error", err)
		}
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
	// return func(o *App) {
	// 	o.Container.Register(func() auth.TokenGenerator {
	// 		return auth.NewTokenGenerator([]byte(o.Config.App.Secret), expire, refreshExp)
	// 	})

	// 	o.Container.RegisterNamed("authMiddleware", func() gin.HandlerFunc {
	// 		return middlewares.JWTAuthMiddleware([]byte(o.Config.App.Secret))
	// 	})
	// }
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
