package tasks

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRetryPolicyValidation(t *testing.T) {
	tests := []struct {
		name    string
		policy  *RetryPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: &RetryPolicy{
				MaxRetries:        3,
				InitialBackoff:    time.Second,
				MaxBackoff:        time.Minute,
				BackoffMultiplier: 2.0,
				RetryableErrorFunc: func(err error) bool { return true },
			},
			wantErr: false,
		},
		{
			name: "zero max retries",
			policy: &RetryPolicy{
				MaxRetries:        0,
				InitialBackoff:    time.Second,
				MaxBackoff:        time.Minute,
			},
			wantErr: true,
		},
		{
			name: "negative initial backoff",
			policy: &RetryPolicy{
				MaxRetries:        3,
				InitialBackoff:    -time.Second,
				MaxBackoff:        time.Minute,
			},
			wantErr: true,
		},
		{
			name: "max backoff less than initial",
			policy: &RetryPolicy{
				MaxRetries:        3,
				InitialBackoff:    time.Minute,
				MaxBackoff:        time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryPolicy.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBackoffCalculator(t *testing.T) {
	tests := []struct {
		name         string
		policy       *RetryPolicy
		attemptCount int
		expectedMin  time.Duration
		expectedMax  time.Duration
	}{
		{
			name: "fixed backoff",
			policy: &RetryPolicy{
				InitialBackoff:  time.Second,
				MaxBackoff:      time.Second,
			},
			attemptCount: 1,
			expectedMin:  time.Second,
			expectedMax:  time.Second,
		},
		{
			name: "linear backoff",
			policy: &RetryPolicy{
				InitialBackoff:    time.Second,
				MaxBackoff:        10 * time.Second,
				BackoffMultiplier: 1.0,
			},
			attemptCount: 3,
			expectedMin:  3 * time.Second,
			expectedMax:  3 * time.Second,
		},
		{
			name: "exponential backoff",
			policy: &RetryPolicy{
				InitialBackoff:    time.Second,
				MaxBackoff:        10 * time.Second,
				BackoffMultiplier: 2.0,
			},
			attemptCount: 3,
			expectedMin:  4 * time.Second,
			expectedMax:  8 * time.Second, // 考虑抖动
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := tt.policy.CalculateBackoff(tt.attemptCount, BackoffStrategyFixed)
			if tt.name == "linear backoff" {
				delay = tt.policy.CalculateBackoff(tt.attemptCount, BackoffStrategyLinear)
			} else if tt.name == "exponential backoff" {
				delay = tt.policy.CalculateBackoff(tt.attemptCount, BackoffStrategyExponential)
			}
			
			if delay < tt.expectedMin || delay > tt.expectedMax {
				t.Errorf("CalculateBackoff() = %v, want between %v and %v", delay, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestRetryableTask(t *testing.T) {
	task := NewTaskBuilder().
		WithType("retryable").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			return nil
		}).
		WithRetry(3).
		WithFixedBackoff(100 * time.Millisecond).
		Build()
	
	// 验证任务实现了RetryableTask接口
	if _, ok := task.(RetryableTask); !ok {
		t.Error("Expected task to implement RetryableTask interface")
	}
	
	retryableTask := task.(RetryableTask)
	
	// 验证重试策略
	retrievedPolicy := retryableTask.GetRetryPolicy()
	if retrievedPolicy.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", retrievedPolicy.MaxRetries)
	}
	
	// 验证重试信息初始化
	retryInfo := retryableTask.GetRetryInfo()
	if retryInfo == nil {
		t.Error("Expected retry info to be initialized")
	}
	
	if retryInfo.CurrentRetry != 0 {
		t.Errorf("Expected CurrentRetry 0, got %d", retryInfo.CurrentRetry)
	}
}

func TestRetryableTaskBuilder(t *testing.T) {
	task := NewTaskBuilder().
		WithType("retryable").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			return nil
		}).
		WithRetry(3).
		WithExponentialBackoff(time.Second).
		WithMaxDelay(time.Minute).
		Build()
	
	// 验证任务实现了RetryableTask接口
	if _, ok := task.(RetryableTask); !ok {
		t.Error("Expected task to implement RetryableTask interface")
	}
	
	retryableTask := task.(RetryableTask)
	
	// 验证重试策略
	retrievedPolicy := retryableTask.GetRetryPolicy()
	if retrievedPolicy.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", retrievedPolicy.MaxRetries)
	}
	
	// 验证重试信息初始化
	retryInfo := retryableTask.GetRetryInfo()
	if retryInfo == nil {
		t.Error("Expected retry info to be initialized")
	}
	
	if retryInfo.CurrentRetry != 0 {
		t.Errorf("Expected CurrentRetry 0, got %d", retryInfo.CurrentRetry)
	}
}

func TestTaskRetryExecution(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close(nil)
	
	var attemptCount int
	var mu sync.Mutex
	
	task := NewTaskBuilder().
		WithType("test_retry_task").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			mu.Lock()
			attemptCount++
			mu.Unlock()
			
			// 前两次失败，第三次成功
			if attemptCount < 3 {
				return errors.New("simulated failure")
			}
			return nil
		}).
		WithRetry(3).
		WithFixedBackoff(100 * time.Millisecond).
		Build()
	
	err := scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("Failed to schedule retryable task: %v", err)
	}
	
	err = scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	
	// 等待重试完成
	time.Sleep(1 * time.Second)
	
	mu.Lock()
	finalCount := attemptCount
	mu.Unlock()
	
	// 验证重试次数
	if finalCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", finalCount)
	}
	
	// 验证最终状态
	status, err := scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	
	if status != TaskStatusCompleted {
		t.Errorf("Expected task status to be completed, got %v", status)
	}
}

func TestTaskMaxRetriesExceeded(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close(nil)
	
	var attemptCount int
	var mu sync.Mutex
	
	task := NewTaskBuilder().
		WithType("always_failing_task").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			mu.Lock()
			attemptCount++
			mu.Unlock()
			return errors.New("always fails")
		}).
		WithRetry(2).
		WithFixedBackoff(50 * time.Millisecond).
		Build()
	
	err := scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("Failed to schedule retryable task: %v", err)
	}
	
	err = scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	
	// 等待重试完成
	time.Sleep(500 * time.Millisecond)
	
	mu.Lock()
	finalCount := attemptCount
	mu.Unlock()
	
	// 验证重试次数（初始执行 + 2次重试）
	if finalCount != 3 {
		t.Errorf("Expected 3 attempts (1 initial + 2 retries), got %d", finalCount)
	}
	
	// 验证任务最终状态
	status, err := scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	
	if status != TaskStatusDeadLetter {
		t.Errorf("Expected task status to be dead_letter, got %v", status)
	}
}