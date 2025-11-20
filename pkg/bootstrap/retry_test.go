package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/tasks"
)

func TestRetryConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		config      *RetryConfig
		expectRetry bool
	}{
		{
			name:        "nil config - no retry",
			config:      nil,
			expectRetry: false,
		},
		{
			name: "disabled retry - no retry",
			config: &RetryConfig{
				Enabled: false,
			},
			expectRetry: false,
		},
		{
			name: "enabled retry - with retry",
			config: &RetryConfig{
				Enabled:       true,
				MaxRetries:    3,
				InitialBackoff: 1 * time.Second,
				MaxBackoff:    10 * time.Second,
			},
			expectRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				retryConfig: tt.config,
			}

			// 创建一个简单的任务构建器
			builder := tasks.NewTaskBuilder().
				WithID("test-task").
				WithHandler(func(ctx context.Context) error {
					return nil
				})

			// 使用 buildTaskWithRetry 方法构建任务
			task := app.buildTaskWithRetry(builder)

			// 验证任务是否包含重试能力
			if tt.expectRetry {
				if _, ok := task.(tasks.RetryableTask); !ok {
					t.Errorf("expected retryable task, got %T", task)
				}
			} else {
				if _, ok := task.(tasks.RetryableTask); ok {
					t.Errorf("expected non-retryable task, got retryable task")
				}
			}
		})
	}
}

func TestRetryBackoffStrategies(t *testing.T) {
	tests := []struct {
		name            string
		config          *RetryConfig
		expectedBackoff string
	}{
		{
			name: "fixed backoff",
			config: &RetryConfig{
				Enabled:           true,
				MaxRetries:        3,
				InitialBackoff:    2 * time.Second,
				MaxBackoff:        2 * time.Second,
				BackoffMultiplier: 1.0,
			},
			expectedBackoff: "fixed",
		},
		{
			name: "linear backoff",
			config: &RetryConfig{
				Enabled:           true,
				MaxRetries:        5,
				InitialBackoff:    1 * time.Second,
				MaxBackoff:        5 * time.Second,
				BackoffMultiplier: 1.0,
			},
			expectedBackoff: "linear",
		},
		{
			name: "exponential backoff",
			config: &RetryConfig{
				Enabled:           true,
				MaxRetries:        5,
				InitialBackoff:    1 * time.Second,
				MaxBackoff:        60 * time.Second,
				BackoffMultiplier: 2.0,
			},
			expectedBackoff: "exponential",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				retryConfig: tt.config,
			}

			// 创建任务构建器
			builder := tasks.NewTaskBuilder().
				WithID("test-retry-task").
				WithHandler(func(ctx context.Context) error {
					return nil
				})

			task := app.buildTaskWithRetry(builder)

			// 验证任务类型
			retryableTask, ok := task.(tasks.RetryableTask)
			if !ok {
				t.Fatalf("expected retryable task, got %T", task)
			}

			// 验证重试策略
			policy := retryableTask.GetRetryPolicy()
			if policy.MaxRetries != tt.config.MaxRetries {
				t.Errorf("expected max retries %d, got %d", tt.config.MaxRetries, policy.MaxRetries)
			}

			// 验证退避策略
			switch tt.expectedBackoff {
			case "fixed":
				if policy.InitialBackoff != tt.config.InitialBackoff || policy.MaxBackoff != tt.config.MaxBackoff {
					t.Errorf("fixed backoff: expected initial=%v, max=%v, got initial=%v, max=%v",
						tt.config.InitialBackoff, tt.config.MaxBackoff, policy.InitialBackoff, policy.MaxBackoff)
				}
			case "linear":
				if policy.BackoffMultiplier != 1.0 {
					t.Errorf("linear backoff: expected multiplier 1.0, got %f", policy.BackoffMultiplier)
				}
			case "exponential":
				if policy.BackoffMultiplier != 2.0 {
					t.Errorf("exponential backoff: expected multiplier 2.0, got %f", policy.BackoffMultiplier)
				}
			}

			t.Logf("Retry task created with %d max retries and %s backoff strategy", 
				policy.MaxRetries, tt.expectedBackoff)
		})
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if !config.Enabled {
		t.Error("default retry config should be enabled")
	}

	if config.MaxRetries != 3 {
		t.Errorf("expected default max retries 3, got %d", config.MaxRetries)
	}

	if config.InitialBackoff != 1*time.Second {
		t.Errorf("expected default initial backoff 1s, got %v", config.InitialBackoff)
	}

	if config.MaxBackoff != 30*time.Second {
		t.Errorf("expected default max backoff 30s, got %v", config.MaxBackoff)
	}

	if config.BackoffMultiplier != 2.0 {
		t.Errorf("expected default backoff multiplier 2.0, got %f", config.BackoffMultiplier)
	}
}

func TestWithRetryOption(t *testing.T) {
	app := NewApp(
		Tasks(),
		WithRetry(
			WithMaxRetries(5),
			WithInitialBackoff(2*time.Second),
		),
	)

	if app.retryConfig == nil {
		t.Fatal("retry config should not be nil")
	}

	if !app.retryConfig.Enabled {
		t.Error("retry should be enabled")
	}

	if app.retryConfig.MaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", app.retryConfig.MaxRetries)
	}

	if app.retryConfig.InitialBackoff != 2*time.Second {
		t.Errorf("expected initial backoff 2s, got %v", app.retryConfig.InitialBackoff)
	}
}