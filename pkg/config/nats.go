package config

type NATsConfig struct {
	URL        string `mapstructure:"url"`
	StreamName string `mapstructure:"stream_name"` // 用于事件总线的流(兼容使用标准事件流)
	MaxAge     int    `mapstructure:"max_age"`     // 事件流最大保存时间，单位秒，默认0表示永久保存
}
