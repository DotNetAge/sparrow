package config

type BadgerConfig struct {
	DataDir        string `mapstructure:"data_dir"`
	EventStoreDir  string `mapstructure:"es_dir"`
	ValueThreshold int64  `mapstructure:"value_threshold"`
	NumCompactors  int    `mapstructure:"num_compactors"`
	InMemory       bool   `mapstructure:"in_memory"`
	MemTableSize   int64  `mapstructure:"mem_table_size"`
	MaxTableSize   int64  `mapstructure:"max_table_size"`
}
