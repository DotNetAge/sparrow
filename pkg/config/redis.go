package config

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	ESDB     int    `mapstructure:"es_db"` // 作为事件存储时的数据库
}
