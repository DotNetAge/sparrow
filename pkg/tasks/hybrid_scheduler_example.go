package tasks

import (
	"context"
	"fmt"
	"log"
	"time"
)

// HybridSchedulerExample 演示如何在实际场景中使用混合任务调度器
func HybridSchedulerExample() {
	// 创建混合调度器配置
	scheduler := NewHybridTaskScheduler(
		WithHybridWorkerCount(10, 2, 3),  // 并发10个worker，顺序2个，流水线3个
		WithHybridMaxConcurrentTasks(50), // 最大并发任务数
	)

	// 注册任务执行策略
	registerTaskPolicies(scheduler)

	// 启动调度器
	ctx := context.Background()
	if err := scheduler.Start(ctx); err != nil {
		log.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(ctx)

	// 模拟高频任务提交
	simulateHighFrequencyTasks(scheduler)

	// 监控调度器状态
	monitorSchedulerStatus(scheduler)
}

// registerTaskPolicies 注册不同任务类型的执行策略
func registerTaskPolicies(scheduler *HybridTaskScheduler) {
	// 数据处理类任务 - 并发执行
	scheduler.RegisterTaskPolicy("data-analysis", PolicyConcurrent)
	scheduler.RegisterTaskPolicy("image-processing", PolicyConcurrent)
	scheduler.RegisterTaskPolicy("video-encoding", PolicyConcurrent)

	// 文件操作类任务 - 顺序执行
	scheduler.RegisterTaskPolicy("file-write", PolicySequential)
	scheduler.RegisterTaskPolicy("database-backup", PolicySequential)
	scheduler.RegisterTaskPolicy("log-rotation", PolicySequential)

	// 流水线处理类任务 - 流水线执行
	scheduler.RegisterTaskPolicy("data-pipeline", PolicyPipeline)
	scheduler.RegisterTaskPolicy("etl-process", PolicyPipeline)
	scheduler.RegisterTaskPolicy("report-generation", PolicyPipeline)
}

// simulateHighFrequencyTasks 模拟高频任务提交场景
func simulateHighFrequencyTasks(scheduler *HybridTaskScheduler) {
	// 模拟高频数据分析和图片处理任务
	go func() {
		for i := 0; i < 100; i++ {
			// 数据分析任务
			dataTask := NewTaskBuilder().
				WithType("data-analysis").
				Immediate().
				WithHandler(func(ctx context.Context) error {
					// 模拟数据分析处理
					time.Sleep(50 * time.Millisecond)
					fmt.Printf("完成数据分析任务 %d\n", i)
					return nil
				}).
				Build()
			scheduler.Schedule(dataTask)

			// 图片处理任务
			imageTask := NewTaskBuilder().
				WithType("image-processing").
				Immediate().
				WithHandler(func(ctx context.Context) error {
					// 模拟图片处理
					time.Sleep(80 * time.Millisecond)
					fmt.Printf("完成图片处理任务 %d\n", i)
					return nil
				}).
				Build()
			scheduler.Schedule(imageTask)

			// 模拟高频提交（每10ms提交一批任务）
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// 模拟文件写入任务（需要顺序执行）
	go func() {
		for i := 0; i < 20; i++ {
			fileTask := NewTaskBuilder().
				WithType("file-write").
				Immediate().
				WithHandler(func(ctx context.Context) error {
					// 模拟文件写入操作
					time.Sleep(30 * time.Millisecond)
					fmt.Printf("完成文件写入任务 %d\n", i)
					return nil
				}).
				Build()
			scheduler.Schedule(fileTask)

			// 文件写入任务相对较少
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// 模拟流水线处理任务
	go func() {
		for i := 0; i < 30; i++ {
			pipelineTask := NewTaskBuilder().
				WithType("data-pipeline").
				Immediate().
				WithHandler(func(ctx context.Context) error {
					// 模拟流水线处理
					time.Sleep(40 * time.Millisecond)
					fmt.Printf("完成流水线处理任务 %d\n", i)
					return nil
				}).
				Build()
			scheduler.Schedule(pipelineTask)

			// 流水线任务中等频率
			time.Sleep(50 * time.Millisecond)
		}
	}()
}

// monitorSchedulerStatus 监控调度器状态
func monitorSchedulerStatus(scheduler *HybridTaskScheduler) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 10; i++ { // 监控20秒
		select {
		case <-ticker.C:
			stats := scheduler.GetStats()
			
			fmt.Println("\n=== 调度器状态报告 ===")
			
			// 并发执行统计
			concurrentStats := stats[PolicyConcurrent]
			fmt.Printf("并发执行: 总任务=%d, 完成=%d, 失败=%d, 进行中=%d\n",
				concurrentStats.TotalTasks, concurrentStats.CompletedTasks,
				concurrentStats.FailedTasks, concurrentStats.RunningTasks)
			
			// 顺序执行统计
			sequentialStats := stats[PolicySequential]
			fmt.Printf("顺序执行: 总任务=%d, 完成=%d, 失败=%d, 进行中=%d\n",
				sequentialStats.TotalTasks, sequentialStats.CompletedTasks,
				sequentialStats.FailedTasks, sequentialStats.RunningTasks)
			
			// 流水线执行统计
			pipelineStats := stats[PolicyPipeline]
			fmt.Printf("流水线执行: 总任务=%d, 完成=%d, 失败=%d, 进行中=%d\n",
				pipelineStats.TotalTasks, pipelineStats.CompletedTasks,
				pipelineStats.FailedTasks, pipelineStats.RunningTasks)
			
			// 总体统计
			totalTasks := scheduler.ListTasks()
			fmt.Printf("当前队列中任务总数: %d\n", len(totalTasks))
		}
	}
}

// RealWorldScenario 演示真实世界场景的使用
func RealWorldScenario() {
	// 创建适用于生产环境的混合调度器
	scheduler := NewHybridTaskScheduler(
		WithHybridWorkerCount(20, 3, 5),  // 生产环境更多worker
		WithHybridMaxConcurrentTasks(100), // 更高的并发限制
	)

	// 注册生产环境任务策略
	scheduler.RegisterTaskPolicy("user-notification", PolicyConcurrent)    // 用户通知可以并发
	scheduler.RegisterTaskPolicy("email-sending", PolicyConcurrent)        // 邮件发送可以并发
	scheduler.RegisterTaskPolicy("report-generation", PolicyPipeline)      // 报告生成用流水线
	scheduler.RegisterTaskPolicy("data-export", PolicySequential)          // 数据导出需要顺序
	scheduler.RegisterTaskPolicy("system-backup", PolicySequential)        // 系统备份需要顺序

	ctx := context.Background()
	scheduler.Start(ctx)
	defer scheduler.Close(ctx)

	// 模拟生产环境的任务负载
	// 用户通知任务（高频，可并发）
	for i := 0; i < 1000; i++ {
		task := NewTaskBuilder().
			WithType("user-notification").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				// 发送用户通知
				time.Sleep(10 * time.Millisecond)
				return nil
			}).
			Build()
		scheduler.Schedule(task)
	}

	// 报告生成任务（中等频率，流水线处理）
	for i := 0; i < 50; i++ {
		task := NewTaskBuilder().
			WithType("report-generation").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				// 生成报告
				time.Sleep(200 * time.Millisecond)
				return nil
			}).
			Build()
		scheduler.Schedule(task)
	}

	// 数据导出任务（低频，顺序执行）
	for i := 0; i < 10; i++ {
		task := NewTaskBuilder().
			WithType("data-export").
			Immediate().
			WithHandler(func(ctx context.Context) error {
				// 导出数据
				time.Sleep(500 * time.Millisecond)
				return nil
			}).
			Build()
		scheduler.Schedule(task)
	}

	// 等待所有任务完成
	time.Sleep(30 * time.Second)

	// 最终统计报告
	finalStats := scheduler.GetStats()
	fmt.Println("\n=== 最终执行报告 ===")
	for policy, stats := range finalStats {
		fmt.Printf("%s: 总任务=%d, 完成=%d, 失败=%d\n",
			policy, stats.TotalTasks, stats.CompletedTasks, stats.FailedTasks)
	}
}