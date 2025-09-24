package config

import (
	"github.com/spf13/viper"
)

func Load() (*Config, error) {
	vp := viper.New()
	vp.SetConfigName("config")
	vp.SetConfigType("yaml")
	vp.AddConfigPath(".")
	vp.AddConfigPath("./configs")

	// 设置默认值
	SetDefaults(vp)

	// 读取环境变量
	vp.AutomaticEnv()

	if err := vp.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件不存在，使用默认值
		} else {
			return nil, err
		}
	}

	var config Config
	if err := vp.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
