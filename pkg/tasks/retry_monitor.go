package tasks

import (
	"sync"
	"time"
)

// RetryMonitor 简化的重试监控器
type RetryMonitor struct {
	mu                sync.RWMutex
	totalRetries      int64
	successfulRetries int64
	failedRetries     int64
	deadLetterCount   int64
	averageRetryTime  time.Duration
	lastRetryAt       time.Time
}

// NewRetryMonitor 创建重试监控器
func NewRetryMonitor() *RetryMonitor {
	return &RetryMonitor{}
}

// RecordRetry 记录重试
func (m *RetryMonitor) RecordRetry(success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRetries++
	m.lastRetryAt = time.Now()

	if success {
		m.successfulRetries++
	} else {
		m.failedRetries++
	}

	// 更新平均重试时间
	if m.totalRetries == 1 {
		m.averageRetryTime = duration
	} else {
		m.averageRetryTime = time.Duration(
			(int64(m.averageRetryTime)*(m.totalRetries-1) + int64(duration)) / m.totalRetries,
		)
	}
}

// RecordDeadLetter 记录死信任务
func (m *RetryMonitor) RecordDeadLetter() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deadLetterCount++
}

// GetStats 获取重试统计
func (m *RetryMonitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_retries":       m.totalRetries,
		"successful_retries":  m.successfulRetries,
		"failed_retries":      m.failedRetries,
		"dead_letter_count":   m.deadLetterCount,
		"average_retry_time":  m.averageRetryTime.String(),
		"last_retry_at":       m.lastRetryAt,
	}
}

// Reset 重置统计信息
func (m *RetryMonitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRetries = 0
	m.successfulRetries = 0
	m.failedRetries = 0
	m.deadLetterCount = 0
	m.averageRetryTime = 0
	m.lastRetryAt = time.Time{}
}