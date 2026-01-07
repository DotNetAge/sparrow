package bootstrap

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/adapter/http/middlewares"
	"github.com/DotNetAge/sparrow/pkg/adapter/http/router"
	"github.com/DotNetAge/sparrow/pkg/auth"
	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/casbin/casbin/v2"

	// "github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/messaging"
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
func SQLiteDB(schema string) Option {
	return func(app *App) {
		app.Container.Register(func() *sql.DB {
			db, err := sql.Open("sqlite3", app.Config.SQL.Dbname)
			if err != nil {
				app.Logger.Error("连接SQLite数据库失败", "error", err)
				panic(err)
			}

			// 启用WAL模式提高并发性能
			_, err = db.Exec(`
							PRAGMA journal_mode = WAL;
							PRAGMA synchronous = NORMAL;
							PRAGMA cache_size = -2000;  -- 2MB缓存
							PRAGMA busy_timeout = 5000;  -- 5秒超时
							PRAGMA foreign_keys = ON;
		`)

			if err != nil {
				app.Logger.Error("配置SQLite数据库失败", "error", err)
				panic(err)
			}

			_, err = db.Exec(schema)
			if err != nil {
				app.Logger.Error("执行SQLite数据库Schema失败", "error", err)
				panic(err)
			}
			return db
		})
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

func newRedis(config *config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     config.Host + ":" + strconv.Itoa(config.Port),
		Password: config.Password,
		DB:       config.DB,
	})
}

// MemBus 使用内存作为事件总线
func MemBus() Option {
	return func(o *App) {
		o.Container.Register(eventbus.NewMemoryEventBus)
	}
}

// Tasks 使用任务系统
// 简化版本：默认使用并发模式，提供基本配置选项
func Tasks(opts ...tasks.SchedulerWrapperOption) Option {
	return func(o *App) {
		// 创建任务调度器
		o.Scheduler = tasks.NewSchedulerWrapper(
			opts...,
		)
		o.NeedCleanup(o.Scheduler.Instance.(usecase.GracefulClose))
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

func SQLRepo[T entity.Entity]() Option {
	return func(o *App) {
		var db *sql.DB
		if err := o.Container.ResolveInstance(&db); err != nil {
			o.Logger.Panic("解析SQL数据库实例失败", "error", err)
		}
		name := utils.GetShotTypeName[T]()
		repoName := utils.Pascal(name + "Repo")
		// prefix := utils.Snake(name)
		o.Container.RegisterNamed(repoName, func() usecase.Repository[T] {
			return repo.NewSqlDBRepository[T](db)
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

func MemRepo[T entity.Entity]() Option {
	return func(o *App) {
		name := utils.GetShotTypeName[T]()
		repoName := utils.Pascal(name + "Repo")
		o.Container.RegisterNamed(repoName, repo.NewMemoryRepository[T])
	}
}

// 向应用添加中间件（最底层的方法）
func Middlewares(middleware ...gin.HandlerFunc) Option {
	return func(o *App) {
		o.Engine.Use(middleware...)
	}
}

func WithJWT(expire time.Duration, refreshExp time.Duration) Option {
	return func(app *App) {
		app.Auth = NewAuthorization(auth.NewTokenGenerator([]byte(app.Config.App.Secret), expire, refreshExp))
		app.Auth.AddMiddleware("jwt", middlewares.JWTAuthMiddleware([]byte(app.Config.App.Secret)))
	}
}

func loadClientKeysFromPath(keyPath string) map[string]string {
	clientPublicKeys := make(map[string]string)
	files, err := os.ReadDir(keyPath)
	if err != nil {
		panic(fmt.Sprintf("读取RSA密钥目录失败: %v", err))
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".pub") {
			continue
		}

		keyFilePath := filepath.Join(keyPath, file.Name())
		publicKey, err := os.ReadFile(keyFilePath)
		if err != nil {
			panic(fmt.Sprintf("读取RSA公钥文件 %s 失败: %v", keyFilePath, err))
		}

		clientID := strings.TrimSuffix(file.Name(), ".pub")
		clientPublicKeys[clientID] = string(publicKey)
	}

	return clientPublicKeys
}

func WithRSA() Option {
	return func(app *App) {
		if app.Config.App.KeyPath == "" {
			app.Logger.Panic("RSA密钥路径为空，无法加载公钥")
		}

		clientPublicKeys := loadClientKeysFromPath(app.Config.App.KeyPath)
		rsaMiddleware, err := middlewares.NewRSAClientAuthMiddleware(clientPublicKeys)
		if err != nil {
			app.Logger.Panic("创建RSA认证中间件失败", "error", err)
		}

		if app.Auth == nil {
			app.Logger.Panic("未初始化授权系统,请先调用WithJWT")
		}
		app.Auth.AddMiddleware("rsa", rsaMiddleware)
	}
}

// WithRBAC 添加RBAC中间件，需要在.env中配置 model_path与policy_path的路径
func WithRBAC() Option {
	return func(app *App) {
		app.Container.Register(func() *casbin.Enforcer {
			enforcer, err := casbin.NewEnforcer(app.Config.Casbin.ModelPath, app.Config.Casbin.PolicyPath)
			if err != nil {
				app.Logger.Panic("创建Casbin enforce失败", "error", err)
			}
			return enforcer
		})

		var enforcer *casbin.Enforcer
		err := app.Container.ResolveInstance(&enforcer)
		if err != nil {
			app.Logger.Panic("解析Casbin enforce失败", "error", err)
		}

		if app.Auth == nil {
			app.Logger.Panic("未初始化授权系统,请先调用WithJWT")
		}

		app.Auth.AddMiddleware("rbac", middlewares.RBACMiddleware(enforcer))
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
