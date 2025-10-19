package config

type LogConfig struct {
	Mode     string `mapstructure:"mode"`
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	Output   string `mapstructure:"output"`
	Filename string `mapstructure:"filename"`
}
