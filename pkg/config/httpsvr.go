package config

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
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
