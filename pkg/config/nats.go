package config

type NATsConfig struct {
	URL        string `mapstructure:"url"`
	StreamName string `mapstructure:"stream_name"` // 用于事件总线的流
}
