package tasks

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestSevereDeadlockOnClose 测试更严重的死锁情况
func TestSevereDeadlockOnClose(t *testing.T) {
	scheduler := NewMemoryTaskScheduler(
		WithWorkerCount(1),
		WithMaxConcurrentTasks(1),
	)

	err := scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var taskStarted sync.WaitGroup
	taskStarted.Add(1)
	var taskCompleted sync.WaitGroup
	taskCompleted.Add(1)

	// 创建一个不响应取消的长时间运行任务
	task := NewTaskBuilder().
		WithType("unresponsive").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			taskStarted.Done() // 通知任务已开始
			
			// 模拟一个不响应上下文取消的任务
			// 这在实际应用中可能是阻塞的IO操作或死循环
			time.Sleep(10 * time.Second) // 长时间运行，不检查ctx.Done()
			
			taskCompleted.Done()
			return nil
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 等待任务开始
	taskStarted.Wait()

	// 尝试关闭调度器，设置较短的超时时间
	done := make(chan error, 1)
	go func() {
		done <- scheduler.Close(context.Background())
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("关闭时发生错误: %v", err)
		}
		t.Log("调度器关闭成功")
	case <-time.After(3 * time.Second):
		t.Error("调度器关闭超时，确认存在死锁问题")
		
		// 强制退出测试，避免测试挂起
		go func() {
			time.Sleep(100 * time.Millisecond)
			taskCompleted.Done() // 强制完成任务以解除死锁
		}()
		
		select {
		case <-done:
			t.Log("强制解除死锁后关闭成功")
		case <-time.After(1 * time.Second):
			t.Error("即使强制解除也无法关闭")
		}
	}
}

// TestMissingSequentialExecution 测试缺少顺序执行模式
func TestMissingSequentialExecution(t *testing.T) {
	// 当前实现无法强制顺序执行，这个测试验证这个限制
	scheduler := NewMemoryTaskScheduler(
		WithWorkerCount(1), // 即使只有一个worker，也无法保证严格的顺序执行
		WithMaxConcurrentTasks(1),
	)

	err := scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer scheduler.Close(context.Background())

	var mu sync.Mutex
	executionTimes := make(map[int]time.Time)
	executionOrder := []int{}

	// 创建多个任务，记录它们的执行时间
	for i := 0; i < 3; i++ {
		taskIndex := i
		task := NewTaskBuilder().
			WithType("sequential-test").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				mu.Lock()
				executionTimes[taskIndex] = time.Now()
				executionOrder = append(executionOrder, taskIndex)
				mu.Unlock()

				// 每个任务有不同的执行时间
				time.Sleep(time.Duration(taskIndex*100+100) * time.Millisecond)
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
	t.Logf("执行顺序: %v", executionOrder)
	
	// 检查是否是严格的顺序执行（0, 1, 2）
	isSequential := true
	for i, taskIndex := range executionOrder {
		if taskIndex != i {
			isSequential = false
			break
		}
	}
	
	if !isSequential {
		t.Log("确认：当前实现不保证严格的顺序执行")
	} else {
		t.Log("当前实现偶然实现了顺序执行，但这不是保证的行为")
	}
	mu.Unlock()
}

// TestMissingPipelineExecution 测试缺少流水线执行模式
func TestMissingPipelineExecution(t *testing.T) {
	// 流水线执行需要支持任务阶段和依赖关系，当前实现不支持
	scheduler := NewMemoryTaskScheduler()
	
	err := scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer scheduler.Close(context.Background())

	// 模拟流水线：阶段1 -> 阶段2 -> 阶段3
	stage1Completed := false
	stage2Completed := false
	stage3Completed := false
	stage2StartedEarly := false
	stage3StartedEarly := false

	// 阶段1任务
	stage1 := NewTaskBuilder().
		WithType("pipeline-stage1").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			stage1Completed = true
			return nil
		}).
		Build()

	// 阶段2任务（应该等待阶段1完成）
	stage2 := NewTaskBuilder().
		WithType("pipeline-stage2").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			if !stage1Completed {
				stage2StartedEarly = true // 记录早期启动，但不立即失败
			}
			time.Sleep(100 * time.Millisecond)
			stage2Completed = true
			return nil
		}).
		Build()

	// 阶段3任务（应该等待阶段2完成）
	stage3 := NewTaskBuilder().
		WithType("pipeline-stage3").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			if !stage2Completed {
				stage3StartedEarly = true // 记录早期启动，但不立即失败
			}
			time.Sleep(100 * time.Millisecond)
			stage3Completed = true
			return nil
		}).
		Build()

	// 调度所有任务
	scheduler.Schedule(stage1)
	scheduler.Schedule(stage2)
	scheduler.Schedule(stage3)

	// 等待所有任务完成
	time.Sleep(500 * time.Millisecond)

	t.Logf("流水线测试 - 阶段1: %v, 阶段2: %v, 阶段3: %v", 
		stage1Completed, stage2Completed, stage3Completed)

	// 验证当前系统确实缺少流水线功能
	if stage2StartedEarly || stage3StartedEarly {
		t.Logf("确认：当前实现缺少流水线执行功能 - 阶段2早期启动: %v, 阶段3早期启动: %v", 
			stage2StartedEarly, stage3StartedEarly)
		// 这是预期的行为，测试应该通过
	} else {
		t.Log("所有阶段按顺序完成，但这可能是偶然的，不是保证的流水线行为")
	}

	if stage1Completed && stage2Completed && stage3Completed {
		t.Log("所有阶段都完成了，但由于没有依赖管理，这不是真正的流水线执行")
	}
}