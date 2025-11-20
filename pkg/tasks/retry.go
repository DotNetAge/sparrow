package tasks

import (
	"errors"
	"math"
	"math/rand"
	"time"
)// RetryPolicy 重试策略配置
type RetryPolicy struct {
	MaxRetries        int           `json:"max_retries"`        // 最大重试次数，0表示不重试
	InitialBackoff     time.Duration `json:"initial_backoff"`     // 初始退避时间
	MaxBackoff         time.Duration `json:"max_backoff"`         // 最大退避时间
	BackoffMultiplier  float64       `json:"backoff_multiplier"`  // 退避倍数
	EnableJitter       bool          `json:"enable_jitter"`       // 是否启用抖动
	RetryableErrorFunc func(error) bool `json:"-"`               // 可重试的错误判断函数
}

// BackoffStrategy 退避策略类型
type BackoffStrategy string

const (
	BackoffStrategyFixed       BackoffStrategy = "fixed"
	BackoffStrategyLinear      BackoffStrategy = "linear"
	BackoffStrategyExponential BackoffStrategy = "exponential"
)

// DefaultRetryPolicy 默认重试策略
var DefaultRetryPolicy = &RetryPolicy{
	MaxRetries:        3,
	InitialBackoff:    1 * time.Second,
	MaxBackoff:        60 * time.Second,
	BackoffMultiplier: 2.0,
	EnableJitter:      true,
	RetryableErrorFunc: func(err error) bool { return err != nil },
}

// TaskRetryInfo 重试信息
type TaskRetryInfo struct {
	CurrentRetry  int           `json:"current_retry"`
	LastError     error         `json:"last_error"`
	LastRetryAt   time.Time     `json:"last_retry_at"`
	NextRetryAt   time.Time     `json:"next_retry_at"`
	RetryHistory  []RetryAttempt `json:"retry_history"`
}

// RetryAttempt 重试尝试记录
type RetryAttempt struct {
	Attempt   int           `json:"attempt"`
	Error     string        `json:"error"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// RetryableTask 可重试任务接口
type RetryableTask interface {
	Task
	GetRetryPolicy() *RetryPolicy
	GetRetryInfo() *TaskRetryInfo
	SetRetryInfo(*TaskRetryInfo)
}

// Validate 验证重试策略配置
func (p *RetryPolicy) Validate() error {
	if p.MaxRetries < 0 {
		return errors.New("MaxRetries不能小于0")
	}
	if p.InitialBackoff < 0 {
		return errors.New("InitialBackoff不能为负数")
	}
	if p.MaxBackoff < 0 {
		return errors.New("MaxBackoff不能为负数")
	}
	if p.BackoffMultiplier <= 0 {
		return errors.New("BackoffMultiplier必须大于0")
	}
	if p.InitialBackoff > 0 && p.MaxBackoff > 0 && p.InitialBackoff > p.MaxBackoff {
		return errors.New("InitialBackoff不能大于MaxBackoff")
	}
	return nil
}

// ShouldRetry 判断是否应该重试
func (p *RetryPolicy) ShouldRetry(attempt int, err error) bool {
	if p == nil || attempt > p.MaxRetries {
		return false
	}
	if p.RetryableErrorFunc != nil && !p.RetryableErrorFunc(err) {
		return false
	}
	return true
}

// CalculateBackoff 计算退避时间
func (p *RetryPolicy) CalculateBackoff(attempt int, strategy BackoffStrategy) time.Duration {
	if p == nil {
		return 0
	}

	var baseDelay time.Duration
	switch strategy {
	case BackoffStrategyFixed:
		baseDelay = p.InitialBackoff
	case BackoffStrategyLinear:
		baseDelay = time.Duration(float64(p.InitialBackoff) * p.BackoffMultiplier * float64(attempt))
	case BackoffStrategyExponential:
		expMultiplier := math.Pow(p.BackoffMultiplier, float64(attempt-1))
		baseDelay = time.Duration(float64(p.InitialBackoff) * expMultiplier)
	default:
		baseDelay = p.InitialBackoff
	}

	// 应用最大退避时间限制
	if p.MaxBackoff > 0 && baseDelay > p.MaxBackoff {
		baseDelay = p.MaxBackoff
	}

	// 应用抖动
	if p.EnableJitter {
		jitter := time.Duration(rand.Float64() * float64(baseDelay) * 0.1)
		baseDelay += jitter
	}

	return baseDelay
}

// NewRetryInfo 创建新的重试信息
func NewRetryInfo() *TaskRetryInfo {
	return &TaskRetryInfo{
		CurrentRetry: 0,
		RetryHistory: make([]RetryAttempt, 0),
	}
}

// AddRetryAttempt 添加重试尝试记录
func (info *TaskRetryInfo) AddRetryAttempt(attempt int, err error, duration time.Duration) {
	info.CurrentRetry = attempt
	info.LastError = err
	info.LastRetryAt = time.Now()

	info.RetryHistory = append(info.RetryHistory, RetryAttempt{
		Attempt:   attempt,
		Error:     err.Error(),
		Timestamp: time.Now(),
		Duration:  duration,
	})

	// 限制历史记录数量，避免内存泄漏
	if len(info.RetryHistory) > 50 {
		info.RetryHistory = info.RetryHistory[1:]
	}
}

// GetAverageRetryDelay 获取平均重试延迟
func (info *TaskRetryInfo) GetAverageRetryDelay() time.Duration {
	if len(info.RetryHistory) == 0 {
		return 0
	}

	var totalDelay time.Duration
	for _, attempt := range info.RetryHistory {
		totalDelay += attempt.Duration
	}

	return totalDelay / time.Duration(len(info.RetryHistory))
}