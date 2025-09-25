package bootstrap

import (
	"strconv"
	"time"

	"github.com/DotNetAge/sparrow/pkg/adapter/http/router"
	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/eventbus/jetstream"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/persistence/eventstore"
	"github.com/DotNetAge/sparrow/pkg/persistence/repo"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/dgraph-io/badger/v4"
	"github.com/redis/go-redis/v9"
)

func WithPort(port int) Option {
	return func(o *App) {
		o.Config.Server.Port = port
	}
}

func WithHost(host string) Option {
	return func(o *App) {
		o.Config.Server.Host = host
	}
}

// UseSQL 配置SQL数据库连接
func UseSQL() Option {
	return func(o *App) {
	}
}

// UseRedis 配置Redis数据库连接
func UseRedis() Option {
	return func(o *App) {
		if o.Db.RedisClient == nil {
			o.Db.RedisClient = newRedis(&o.Config.Redis)
		}
	}
}

// UseBadgerStore 配置Badger数据库连接
func UseBadger() Option {
	return func(o *App) {
		if o.Db.BadgerDB == nil {
			db, err := badger.Open(badger.DefaultOptions(o.Config.Badger.DataDir))
			if err != nil {
				o.Logger.Error("Failed to create Badger DB", "error", err)
				panic(err)
			}
			o.Db.BadgerDB = db
		}
	}
}

// UseBadgerStore 使用Badger作为事件存储
func UseBadgerStore() Option {
	return func(o *App) {
		if o.Store == nil {
			store, err := eventstore.NewBadgerEventStore(o.Config.Badger.EventStoreDir, o.Logger)
			if err != nil {
				o.Logger.Error("Failed to create Badger event store", "error", err)
				panic(err)
			}
			o.Store = store
		}
	}
}

func newRedis(config *config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     config.Host + ":" + strconv.Itoa(config.Port),
		Password: config.Password,
		DB:       config.DB,
	})
}

// UseRedisStore 使用Redis作为事件存储
func UseRedisStore() Option {
	return func(o *App) {
		if o.Store == nil {
			if o.Db.RedisClient == nil {
				o.Db.RedisClient = newRedis(&o.Config.Redis)
			}
			store, err := eventstore.NewRedisEventStore(o.Db.RedisClient, o.Config.App.Name+":", o.Logger)
			if err != nil {
				o.Logger.Error("Failed to create Redis event store", "error", err)
				panic(err)
			}
			o.Store = store
		}
	}
}

// UseSQLStore 使用SQL作为事件存储
func UseSQLStore() Option {
	return func(o *App) {
	}
}

// UseNatsStore 使用NATS作为事件存储
func UseNatsStore() Option {
	return func(o *App) {
		if o.Store == nil {
			store, err := eventstore.NewNatsEventStore(&o.Config.NATS, o.Logger)
			if err != nil {
				o.Logger.Error("Failed to create NATS event store", "error", err)
				panic(err)
			}
			o.Store = store
		}
	}
}

func UseMessaging() Option {
	return func(a *App) {
		if a.Bus == nil {
			a.Logger.Error("Event bus is not configured, cannot create event publisher and subscriber")
			panic("Event bus is not configured, cannot create event publisher and subscriber")
		}

		if a.Publisher == nil {
			if a.Store == nil {
				a.Logger.Error("Event store is not configured, cannot create event publisher")
				panic("Event store is not configured, cannot create event publisher")
			}
			a.Publisher = messaging.NewEventPublisher(a.Store, a.Bus, a.Config.App.Name)
		}
		if a.Subscriber == nil {
			a.Subscriber = messaging.NewEventSubscriber(a.Bus)
		}
	}
}

// UseNatsBus 使用NATS作为事件总线
func UseNatsBus() Option {
	return func(o *App) {
		if o.Bus == nil {
			eventBus, err := jetstream.NewJetStreamEventBus(&o.Config.NATS)
			if err != nil {
				o.Logger.Error("Failed to create NATS event bus", "error", err)
				panic(err)
			}
			o.Bus = eventBus
		}
	}
}

// UseMemBus 使用内存作为事件总线
func UseMemBus() Option {
	return func(o *App) {
		o.Bus = eventbus.NewMemoryEventBus()
	}
}

// UseTasks 使用任务系统
// 任务系统使用内存作为存储
func UseTasks() Option {
	return func(o *App) {
		if o.Tasks == nil {
			memRepo := repo.NewMemoryRepository[*entity.Task]()
			o.Tasks = usecase.NewTaskService(memRepo, o.Logger)
		}
		router.RegisterTaskRoutes(o.Engine, o.Tasks)
	}
}

// UseSession 使用会话系统
// 会话系统使用内存作为存储
func UseSession(expire time.Duration) Option {
	return func(o *App) {
		if o.Sessions == nil {
			memRepo := repo.NewMemoryRepository[*entity.Session]()
			o.Sessions = usecase.NewSessionService(memRepo, expire, o.Logger)
		}
		router.RegisterSessionRoutes(o.Engine, o.Sessions)
	}
}

func UseHealtCheck() Option {
	return func(o *App) {
		router.RegisterHealthRoutes(&o.Config.App, o.Engine)
	}
}
