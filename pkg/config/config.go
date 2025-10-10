package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	CORS     CORSConfig     `mapstructure:"cors"`
	Log      LogConfig      `mapstructure:"log"`
	NATS     NATsConfig     `mapstructure:"nats"`
	SQL      SQLConfig      `mapstructure:"sql"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Badger   BadgerConfig   `mapstructure:"badger"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
}

func SetDefaults(viper *viper.Viper) {
	// 应用配置默认值
	viper.SetDefault("app.name", "sparrow")
	viper.SetDefault("app.version", "1.0.0")

	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	// viper.SetDefault("server.read_timeout", "30s")
	// viper.SetDefault("server.write_timeout", "30s")
	// viper.SetDefault("server.idle_timeout", "60s")

	// CORS 配置
	viper.SetDefault("cors.allow_origins", []string{"*"})
	viper.SetDefault("cors.allow_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("cors.allow_headers", []string{"Origin", "Content-Type", "Accept"})
	viper.SetDefault("cors.allow_credentials", true)
	viper.SetDefault("cors.max_age_hours", 1)

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

	// RabbitMQ 配置
	viper.SetDefault("rabbitmq.host", "localhost")
	viper.SetDefault("rabbitmq.port", 5672)
	viper.SetDefault("rabbitmq.username", "guest")
	viper.SetDefault("rabbitmq.password", "guest")
	viper.SetDefault("rabbitmq.vhost", "/")
	viper.SetDefault("rabbitmq.exchange", "events")
}
