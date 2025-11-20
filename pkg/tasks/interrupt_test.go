package tasks

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestGracefulShutdownWithInterrupt 测试在任务执行过程中收到中断信号时的优雅关闭
func TestGracefulShutdownWithInterrupt(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	
	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}

	// 创建一个长时间运行的任务
	var taskStarted sync.WaitGroup
	var taskCompleted sync.WaitGroup
	
	taskStarted.Add(1)
	taskCompleted.Add(1)
	
	longRunningTask := NewTaskBuilder().
		WithType("long_running").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			taskStarted.Done() // 通知任务已开始
			
			// 模拟长时间工作，同时监听上下文取消
			select {
			case <-ctx.Done():
				t.Log("任务收到取消信号，正在优雅退出")
				return ctx.Err()
			case <-time.After(10 * time.Second):
				t.Log("任务正常完成")
				taskCompleted.Done()
				return nil
			}
		}).
		Build()

	// 调度任务
	err = scheduler.Schedule(longRunningTask)
	if err != nil {
		t.Fatalf("调度任务失败: %v", err)
	}

	// 等待任务开始
	taskStarted.Wait()
	t.Log("任务已开始执行")

	// 模拟收到中断信号
	go func() {
		time.Sleep(100 * time.Millisecond) // 给任务一点时间开始
		t.Log("模拟收到中断信号...")
		
		// 调用Close方法模拟优雅关闭
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		err := scheduler.Close(shutdownCtx)
		if err != nil {
			t.Errorf("关闭调度器失败: %v", err)
		} else {
			t.Log("调度器已优雅关闭")
		}
	}()

	// 等待任务完成或被取消
	done := make(chan struct{})
	go func() {
		taskCompleted.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("任务正常完成")
	case <-time.After(2 * time.Second):
		// 如果2秒后还没完成，说明可能有问题
		t.Log("任务在2秒内未完成，检查是否被正确取消")
		
		// 检查任务状态
		tasks := scheduler.ListTasks()
		for _, task := range tasks {
			if task.ID == longRunningTask.ID() {
				t.Logf("任务状态: %s", task.Status)
				if task.Status == TaskStatusCancelled {
					t.Log("✓ 任务被正确取消")
				} else if task.Status == TaskStatusRunning {
					t.Error("✗ 任务仍在运行，可能存在死锁")
				}
			}
		}
	}
}

// TestMultipleTasksInterrupt 测试多个任务执行时的中断处理
func TestMultipleTasksInterrupt(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	
	// 设置为并发模式以测试多个任务同时执行
	err := scheduler.SetExecutionMode(ExecutionModeConcurrent)
	if err != nil {
		t.Fatalf("设置并发执行模式失败: %v", err)
	}

	ctx := context.Background()
	err = scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}

	var tasksStarted sync.WaitGroup
	var tasksCount int

	// 创建多个长时间运行的任务
	for i := 0; i < 3; i++ {
		tasksStarted.Add(1)
		tasksCount++
		
		task := NewTaskBuilder().
			WithType("long_running").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				tasksStarted.Done() // 通知任务已开始
				
				// 模拟长时间工作，同时监听上下文取消
				select {
				case <-ctx.Done():
					t.Logf("任务 %d 收到取消信号，正在优雅退出", i)
					return ctx.Err()
				case <-time.After(10 * time.Second):
					t.Logf("任务 %d 正常完成", i)
					return nil
				}
			}).
			Build()

		err = scheduler.Schedule(task)
		if err != nil {
			t.Fatalf("调度任务 %d 失败: %v", i, err)
		}
	}

	// 等待所有任务开始
	tasksStarted.Wait()
	t.Logf("所有 %d 个任务已开始执行", tasksCount)

	// 模拟收到中断信号
	go func() {
		time.Sleep(100 * time.Millisecond)
		t.Log("模拟收到中断信号，关闭调度器...")
		
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		start := time.Now()
		err := scheduler.Close(shutdownCtx)
		duration := time.Since(start)
		
		if err != nil {
			t.Errorf("关闭调度器失败: %v", err)
		} else {
			t.Logf("✓ 调度器在 %v 内优雅关闭", duration)
			
			// 验证关闭时间是否合理（不应该超过5秒）
			if duration > 6*time.Second {
				t.Errorf("关闭时间过长: %v，可能存在死锁", duration)
			}
		}
	}()

	// 等待关闭完成
	time.Sleep(2 * time.Second)
	
	// 检查任务状态
	tasks := scheduler.ListTasks()
	cancelledCount := 0
	
	for _, task := range tasks {
		t.Logf("任务 %s 状态: %s", task.ID, task.Status)
		if task.Status == TaskStatusCancelled {
			cancelledCount++
		}
	}
	
	if cancelledCount == tasksCount {
		t.Logf("✓ 所有 %d 个任务都被正确取消", cancelledCount)
	} else {
		t.Errorf("✗ 期望 %d 个任务被取消，实际 %d 个", tasksCount, cancelledCount)
	}
}

// TestRealSignalHandling 测试真实的信号处理（仅在非测试环境中运行）
func TestRealSignalHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过真实信号处理测试")
	}
	
	scheduler := NewMemoryTaskScheduler()
	
	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}

	// 创建一个任务
	task := NewTaskBuilder().
		WithType("test_task").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				t.Log("任务收到取消信号")
				return ctx.Err()
			case <-time.After(30 * time.Second):
				t.Log("任务完成")
				return nil
			}
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("调度任务失败: %v", err)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在单独的goroutine中处理信号
	go func() {
		sig := <-sigChan
		t.Logf("收到信号: %v", sig)
		
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		err := scheduler.Close(shutdownCtx)
		if err != nil {
			t.Errorf("关闭调度器失败: %v", err)
		} else {
			t.Log("调度器已优雅关闭")
		}
	}()

	t.Log("测试已设置，请在5秒内按 Ctrl+C 发送中断信号...")
	
	// 等待信号或超时
	select {
	case <-time.After(5 * time.Second):
		t.Log("测试超时，手动关闭调度器")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		scheduler.Close(shutdownCtx)
	case <-sigChan:
		t.Log("收到中断信号，测试完成")
		time.Sleep(1 * time.Second) // 给关闭操作一点时间
	}
}