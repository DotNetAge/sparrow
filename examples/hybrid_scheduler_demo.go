package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/DotNetAge/sparrow/pkg/tasks"
)

func main() {
	fmt.Println("=== 混合任务调度器演示 ===")
	
	// 创建混合调度器
	scheduler := tasks.NewHybridTaskScheduler(
		tasks.WithHybridWorkerCount(5, 2, 2),  // 并发5个worker，顺序2个，流水线2个
		tasks.WithHybridMaxConcurrentTasks(20), // 最大并发任务数
	)

	// 注册任务策略
	registerTaskPolicies(scheduler)

	// 启动调度器
	ctx := context.Background()
	if err := scheduler.Start(ctx); err != nil {
		log.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(ctx)

	fmt.Println("调度器已启动，开始提交任务...")

	// 模拟高频任务提交
	simulateHighFrequencyTasks(scheduler)

	// 监控调度器状态
	monitorSchedulerStatus(scheduler, 10*time.Second)

	fmt.Println("演示完成")
}

// registerTaskPolicies 注册不同任务类型的执行策略
func registerTaskPolicies(scheduler *tasks.HybridTaskScheduler) {
	// 数据处理类任务 - 并发执行
	scheduler.RegisterTaskPolicy("data-analysis", tasks.PolicyConcurrent)
	scheduler.RegisterTaskPolicy("image-processing", tasks.PolicyConcurrent)
	scheduler.RegisterTaskPolicy("video-encoding", tasks.PolicyConcurrent)

	// 文件操作类任务 - 顺序执行
	scheduler.RegisterTaskPolicy("file-write", tasks.PolicySequential)
	scheduler.RegisterTaskPolicy("database-backup", tasks.PolicySequential)
	scheduler.RegisterTaskPolicy("log-rotation", tasks.PolicySequential)

	// 流水线处理类任务 - 流水线执行
	scheduler.RegisterTaskPolicy("data-pipeline", tasks.PolicyPipeline)
	scheduler.RegisterTaskPolicy("etl-process", tasks.PolicyPipeline)
	scheduler.RegisterTaskPolicy("report-generation", tasks.PolicyPipeline)

	fmt.Println("任务策略注册完成")
}

// simulateHighFrequencyTasks 模拟高频任务提交场景
func simulateHighFrequencyTasks(scheduler *tasks.HybridTaskScheduler) {
	// 模拟高频数据分析和图片处理任务
	go func() {
		for i := 0; i < 50; i++ {
			// 数据分析任务
			dataTask := tasks.NewTaskBuilder().
				WithType("data-analysis").
				Immediate().
				WithHandler(func(ctx context.Context) error {
					// 模拟数据分析处理
					time.Sleep(50 * time.Millisecond)
					fmt.Printf("✓ 完成数据分析任务 %d\n", i)
					return nil
				}).
				Build()
			scheduler.Schedule(dataTask)

			// 图片处理任务
			imageTask := tasks.NewTaskBuilder().
				WithType("image-processing").
				Immediate().
				WithHandler(func(ctx context.Context) error {
					// 模拟图片处理
					time.Sleep(80 * time.Millisecond)
					fmt.Printf("✓ 完成图片处理任务 %d\n", i)
					return nil
				}).
				Build()
			scheduler.Schedule(imageTask)

			// 模拟高频提交（每20ms提交一批任务）
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// 模拟文件写入任务（需要顺序执行）
	go func() {
		for i := 0; i < 10; i++ {
			fileTask := tasks.NewTaskBuilder().
				WithType("file-write").
				Immediate().
				WithHandler(func(ctx context.Context) error {
					// 模拟文件写入操作
					time.Sleep(30 * time.Millisecond)
					fmt.Printf("✓ 完成文件写入任务 %d\n", i)
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
		for i := 0; i < 15; i++ {
			pipelineTask := tasks.NewTaskBuilder().
				WithType("data-pipeline").
				Immediate().
				WithHandler(func(ctx context.Context) error {
					// 模拟流水线处理
					time.Sleep(40 * time.Millisecond)
					fmt.Printf("✓ 完成流水线处理任务 %d\n", i)
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
func monitorSchedulerStatus(scheduler *tasks.HybridTaskScheduler, duration time.Duration) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(duration)
	
	fmt.Println("\n开始监控调度器状态...")
	
	for {
		select {
		case <-ticker.C:
			stats := scheduler.GetStats()
			
			fmt.Println("\n=== 调度器状态报告 ===")
			
			// 并发执行统计
			concurrentStats := stats[tasks.PolicyConcurrent]
			fmt.Printf("并发执行: 总任务=%d, 完成=%d, 失败=%d, 运行中=%d\n",
				concurrentStats.TotalTasks, concurrentStats.CompletedTasks,
				concurrentStats.FailedTasks, concurrentStats.RunningTasks)
			
			// 顺序执行统计
			sequentialStats := stats[tasks.PolicySequential]
			fmt.Printf("顺序执行: 总任务=%d, 完成=%d, 失败=%d, 运行中=%d\n",
				sequentialStats.TotalTasks, sequentialStats.CompletedTasks,
				sequentialStats.FailedTasks, sequentialStats.RunningTasks)
			
			// 流水线执行统计
			pipelineStats := stats[tasks.PolicyPipeline]
			fmt.Printf("流水线执行: 总任务=%d, 完成=%d, 失败=%d, 运行中=%d\n",
				pipelineStats.TotalTasks, pipelineStats.CompletedTasks,
				pipelineStats.FailedTasks, pipelineStats.RunningTasks)
			
			// 总体统计
			totalTasks := scheduler.ListTasks()
			fmt.Printf("当前队列中任务总数: %d\n", len(totalTasks))
			
		case <-timeout:
			fmt.Println("\n=== 最终统计报告 ===")
			finalStats := scheduler.GetStats()
			for policy, stats := range finalStats {
				fmt.Printf("%s: 总任务=%d, 完成=%d, 失败=%d\n",
					policy, stats.TotalTasks, stats.CompletedTasks, stats.FailedTasks)
			}
			return
		}
	}
}