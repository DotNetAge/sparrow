package tasks

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestFixedGracefulShutdown 测试修复后的优雅关闭
func TestFixedGracefulShutdown(t *testing.T) {
	scheduler := NewMemoryTaskScheduler(
		WithWorkerCount(2),
		WithMaxConcurrentTasks(2),
	)

	err := scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var taskStarted sync.WaitGroup
	taskStarted.Add(1)

	// 创建一个长时间运行但不响应取消的任务
	task := NewTaskBuilder().
		WithType("long-running").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			taskStarted.Done()
			// 模拟不响应取消的长时间任务
			time.Sleep(10 * time.Second)
			return nil
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 等待任务开始
	taskStarted.Wait()

	// 使用带超时的上下文关闭调度器
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err = scheduler.Close(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Logf("关闭时发生错误（预期）: %v", err)
	}

	// 关闭应该在超时时间内完成
	if duration > 3*time.Second {
		t.Errorf("关闭时间过长: %v", duration)
	} else {
		t.Logf("关闭成功，耗时: %v", duration)
	}
}

// TestFixedGracefulShutdownWithResponsiveTasks 测试响应取消的任务的关闭
func TestFixedGracefulShutdownWithResponsiveTasks(t *testing.T) {
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
	cancelledTasks := 0

	// 创建多个任务，一些响应取消，一些不响应
	for i := 0; i < 4; i++ {
		taskIndex := i
		task := NewTaskBuilder().
			WithType("mixed").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				if taskIndex < 2 {
					// 前两个任务响应取消
					select {
					case <-time.After(5 * time.Second):
						mu.Lock()
						completedTasks++
						mu.Unlock()
						return nil
					case <-ctx.Done():
						mu.Lock()
						cancelledTasks++
						mu.Unlock()
						return ctx.Err()
					}
				} else {
					// 后两个任务不响应取消
					time.Sleep(5 * time.Second)
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

	// 使用带超时的上下文关闭
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	err = scheduler.Close(ctx)
	duration := time.Since(start)

	t.Logf("关闭耗时: %v", duration)
	t.Logf("完成的任务数: %d", completedTasks)
	t.Logf("取消的任务数: %d", cancelledTasks)

	if duration > 4*time.Second {
		t.Errorf("关闭时间过长: %v", duration)
	}
}