package config

// BoltConfig bbolt 数据库配置
type BoltConfig struct {
	DataDir string `mapstructure:"data_dir"`
}
