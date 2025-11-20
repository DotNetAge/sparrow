package tasks

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestSequentialExecution 测试顺序执行模式
func TestSequentialExecution(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	
	// 设置为顺序执行模式
	err := scheduler.SetExecutionMode(ExecutionModeSequential)
	if err != nil {
		t.Fatalf("设置顺序执行模式失败: %v", err)
	}

	// 验证执行模式
	if scheduler.GetExecutionMode() != ExecutionModeSequential {
		t.Fatal("执行模式设置失败")
	}

	ctx := context.Background()
	err = scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(context.Background())

	var executionOrder []int
	var mu sync.Mutex

	// 创建5个任务，每个任务记录执行顺序
	for i := 0; i < 5; i++ {
		taskIndex := i
		task := NewTaskBuilder().
			WithType("sequential_test").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				mu.Lock()
				executionOrder = append(executionOrder, taskIndex)
				mu.Unlock()
				
				// 模拟任务执行时间
				time.Sleep(100 * time.Millisecond)
				return nil
			}).
			Build()

		err := scheduler.Schedule(task)
		if err != nil {
			t.Fatalf("调度任务失败: %v", err)
		}
	}

	// 等待所有任务完成
	time.Sleep(1 * time.Second)

	// 验证执行顺序
	mu.Lock()
	defer mu.Unlock()
	
	if len(executionOrder) != 5 {
		t.Fatalf("期望执行5个任务，实际执行了%d个", len(executionOrder))
	}

	// 顺序执行模式下，任务应该按调度顺序执行
	for i := 0; i < 5; i++ {
		if executionOrder[i] != i {
			t.Errorf("执行顺序错误，期望位置%d为%d，实际为%d", i, i, executionOrder[i])
		}
	}

	t.Logf("顺序执行模式测试通过，执行顺序: %v", executionOrder)
}

// TestPipelineExecution 测试流水线执行模式
func TestPipelineExecution(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	
	// 设置为流水线执行模式
	err := scheduler.SetExecutionMode(ExecutionModePipeline)
	if err != nil {
		t.Fatalf("设置流水线执行模式失败: %v", err)
	}

	// 验证执行模式
	if scheduler.GetExecutionMode() != ExecutionModePipeline {
		t.Fatal("执行模式设置失败")
	}

	ctx := context.Background()
	err = scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(context.Background())

	var executionOrder []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 创建5个任务，每个任务记录执行顺序
	for i := 0; i < 5; i++ {
		taskIndex := i
		wg.Add(1)
		
		task := NewTaskBuilder().
			WithType("pipeline_test").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				defer wg.Done()
				
				mu.Lock()
				executionOrder = append(executionOrder, taskIndex)
				mu.Unlock()
				
				// 模拟任务执行时间
				time.Sleep(50 * time.Millisecond)
				return nil
			}).
			Build()

		err := scheduler.Schedule(task)
		if err != nil {
			t.Fatalf("调度任务失败: %v", err)
		}
	}

	// 等待所有任务完成
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// 验证执行结果
	mu.Lock()
	defer mu.Unlock()
	
	if len(executionOrder) != 5 {
		t.Fatalf("期望执行5个任务，实际执行了%d个", len(executionOrder))
	}

	// 流水线模式下，任务应该按调度顺序开始执行（但可能并发完成）
	// 这里我们主要验证所有任务都被执行了
	t.Logf("流水线执行模式测试通过，执行顺序: %v", executionOrder)
}

// TestExecutionModeSwitching 测试执行模式切换
func TestExecutionModeSwitching(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()

	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(context.Background())

	// 测试默认模式（并发）
	if scheduler.GetExecutionMode() != ExecutionModeConcurrent {
		t.Error("默认执行模式应该是并发模式")
	}

	// 切换到顺序模式
	err = scheduler.SetExecutionMode(ExecutionModeSequential)
	if err != nil {
		t.Fatalf("切换到顺序模式失败: %v", err)
	}

	if scheduler.GetExecutionMode() != ExecutionModeSequential {
		t.Error("执行模式切换失败")
	}

	// 切换到流水线模式
	err = scheduler.SetExecutionMode(ExecutionModePipeline)
	if err != nil {
		t.Fatalf("切换到流水线模式失败: %v", err)
	}

	if scheduler.GetExecutionMode() != ExecutionModePipeline {
		t.Error("执行模式切换失败")
	}

	// 切换回并发模式
	err = scheduler.SetExecutionMode(ExecutionModeConcurrent)
	if err != nil {
		t.Fatalf("切换到并发模式失败: %v", err)
	}

	if scheduler.GetExecutionMode() != ExecutionModeConcurrent {
		t.Error("执行模式切换失败")
	}

	t.Log("执行模式切换测试通过")
}

// TestConcurrentVsSequentialPerformance 测试并发vs顺序执行的性能差异
func TestConcurrentVsSequentialPerformance(t *testing.T) {
	// 测试并发模式
	concurrentScheduler := NewMemoryTaskScheduler()
	err := concurrentScheduler.SetExecutionMode(ExecutionModeConcurrent)
	if err != nil {
		t.Fatalf("设置并发模式失败: %v", err)
	}

	ctx := context.Background()
	err = concurrentScheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动并发调度器失败: %v", err)
	}
	defer concurrentScheduler.Close(context.Background())

	concurrentStart := time.Now()
	var concurrentWG sync.WaitGroup

	// 创建5个任务，每个执行100ms
	for i := 0; i < 5; i++ {
		concurrentWG.Add(1)
		task := NewTaskBuilder().
			WithType("concurrent_perf_test").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				defer concurrentWG.Done()
				time.Sleep(100 * time.Millisecond)
				return nil
			}).
			Build()

		concurrentScheduler.Schedule(task)
	}

	concurrentWG.Wait()
	concurrentDuration := time.Since(concurrentStart)

	// 测试顺序模式
	sequentialScheduler := NewMemoryTaskScheduler()
	err = sequentialScheduler.SetExecutionMode(ExecutionModeSequential)
	if err != nil {
		t.Fatalf("设置顺序模式失败: %v", err)
	}

	err = sequentialScheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动顺序调度器失败: %v", err)
	}
	defer sequentialScheduler.Close(context.Background())

	sequentialStart := time.Now()
	var sequentialWG sync.WaitGroup

	// 创建5个任务，每个执行100ms
	for i := 0; i < 5; i++ {
		sequentialWG.Add(1)
		task := NewTaskBuilder().
			WithType("sequential_perf_test").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				defer sequentialWG.Done()
				time.Sleep(100 * time.Millisecond)
				return nil
			}).
			Build()

		sequentialScheduler.Schedule(task)
	}

	sequentialWG.Wait()
	sequentialDuration := time.Since(sequentialStart)

	t.Logf("并发模式执行时间: %v", concurrentDuration)
	t.Logf("顺序模式执行时间: %v", sequentialDuration)

	// 并发模式应该比顺序模式快
	if concurrentDuration >= sequentialDuration {
		t.Error("并发模式应该比顺序模式执行得更快")
	}

	// 顺序模式应该大约是并发模式的5倍时间（5个任务串行执行）
	expectedSequentialDuration := concurrentDuration * 5
	if sequentialDuration < expectedSequentialDuration/2 { // 允许一定误差
		t.Errorf("顺序模式执行时间不符合预期，期望约%v，实际%v", expectedSequentialDuration, sequentialDuration)
	}
}