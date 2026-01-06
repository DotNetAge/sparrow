package config

type CasbinConfig struct {
	ModelPath  string `mapstructure:"model_path"`
	PolicyPath string `mapstructure:"policy_path"`
}
