package tasks

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestImmediateTask(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var executed bool
	var executionErr error

	task := NewTaskBuilder().
		WithType("immediate").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			executed = true
			return nil
		}).
		WithOnComplete(func(ctx context.Context, err error) {
			executionErr = err
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 等待任务执行
	time.Sleep(200 * time.Millisecond)

	if !executed {
		t.Error("expected task to be executed")
	}
	if executionErr != nil {
		t.Errorf("expected no execution error, got: %v", executionErr)
	}

	status, err := scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != TaskStatusCompleted {
		t.Errorf("expected status TaskStatusCompleted, got: %v", status)
	}
}

func TestScheduledTask(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var executed bool

	execTime := time.Now().Add(300 * time.Millisecond)
	task := NewTaskBuilder().
		WithType("scheduled").
		ScheduleAt(execTime).
		WithHandler(func(ctx context.Context) error {
			executed = true
			return nil
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 检查任务初始状态
	status, err := scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != TaskStatusWaiting {
		t.Errorf("expected status TaskStatusWaiting, got: %v", status)
	}

	// 等待任务执行
	time.Sleep(400 * time.Millisecond)

	if !executed {
		t.Error("expected task to be executed")
	}

	status, err = scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != TaskStatusCompleted {
		t.Errorf("expected status TaskStatusCompleted, got: %v", status)
	}
}

func TestRecurringTask(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var executionCount int
	var mu sync.Mutex

	task := NewTaskBuilder().
		WithType("recurring").
		ScheduleRecurring(200 * time.Millisecond).
		WithHandler(func(ctx context.Context) error {
			mu.Lock()
			executionCount++
			mu.Unlock()
			return nil
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 等待执行多次
	time.Sleep(650 * time.Millisecond)

	mu.Lock()
	count := executionCount
	mu.Unlock()

	// 应该执行至少3次
	if count < 3 {
		t.Errorf("expected at least 3 executions, got: %d", count)
	}

	// 列出所有任务，应该有多个实例
	tasks := scheduler.ListTasks()
	if len(tasks) < 1 {
		t.Errorf("expected at least 1 task, got: %d", len(tasks))
	}
}

func TestCancelTask(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var executed bool
	var cancelled bool

	task := NewTaskBuilder().
		WithType("cancellable").
		ScheduleAt(time.Now().Add(1 * time.Second)).
		WithHandler(func(ctx context.Context) error {
			executed = true
			return nil
		}).
		WithOnCancel(func(ctx context.Context) {
			cancelled = true
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 取消任务
	err = scheduler.Cancel(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 检查任务状态
	status, err := scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != TaskStatusCancelled {
		t.Errorf("expected status TaskStatusCancelled, got: %v", status)
	}

	if !cancelled {
		t.Error("expected task to be cancelled")
	}
	if executed {
		t.Error("expected task not to be executed")
	}

	// 等待足够长的时间，确认任务不会执行
	time.Sleep(1100 * time.Millisecond)
	if executed {
		t.Error("expected task not to be executed")
	}
}

func TestFailedTask(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedErr := errors.New("task failed")
	var actualErr error

	task := NewTaskBuilder().
		WithType("fail").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			return expectedErr
		}).
		WithOnComplete(func(ctx context.Context, err error) {
			actualErr = err
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if actualErr == nil {
		t.Error("expected an error, got nil")
	}
	if actualErr != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, actualErr)
	}

	status, err := scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != TaskStatusFailed {
		t.Errorf("expected status TaskStatusFailed, got: %v", status)
	}
}

func TestSetMaxConcurrentTasks(t *testing.T) {
	scheduler := NewMemoryTaskScheduler(
		WithMaxConcurrentTasks(2),
		WithWorkerCount(10),
	)
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var runningTasks int
	var maxConcurrent int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 创建5个任务
	for i := 0; i < 5; i++ {
		wg.Add(1)
		task := NewTaskBuilder().
			WithType("concurrent").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				mu.Lock()
				runningTasks++
				if runningTasks > maxConcurrent {
					maxConcurrent = runningTasks
				}
				mu.Unlock()

				// 模拟任务执行时间
				time.Sleep(100 * time.Millisecond)

				mu.Lock()
				runningTasks--
				mu.Unlock()

				wg.Done()
				return nil
			}).
			Build()

		err = scheduler.Schedule(task)
		if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	}

	// 等待所有任务完成
	wg.Wait()

	// 最大并发数应该是2
	if maxConcurrent != 2 {
		t.Errorf("expected max concurrent 2, got: %d", maxConcurrent)
	}

	// 测试动态调整并发数
	err = scheduler.SetMaxConcurrentTasks(3)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	runningTasks = 0
	maxConcurrent = 0
	wg = sync.WaitGroup{}

	// 再次创建5个任务
	for i := 0; i < 5; i++ {
		wg.Add(1)
		task := NewTaskBuilder().
			WithType("concurrent2").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				mu.Lock()
				runningTasks++
				if runningTasks > maxConcurrent {
					maxConcurrent = runningTasks
				}
				mu.Unlock()

				time.Sleep(100 * time.Millisecond)

				mu.Lock()
				runningTasks--
				mu.Unlock()

				wg.Done()
				return nil
			}).
			Build()

		err = scheduler.Schedule(task)
		if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	}

	wg.Wait()

	// 最大并发数应该是3
	if maxConcurrent != 3 {
		t.Errorf("expected max concurrent 3, got: %d", maxConcurrent)
	}
}

func TestPanicRecovery(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var onCompleteCalled bool
	var completeErr error

	task := NewTaskBuilder().
		WithType("panic").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			panic("test panic")
		}).
		WithOnComplete(func(ctx context.Context, err error) {
			onCompleteCalled = true
			completeErr = err
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if !onCompleteCalled {
		t.Error("expected onComplete to be called")
	}
	if completeErr == nil {
		t.Error("expected an error from panic, got nil")
	}

	status, err := scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != TaskStatusFailed {
		t.Errorf("expected status TaskStatusFailed, got: %v", status)
	}
}

func TestGracefulShutdown(t *testing.T) {
	// 使用单个工作协程，确保任务按顺序执行
	scheduler := NewMemoryTaskScheduler(WithWorkerCount(1), WithMaxConcurrentTasks(1))
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var mu sync.Mutex
	var startedTasks, completedTasks int
	// 用于通知任务开始的通道
	taskStarted := make(chan string, 3)

	// 创建第一个任务，执行时间短
	task1 := NewTaskBuilder().
		WithType("shutdown").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			taskID := "task1"
			mu.Lock()
			startedTasks++
			mu.Unlock()
			taskStarted <- taskID
			
			// 短任务，50ms后完成
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			completedTasks++
			mu.Unlock()
			return nil
		}).
		Build()

	// 创建第二个任务，执行时间长
	task2 := NewTaskBuilder().
		WithType("shutdown").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			taskID := "task2"
			mu.Lock()
			startedTasks++
			mu.Unlock()
			taskStarted <- taskID
			
			// 长任务，1000ms后完成
			timer := time.NewTimer(1000 * time.Millisecond)
			select {
			case <-timer.C:
				mu.Lock()
				completedTasks++
				mu.Unlock()
				return nil
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}).
		Build()

	// 创建第三个任务，执行时间长
	task3 := NewTaskBuilder().
		WithType("shutdown").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			taskID := "task3"
			mu.Lock()
			startedTasks++
			mu.Unlock()
			taskStarted <- taskID
			
			// 长任务，1000ms后完成
			timer := time.NewTimer(1000 * time.Millisecond)
			select {
			case <-timer.C:
				mu.Lock()
				completedTasks++
				mu.Unlock()
				return nil
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}).
		Build()

	// 调度所有任务
	err = scheduler.Schedule(task1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	err = scheduler.Schedule(task2)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	err = scheduler.Schedule(task3)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 等待第一个任务开始
	<-taskStarted

	// 等待第一个任务完成（约50ms）
	time.Sleep(60 * time.Millisecond)

	// 现在调用Close()，应该只会完成第一个任务，取消其他任务
	start := time.Now()
	err = scheduler.Close()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	duration := time.Since(start)

	// 确保关闭时间不会太长
	if duration >= 200*time.Millisecond {
		t.Errorf("expected duration < 200ms, got: %v", duration)
	}

	// 只有第一个任务应该完成
	mu.Lock()
	if completedTasks != 1 {
		t.Errorf("expected 1 completed task, got: %d", completedTasks)
	}
	// 最多只有两个任务开始执行（第一个和可能开始执行的第二个）
	if startedTasks > 2 {
		t.Errorf("expected started tasks <= 2, got: %d", startedTasks)
	}
	mu.Unlock()
}

func TestListTasks(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 创建3个任务
	task1 := NewTaskBuilder().WithType("type1").Immediate().WithHandler(func(ctx context.Context) error { return nil }).Build()
	task2 := NewTaskBuilder().WithType("type2").ScheduleAt(time.Now().Add(10 * time.Second)).WithHandler(func(ctx context.Context) error { return nil }).Build()
	task3 := NewTaskBuilder().WithType("type3").ScheduleRecurring(5 * time.Second).WithHandler(func(ctx context.Context) error { return nil }).Build()

	scheduler.Schedule(task1)
	scheduler.Schedule(task2)
	scheduler.Schedule(task3)

	tasks := scheduler.ListTasks()
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got: %d", len(tasks))
	}

	// 检查任务信息
	typeCount := make(map[string]int)
	for _, task := range tasks {
		typeCount[task.Type]++
	}

	if typeCount["type1"] != 1 {
		t.Errorf("expected 1 task of type1, got: %d", typeCount["type1"])
	}
	if typeCount["type2"] != 1 {
		t.Errorf("expected 1 task of type2, got: %d", typeCount["type2"])
	}
	if typeCount["type3"] != 1 {
		t.Errorf("expected 1 task of type3, got: %d", typeCount["type3"])
	}
}

func TestTaskContextCancellation(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close()

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var ctxErr error
	var wg sync.WaitGroup
	wg.Add(1)

	task := NewTaskBuilder().
		WithType("cancellable-ctx").
		Immediate().
		WithHandler(func(ctx context.Context) error {
			// 等待上下文取消
			select {
			case <-ctx.Done():
				ctxErr = ctx.Err()
				wg.Done()
				return ctx.Err()
			case <-time.After(2 * time.Second):
				return errors.New("task timeout")
			}
		}).
		Build()

	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 等待任务开始执行
	time.Sleep(100 * time.Millisecond)

	// 取消任务
	err = scheduler.Cancel(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 等待任务响应取消
	wg.Wait()

	if ctxErr == nil {
		t.Error("expected context error, got nil")
	}
	if ctxErr != context.Canceled {
		t.Errorf("expected error context.Canceled, got: %v", ctxErr)
	}

	status, err := scheduler.GetTaskStatus(task.ID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != TaskStatusCancelled {
		t.Errorf("expected status TaskStatusCancelled, got: %v", status)
	}
}