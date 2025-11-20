package tasks

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestDeadlockOnClose 测试优雅关闭时的死锁问题
func TestDeadlockOnClose(t *testing.T) {
	scheduler := NewMemoryTaskScheduler(
		WithWorkerCount(2),
		WithMaxConcurrentTasks(2),
	)

	err := scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var mu sync.Mutex
	completedTasks := 0

	// 创建多个长时间运行的任务，其中一些不响应取消
	for i := 0; i < 5; i++ {
		taskIndex := i
		task := NewTaskBuilder().
			WithType("long-running").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				if taskIndex < 2 {
					// 前两个任务正确响应取消
					select {
					case <-time.After(5 * time.Second):
						mu.Lock()
						completedTasks++
						mu.Unlock()
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				} else {
					// 后面的任务不响应取消，只通过超时完成
					time.Sleep(3 * time.Second)
					mu.Lock()
						completedTasks++
					mu.Unlock()
					return nil
				}
			}).
			Build()

		err := scheduler.Schedule(task)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	}

	// 等待任务开始执行
	time.Sleep(100 * time.Millisecond)

	// 尝试关闭调度器，设置超时以检测死锁
	done := make(chan error, 1)
	go func() {
		done <- scheduler.Close(context.Background())
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error during close: %v", err)
		}
		t.Log("调度器关闭成功")
	case <-time.After(10 * time.Second):
		t.Error("调度器关闭超时，可能存在死锁问题")
	}

	mu.Lock()
	t.Logf("完成的任务数: %d", completedTasks)
	mu.Unlock()
}

// TestExecutionModes 测试当前只支持并发执行模式
func TestExecutionModes(t *testing.T) {
	scheduler := NewMemoryTaskScheduler(
		WithWorkerCount(3),
		WithMaxConcurrentTasks(3),
	)

	err := scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer scheduler.Close(context.Background())

	var mu sync.Mutex
	runningTasks := 0
	maxConcurrent := 0
	executionOrder := []int{}

	// 创建5个任务来测试执行模式
	for i := 0; i < 5; i++ {
		taskIndex := i
		task := NewTaskBuilder().
			WithType("concurrent-test").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				mu.Lock()
				runningTasks++
				if runningTasks > maxConcurrent {
					maxConcurrent = runningTasks
				}
				executionOrder = append(executionOrder, taskIndex)
				mu.Unlock()

				// 模拟任务执行
				time.Sleep(200 * time.Millisecond)

				mu.Lock()
				runningTasks--
				mu.Unlock()
				return nil
			}).
			Build()

		err := scheduler.Schedule(task)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	}

	// 等待所有任务完成
	time.Sleep(1 * time.Second)

	mu.Lock()
	t.Logf("最大并发任务数: %d", maxConcurrent)
	t.Logf("执行顺序: %v", executionOrder)
	mu.Unlock()

	// 验证当前只支持并发执行
	if maxConcurrent <= 1 {
		t.Error("期望支持并发执行，但实际最大并发数为1")
	}

	// 检查是否有顺序执行的能力
	// 在当前实现中，任务执行顺序是不确定的
	if len(executionOrder) != 5 {
		t.Errorf("期望5个任务执行，实际执行了%d个", len(executionOrder))
	}
}