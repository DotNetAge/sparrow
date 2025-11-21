package config

import "time"

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries        int           // 最大重试次数，默认3次
	InitialBackoff    time.Duration // 初始退避时间，默认1秒
	MaxBackoff        time.Duration // 最大退避时间，默认30秒
	BackoffMultiplier float64       // 退避倍数，默认2.0（指数退避）
	Enabled           bool          // 是否启用重试，默认true
}
