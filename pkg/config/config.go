package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	App    AppConfig    `mapstructure:"app"`
	Server ServerConfig `mapstructure:"server"`
	Log    LogConfig    `mapstructure:"log"`
	NATS   NATsConfig   `mapstructure:"nats"`
	SQL    SQLConfig    `mapstructure:"sql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Badger BadgerConfig `mapstructure:"badger"`
}

func SetDefaults(viper *viper.Viper) {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "60s")

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output", "stdout")

	// Nats 配置
	viper.SetDefault("nats.nats_url", "nats://localhost:4222")
	viper.SetDefault("nats.stream_name", "events")
	viper.SetDefault("nats.store_stream", "events_store")
	viper.SetDefault("nats.bucket_name", "events_bucket")
	viper.SetDefault("nats.durable_name", "default")
	viper.SetDefault("nats.max_deliver", 10)

	// Redis 配置
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.es_db", 1) // 作为事件存储时的数据库

	// Badger 配置
	viper.SetDefault("badger.data_dir", "./badger")              // 作为仓库存储的路径
	viper.SetDefault("badger.es_dir", "./badger/es")             // 作为事件存储时的路径
	viper.SetDefault("badger.value_threshold", int64(1024*1024)) // 1MB
	viper.SetDefault("badger.num_compactors", 1)

	// SQL 配置
	viper.SetDefault("sql.driver", "postgres")
	viper.SetDefault("sql.host", "localhost")
	viper.SetDefault("sql.port", "5432")
	viper.SetDefault("sql.user", "postgres")
	viper.SetDefault("sql.password", "postgres")
	viper.SetDefault("sql.dbname", "postgres")
	viper.SetDefault("sql.es_dbname", "postgres") // 作为事件存储时的数据库名
}
