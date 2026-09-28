package config

import "time"

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	// ReadTimeout/WriteTimeout 零值时由 bootstrap 兜底为 10s（与历史硬编码行为一致）。
	// 含 LLM 同步调用的服务需放宽 WriteTimeout，否则长请求会被服务端强断（客户端表现为 EOF）。
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// CORSConfig 映射 CORS 相关的环境变量
type CORSConfig struct {
	// csv 标签告诉 mapstructure 将逗号分隔的字符串解析为 []string
	AllowOrigins     []string `mapstructure:"allow_origins,csv"`
	AllowMethods     []string `mapstructure:"allow_methods,csv"`
	AllowHeaders     []string `mapstructure:"allow_headers,csv"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAgeHours      int      `mapstructure:"max_age_hours"`
}
