package config

type SQLConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Dbname   string `mapstructure:"dbname"`
	ESDbname string `mapstructure:"es_dbname"` // 作为事件存储时的数据库名
}

func (c *SQLConfig) Dsn() string {
	return c.Driver + ":" + c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + c.Port + ")/" + c.Dbname + "?charset=utf8mb4&parseTime=True&loc=Local"
}

func (c *SQLConfig) ESDsn() string {
	return c.Driver + ":" + c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + c.Port + ")/" + c.ESDbname + "?charset=utf8mb4&parseTime=True&loc=Local"
}
