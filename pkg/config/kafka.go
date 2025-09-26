package config

// KafkaConfig 定义Kafka连接配置
type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	GroupID string   `mapstructure:"group_id"`
	Topic   string   `mapstructure:"topic"`
}
