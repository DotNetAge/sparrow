package tasks

import (
	"errors"
	"time"
)

// 重试相关的错误
var (
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
)

// CalculateNextRetry 计算下次重试时间，使用简单的指数退避算法
func CalculateNextRetry(retryCount int, baseDelay time.Duration) time.Time {
	// 基础延迟时间，避免立即重试
	if baseDelay == 0 {
		baseDelay = 1 * time.Second
	}

	// 简单的指数退避：2^retryCount * baseDelay
	// 限制最大延迟为30秒
	maxDelay := 30 * time.Second
	delay := baseDelay
	for i := 0; i < retryCount && delay < maxDelay; i++ {
		delay *= 2
	}
	if delay > maxDelay {
		delay = maxDelay
	}

	return time.Now().Add(delay)
}

// ShouldRetry 检查是否应该重试任务
func ShouldRetry(info *TaskInfo) bool {
	// 如果设置了最大重试次数且重试次数小于最大重试次数，则可以重试
	return info.MaxRetries > 0 && info.RetryCount < info.MaxRetries
}

// PrepareRetry 准备任务重试，更新任务信息
func PrepareRetry(info *TaskInfo, err error) (time.Time, error) {
	if !ShouldRetry(info) {
		return time.Time{}, ErrMaxRetriesExceeded
	}

	// 更新重试信息
	info.RetryCount++
	info.LastError = err.Error()
	info.Status = TaskStatusRetrying
	info.UpdatedAt = time.Now()

	// 计算下次重试时间
	nextRetryAt := CalculateNextRetry(info.RetryCount, 1*time.Second)
	info.NextRetryAt = nextRetryAt
	info.Schedule = nextRetryAt // 更新调度时间

	return nextRetryAt, nil
}
