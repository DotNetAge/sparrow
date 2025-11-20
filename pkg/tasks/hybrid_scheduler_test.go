package tasks

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestHybridTaskScheduler 测试混合任务调度器
func TestHybridTaskScheduler(t *testing.T) {
	// 创建混合调度器
	scheduler := NewHybridTaskScheduler(
		WithHybridWorkerCount(3, 1, 1),
		WithHybridMaxConcurrentTasks(5),
	)
	defer scheduler.Close(context.Background())

	// 注册任务策略
	err := scheduler.RegisterTaskPolicy("data-processing", PolicyConcurrent)
	assert.NoError(t, err)
	
	err = scheduler.RegisterTaskPolicy("file-operation", PolicySequential)
	assert.NoError(t, err)
	
	err = scheduler.RegisterTaskPolicy("pipeline-stage", PolicyPipeline)
	assert.NoError(t, err)

	// 启动调度器
	err = scheduler.Start(context.Background())
	assert.NoError(t, err)
	defer scheduler.Stop()

	var wg sync.WaitGroup
	completedTasks := make(map[string]string)
	var mu sync.Mutex

	// 创建并发任务
	for i := 0; i < 5; i++ {
		wg.Add(1)
		taskIndex := i
		task := NewTaskBuilder().
			WithType("data-processing").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				defer wg.Done()
				time.Sleep(100 * time.Millisecond) // 模拟处理时间
				
				mu.Lock()
				completedTasks[fmt.Sprintf("concurrent-%d", taskIndex)] = "completed"
				mu.Unlock()
				
				return nil
			}).
			Build()
		
		err := scheduler.Schedule(task)
		assert.NoError(t, err)
	}

	// 创建顺序任务
	for i := 0; i < 3; i++ {
		wg.Add(1)
		taskIndex := i
		task := NewTaskBuilder().
			WithType("file-operation").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				defer wg.Done()
				time.Sleep(50 * time.Millisecond) // 模拟文件操作
				
				mu.Lock()
				completedTasks[fmt.Sprintf("sequential-%d", taskIndex)] = "completed"
				mu.Unlock()
				
				return nil
			}).
			Build()
		
		err := scheduler.Schedule(task)
		assert.NoError(t, err)
	}

	// 创建流水线任务
	for i := 0; i < 3; i++ {
		wg.Add(1)
		taskIndex := i
		task := NewTaskBuilder().
			WithType("pipeline-stage").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				defer wg.Done()
				time.Sleep(30 * time.Millisecond) // 模拟流水线处理
				
				mu.Lock()
				completedTasks[fmt.Sprintf("pipeline-%d", taskIndex)] = "completed"
				mu.Unlock()
				
				return nil
			}).
			Build()
		
		err := scheduler.Schedule(task)
		assert.NoError(t, err)
	}

	// 等待所有任务完成
	wg.Wait()
	time.Sleep(200 * time.Millisecond) // 额外等待确保所有任务完成

	// 验证结果
	mu.Lock()
	assert.Equal(t, 11, len(completedTasks), "应该完成所有任务")
	
	// 验证不同类型的任务都已完成
	concurrentCount := 0
	sequentialCount := 0
	pipelineCount := 0
	
	for taskID := range completedTasks {
		switch {
		case taskID[:10] == "concurrent":
			concurrentCount++
		case taskID[:10] == "sequential":
			sequentialCount++
		case taskID[:8] == "pipeline":
			pipelineCount++
		}
	}
	
	assert.Equal(t, 5, concurrentCount, "应该完成5个并发任务")
	assert.Equal(t, 3, sequentialCount, "应该完成3个顺序任务")
	assert.Equal(t, 3, pipelineCount, "应该完成3个流水线任务")
	mu.Unlock()

	// 检查统计信息
	stats := scheduler.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(5), stats[PolicyConcurrent].TotalTasks)
	assert.Equal(t, int64(3), stats[PolicySequential].TotalTasks)
	assert.Equal(t, int64(3), stats[PolicyPipeline].TotalTasks)
}

// TestHybridTaskSchedulerPerformance 测试混合调度器性能
func TestHybridTaskSchedulerPerformance(t *testing.T) {
	scheduler := NewHybridTaskScheduler(
		WithHybridWorkerCount(5, 1, 1),
		WithHybridMaxConcurrentTasks(10),
	)
	defer scheduler.Close(context.Background())

	// 注册任务策略
	scheduler.RegisterTaskPolicy("concurrent-task", PolicyConcurrent)
	scheduler.RegisterTaskPolicy("sequential-task", PolicySequential)

	err := scheduler.Start(context.Background())
	assert.NoError(t, err)
	defer scheduler.Stop()

	startTime := time.Now()
	var wg sync.WaitGroup

	// 创建大量并发任务
	for i := 0; i < 100; i++ {
		wg.Add(1)
		task := NewTaskBuilder().
			WithType("concurrent-task").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond)
				return nil
			}).
			Build()
		
		scheduler.Schedule(task)
	}

	// 创建一些顺序任务
	for i := 0; i < 10; i++ {
		wg.Add(1)
		task := NewTaskBuilder().
			WithType("sequential-task").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				defer wg.Done()
				time.Sleep(20 * time.Millisecond)
				return nil
			}).
			Build()
		
		scheduler.Schedule(task)
	}

	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("执行110个任务耗时: %v", duration)
	
	// 并发任务应该能够快速完成
	assert.Less(t, duration, 2*time.Second, "并发任务应该能够在2秒内完成")

	// 检查统计信息
	stats := scheduler.GetStats()
	assert.Equal(t, int64(100), stats[PolicyConcurrent].TotalTasks)
	assert.Equal(t, int64(10), stats[PolicySequential].TotalTasks)
}

// ExampleHybridTaskScheduler 混合调度器使用示例
func ExampleHybridTaskScheduler() {
	// 创建混合调度器
	scheduler := NewHybridTaskScheduler(
		WithHybridWorkerCount(5, 2, 2),
		WithHybridMaxConcurrentTasks(20),
	)

	// 注册不同任务类型的执行策略
	scheduler.RegisterTaskPolicy("image-processing", PolicyConcurrent)    // 图片处理可以并发
	scheduler.RegisterTaskPolicy("file-write", PolicySequential)          // 文件写入需要顺序
	scheduler.RegisterTaskPolicy("data-pipeline", PolicyPipeline)        // 数据处理流水线

	// 启动调度器
	scheduler.Start(context.Background())
	defer scheduler.Close(context.Background())

	// 调度不同类型的任务
	// 并发处理的图片任务
	for i := 0; i < 10; i++ {
		task := NewTaskBuilder().
			WithType("image-processing").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				// 图片处理逻辑
				fmt.Printf("处理图片 %d\n", i)
				return nil
			}).
			Build()
		scheduler.Schedule(task)
	}

	// 顺序执行的文件写入任务
	for i := 0; i < 5; i++ {
		task := NewTaskBuilder().
			WithType("file-write").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				// 文件写入逻辑
				fmt.Printf("写入文件 %d\n", i)
				return nil
			}).
			Build()
		scheduler.Schedule(task)
	}

	// 流水线处理的数据任务
	for i := 0; i < 3; i++ {
		task := NewTaskBuilder().
			WithType("data-pipeline").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				// 数据流水线处理逻辑
				fmt.Printf("流水线处理阶段 %d\n", i)
				return nil
			}).
			Build()
		scheduler.Schedule(task)
	}

	// 获取执行统计信息
	stats := scheduler.GetStats()
	fmt.Printf("并发任务统计: %+v\n", stats[PolicyConcurrent])
	fmt.Printf("顺序任务统计: %+v\n", stats[PolicySequential])
	fmt.Printf("流水线任务统计: %+v\n", stats[PolicyPipeline])
}