package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

func Load(appName string) (*Config, error) {
	vp := viper.New()
	// 使用传入的应用名称作为配置文件名
	configName := appName
	if configName == "" {
		configName = "config" // 默认值，保持向后兼容
	}
	vp.SetConfigName(configName)
	// 添加配置搜索路径
	vp.AddConfigPath(".")
	vp.AddConfigPath("./configs")
	if appName != "" {
		vp.AddConfigPath(fmt.Sprintf("/etc/%s/", appName))
		vp.AddConfigPath(fmt.Sprintf("$HOME/.%s/", appName))
	}

	// 设置默认值
	SetDefaults(vp)

	// 从.env文件加载环境变量覆盖当前环境变量
	gotenv.Load()

	// 设置环境变量键名替换规则（显式声明）
	vp.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 读取环境变量
	vp.AutomaticEnv()

	// 尝试多种格式的配置文件
	configFormats := []string{"yaml", "yml", "json", "toml", "properties", "props", "prop"}
	for _, format := range configFormats {
		vp.SetConfigType(format)
		err := vp.ReadInConfig()
		if err == nil {
			// 成功读取配置文件，跳出循环
			break
		}
		// 如果是配置文件不存在的错误，继续尝试下一种格式
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := vp.Unmarshal(&config); err != nil {
		return nil, err
	}

	// 添加配置验证逻辑
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// 配置验证函数
func validateConfig(config *Config) error {
	// 检查必要的配置项
	if config.App.Name == "" {
		return fmt.Errorf("应用名称不能为空")
	}

	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("服务器端口必须在1到65535之间")
	}

	// 可以添加更多验证规则
	return nil
}
