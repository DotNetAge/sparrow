package config

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Secret  string `mapstructure:"secret"`
	KeyPath string `mapstructure:"key_path"`
}
