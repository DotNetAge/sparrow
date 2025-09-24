package config

type NATsConfig struct {
	NATSURL    string `mapstructure:"nats_url"`
	StreamName string `mapstructure:"stream_name"` // 用于事件总线的流
  StoreStream string `mapstructure:"store_stream"` // 用于事件存储的流
  DurableName string `mapstructure:"durable_name"`
  MaxDeliver int `mapstructure:"max_deliver"`
  BucketName string `mapstructure:"bucket_name"` // 用于事件存储的桶
}
