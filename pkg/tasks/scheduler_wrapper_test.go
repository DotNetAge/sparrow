package tasks

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/stretchr/testify/assert"
)

// 使用包中已定义的任务状态常量，无需重复定义

// TestSchedulerWrapper_ImmediateTask 测试立即执行任务
func TestSchedulerWrapper_ImmediateTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建内存调度器和包装器
	scheduler := NewMemoryTaskScheduler()
	// 启动调度器
	assert.NoError(t, scheduler.Start(ctx))
	defer scheduler.Stop()
	
	// 创建包装器
	wrapper := NewSchedulerWrapper(scheduler, nil)

	// 用于验证任务执行的通道
	done := make(chan struct{})

	// 提交立即执行的任务
	taskID, err := wrapper.RunTask(func(ctx context.Context) error {
		close(done)
		return nil
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 等待任务执行完成
	select {
	case <-done:
		// 任务成功执行
	case <-time.After(2 * time.Second):
		t.Fatal("任务未能在预期时间内执行")
	}

	// 验证任务状态
	status, err := scheduler.GetTaskStatus(taskID)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusCompleted, status)
}

// TestSchedulerWrapper_ScheduledTask 测试定时执行任务
func TestSchedulerWrapper_ScheduledTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建内存调度器和包装器
	scheduler := NewMemoryTaskScheduler()
	// 启动调度器
	assert.NoError(t, scheduler.Start(ctx))
	defer scheduler.Stop()
	
	// 创建包装器
	wrapper := NewSchedulerWrapper(scheduler, nil)

	// 用于验证任务执行的通道
	done := make(chan struct{})

	// 提交300毫秒后执行的任务
	executeTime := time.Now().Add(300 * time.Millisecond)
	taskID, err := wrapper.RunTaskAt(executeTime, func(ctx context.Context) error {
		close(done)
		return nil
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 验证任务在执行前处于等待状态
	time.Sleep(100 * time.Millisecond)
	status, err := scheduler.GetTaskStatus(taskID)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusWaiting, status)

	// 等待任务执行完成
	select {
	case <-done:
		// 任务成功执行
	case <-time.After(1 * time.Second):
		t.Fatal("任务未能在预期时间内执行")
	}

	// 验证任务状态
	status, err = scheduler.GetTaskStatus(taskID)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusCompleted, status)
}

// TestSchedulerWrapper_RecurringTask 测试周期性执行任务
func TestSchedulerWrapper_RecurringTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建内存调度器和包装器
	scheduler := NewMemoryTaskScheduler()
	// 启动调度器
	assert.NoError(t, scheduler.Start(ctx))
	defer scheduler.Stop()
	
	// 创建包装器
	wrapper := NewSchedulerWrapper(scheduler, nil)

	// 用于计数任务执行次数
	var counter int
	var mu sync.Mutex

	// 提交每200毫秒执行一次的周期性任务
	taskID, err := wrapper.RunTaskRecurring(200*time.Millisecond, func(ctx context.Context) error {
		mu.Lock()
		counter++
		mu.Unlock()
		return nil
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 等待任务执行几次
	time.Sleep(700 * time.Millisecond)

	// 取消任务
	assert.NoError(t, scheduler.Cancel(taskID))

	// 验证任务至少执行了3次
	mu.Lock()
	assert.GreaterOrEqual(t, counter, 3)
	mu.Unlock()
}

// TestSchedulerWrapper_CancelTask 测试取消任务
func TestSchedulerWrapper_CancelTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建内存调度器和包装器
	scheduler := NewMemoryTaskScheduler()
	// 启动调度器
	assert.NoError(t, scheduler.Start(ctx))
	defer scheduler.Stop()
	
	// 创建包装器
	wrapper := NewSchedulerWrapper(scheduler, nil)

	// 用于验证任务是否被执行（不应该被执行）
	var executed bool

	// 提交1秒后执行的任务
	taskID, err := wrapper.RunTaskAt(time.Now().Add(1*time.Second), func(ctx context.Context) error {
		executed = true
		return nil
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 取消任务
	assert.NoError(t, scheduler.Cancel(taskID))

	// 等待足够长的时间，确保任务不会执行
	time.Sleep(1500 * time.Millisecond)

	// 验证任务未执行
	assert.False(t, executed)

	// 验证任务状态
	status, err := scheduler.GetTaskStatus(taskID)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusCancelled, status)
}

// TestSchedulerWrapper_FailedTaskWithRetry 测试任务失败和重试
func TestSchedulerWrapper_FailedTaskWithRetry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建重试配置
	retryConfig := &config.RetryConfig{
		Enabled:           true,
		MaxRetries:        2,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 1.0, // 固定退避
	}

	// 创建内存调度器和包装器
	scheduler := NewMemoryTaskScheduler()
	// 启动调度器
	assert.NoError(t, scheduler.Start(ctx))
	defer scheduler.Stop()
	
	// 创建包装器
	wrapper := NewSchedulerWrapper(scheduler, retryConfig)

	// 用于计数任务执行次数
	var counter int
	var mu sync.Mutex

	// 提交一个会失败的任务
	taskID, err := wrapper.RunTask(func(ctx context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		counter++
		return assert.AnError // 任务执行失败
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 等待任务开始重试
	time.Sleep(1 * time.Second)

	// 验证任务至少执行了1次
	mu.Lock()
	assert.GreaterOrEqual(t, counter, 1)
	mu.Unlock()

	// 验证任务状态（可能处于retrying状态）
	status, err := scheduler.GetTaskStatus(taskID)
	assert.NoError(t, err)
	assert.True(t, status == TaskStatusFailed || status == TaskStatusRetrying, "任务状态应为failed或retrying")
}

// TestSchedulerWrapper_HybridScheduler 测试混合调度器的任务类型和执行策略
func TestSchedulerWrapper_HybridScheduler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建混合调度器（启用顺序执行策略）
	hybridScheduler := NewHybridTaskScheduler(
		WithHybridWorkerCount(3, 1), // 3个并发工作协程，1个顺序工作协程
	)

	// 注册任务类型的执行策略
	assert.NoError(t, hybridScheduler.RegisterTaskPolicy("sequential-task", PolicySequential))
	assert.NoError(t, hybridScheduler.RegisterTaskPolicy("concurrent-task", PolicyConcurrent))

	// 启动调度器
	assert.NoError(t, hybridScheduler.Start(ctx))
	defer hybridScheduler.Stop()

	// 创建包装器
	wrapper := NewSchedulerWrapper(hybridScheduler, nil)

	// 用于验证顺序执行
	sequentialDone := make(chan struct{})
	var sequentialOrder []int
	var seqMu sync.Mutex

	// 提交两个顺序执行的任务
	for i := 1; i <= 2; i++ {
		index := i
		_, err := wrapper.RunTypedTask("sequential-task", func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond) // 模拟工作
			seqMu.Lock()
			sequentialOrder = append(sequentialOrder, index)
			seqMu.Unlock()
			if index == 2 {
				close(sequentialDone)
			}
			return nil
		})
		assert.NoError(t, err)
	}

	// 等待顺序任务完成
	select {
	case <-sequentialDone:
		// 顺序任务成功执行
	case <-time.After(500 * time.Millisecond):
		t.Fatal("顺序任务未能在预期时间内执行完成")
	}

	// 验证顺序任务的执行顺序
	seqMu.Lock()
	assert.Equal(t, []int{1, 2}, sequentialOrder)
	seqMu.Unlock()

	// 用于验证并发执行
	concurrentStart := make(chan struct{})
	concurrentDone := make(chan struct{})
	var concurrentCount int
	var concMu sync.Mutex

	// 提交3个并发执行的任务
	for i := 0; i < 3; i++ {
		_, err := wrapper.RunTypedTask("concurrent-task", func(ctx context.Context) error {
			<-concurrentStart // 等待信号，确保同时开始
			time.Sleep(100 * time.Millisecond) // 模拟工作
			concMu.Lock()
			concurrentCount++
			if concurrentCount == 3 {
				close(concurrentDone)
			}
			concMu.Unlock()
			return nil
		})
		assert.NoError(t, err)
	}

	// 释放并发任务
	close(concurrentStart)

	// 等待并发任务完成
	select {
	case <-concurrentDone:
		// 并发任务成功执行
	case <-time.After(300 * time.Millisecond):
		t.Fatal("并发任务未能在预期时间内执行完成")
	}

	// 验证所有并发任务都已执行
	concMu.Lock()
	assert.Equal(t, 3, concurrentCount)
	concMu.Unlock()
}