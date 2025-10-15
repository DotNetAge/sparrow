package config

type BadgerConfig struct {
	DataDir        string `mapstructure:"data_dir"`
	EventStoreDir  string `mapstructure:"es_dir"`
	ValueThreshold int64  `mapstructure:"value_threshold"`
	NumCompactors  int    `mapstructure:"num_compactors"`
	InMemory       bool   `mapstructure:"in_memory"`
}
