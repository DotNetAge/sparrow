package config

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// CORSConfig 映射 CORS 相关的环境变量
type CORSConfig struct {
	// csv 标签告诉 mapstructure 将逗号分隔的字符串解析为 []string
	AllowOrigins     []string `mapstructure:"CORS_ALLOW_ORIGINS,csv"`
	AllowMethods     []string `mapstructure:"CORS_ALLOW_METHODS,csv"`
	AllowHeaders     []string `mapstructure:"CORS_ALLOW_HEADERS,csv"`
	AllowCredentials bool     `mapstructure:"CORS_ALLOW_CREDENTIALS"`
	MaxAgeHours      int      `mapstructure:"CORS_MAX_AGE_HOURS"`
}
