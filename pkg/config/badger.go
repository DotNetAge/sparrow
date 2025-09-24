package config 

type BadgerConfig struct {
	DataDir        string `mapstructure:"data_dir"`
  EventStoreDir  string `mapstructure:"es_dir"`
  ViewDataDir     string `mapstructure:"vd_dir"`
	ValueThreshold int64  `mapstructure:"value_threshold"`
	NumCompactors  int    `mapstructure:"num_compactors"`
}