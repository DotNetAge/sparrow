package tasks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateNextRetry(t *testing.T) {
	// 测试指数退避算法
	nextRetry := CalculateNextRetry(0, 1*time.Second)
	assert.NotZero(t, nextRetry)
	assert.True(t, nextRetry.After(time.Now()))

	// 测试多次重试的退避时间递增
	nextRetry1 := CalculateNextRetry(1, 1*time.Second)
	nextRetry2 := CalculateNextRetry(2, 1*time.Second)
	assert.True(t, nextRetry2.After(nextRetry1))
}

func TestShouldRetry(t *testing.T) {
	// 测试应该重试的情况
	taskInfo := &TaskInfo{
		RetryCount: 0,
		MaxRetries: 3,
	}
	assert.True(t, ShouldRetry(taskInfo))

	// 测试达到最大重试次数不应该重试
	taskInfo.RetryCount = 3
	assert.False(t, ShouldRetry(taskInfo))

	// 测试未设置最大重试次数不应该重试
	taskInfo = &TaskInfo{
		RetryCount: 0,
		MaxRetries: 0,
	}
	assert.False(t, ShouldRetry(taskInfo))
}

func TestPrepareRetry(t *testing.T) {
	taskInfo := &TaskInfo{
		RetryCount: 0,
		MaxRetries: 3,
	}
	err := assert.AnError

	PrepareRetry(taskInfo, err)

	// 验证重试计数增加
	assert.Equal(t, 1, taskInfo.RetryCount)
	// 验证最后错误信息设置
	assert.Equal(t, err.Error(), taskInfo.LastError)
	// 验证下次重试时间设置
	assert.NotZero(t, taskInfo.NextRetryAt)
	assert.True(t, taskInfo.NextRetryAt.After(time.Now()))
}

func TestTaskInfoWithRetry(t *testing.T) {
	// 测试TaskInfo的重试字段初始化和更新
	taskInfo := &TaskInfo{
		ID:         "test-task",
		Type:       "test-type",
		MaxRetries: 3,
	}

	// 初始状态验证
	assert.Equal(t, 0, taskInfo.RetryCount)
	assert.Equal(t, "", taskInfo.LastError)
	assert.Zero(t, taskInfo.NextRetryAt)

	// 模拟重试准备
	err := assert.AnError
	PrepareRetry(taskInfo, err)

	// 重试后的状态验证
	assert.Equal(t, 1, taskInfo.RetryCount)
	assert.Equal(t, err.Error(), taskInfo.LastError)
	assert.NotZero(t, taskInfo.NextRetryAt)
}