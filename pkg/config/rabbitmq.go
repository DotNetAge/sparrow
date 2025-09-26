package config

// RabbitMQConfig 定义RabbitMQ连接配置
type RabbitMQConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	VHost    string `mapstructure:"vhost"`
	Exchange string `mapstructure:"exchange"`
}
