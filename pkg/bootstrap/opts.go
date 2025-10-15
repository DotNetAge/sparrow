package bootstrap

import (
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/adapter/http/router"
	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/eventbus/nats"
	"github.com/DotNetAge/sparrow/pkg/eventbus/rabbitmq"
	redis_bus "github.com/DotNetAge/sparrow/pkg/eventbus/redis"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/persistence/eventstore"
	"github.com/DotNetAge/sparrow/pkg/persistence/repo"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/DotNetAge/sparrow/pkg/utils"
	"github.com/dgraph-io/badger/v4"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/DotNetAge/sparrow/pkg/adapter/http/middlewares"
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

// BadgerStore 配置Badger数据库连接
func BadgerDB() Option {
	return func(o *App) {
		o.Container.Register(func() *badger.DB {
			if o.Debug {
				db, err := badger.Open(badger.Options{
					InMemory: o.Config.Badger.InMemory,
				})
				if err != nil {
					o.Logger.Error("创建BadgerDB实例失败", "error", err)
					panic(err)
				}
				return db
			} else {
				db, err := badger.Open(badger.DefaultOptions(o.Config.Badger.DataDir))
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

// NatsStore 使用NATS作为事件存储
func JetStreamStore() Option {
	return func(o *App) {
		o.Container.Register(func() usecase.EventStore {
			store, err := eventstore.NewNatsEventStore(&o.Config.NATS, o.Logger)
			if err != nil {
				o.Logger.Error("创建NATS事件存储失败", "error", err)
				panic(err)
			}
			return store
		})
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

			appName := strings.ReplaceAll(a.Config.App.Name, "-", "_")
			return messaging.NewEventPublisher(store, bus, utils.Pascal(utils.Snake(appName)))
		})
	}
}

// NatsBus 使用NATS作为事件总线
func NatsBus() Option {
	return func(o *App) {
		o.Container.Register(func() eventbus.EventBus {
			eventBus, err := nats.NewNatsEventBus(&o.Config.NATS)
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
		o.Container.RegisterNamed("taskRepo", repo.NewMemoryRepository[*entity.Task])
		o.Container.Register(func() *usecase.TaskService {
			var repo usecase.Repository[*entity.Task]
			if err := o.Container.ResolveByName("taskRepo", &repo); err != nil {
				o.Logger.Error("解析任务存储库失败", "error", err)
				panic(err)
			}
			return usecase.NewTaskService(repo, o.Logger)
		})
		router.RegisterTaskRoutes(o.Engine, o.GetTasks())
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

func BadgerRepo[T entity.Entity](name string) Option {
	return func(o *App) {
		var db *badger.DB
		if err := o.Container.ResolveInstance(&db); err != nil {
			o.Logger.Fatal("解析Badger数据库实例失败", "error", err)
			panic(err)
		}
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
			panic(err)
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
	return func(o *App) {
		o.Container.Register(func() auth.TokenGenerator {
			return auth.NewTokenGenerator([]byte(o.Config.App.Secret), expire, refreshExp)
		})

		o.Container.RegisterNamed("authMiddleware", func() gin.HandlerFunc {
			return middlewares.JWTAuthMiddleware([]byte(o.Config.App.Secret))
		})
	}
}
