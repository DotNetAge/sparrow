package main

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
	fmt.Println("=== 单一调度器执行模式示例 ===\n")

	// 示例1：并发执行模式（默认）
	fmt.Println("1. 并发执行模式示例:")
	concurrentApp := bootstrap.NewApp(
		bootstrap.Tasks(
			bootstrap.WithConcurrentMode(),
			bootstrap.WithWorkerCount(3),
		),
	)

	fmt.Println("   提交3个任务，将并发执行...")
	start := time.Now()
	
	taskIds := make([]string, 3)
	for i := 0; i < 3; i++ {
		index := i
		taskId := concurrentApp.RunTask(func(ctx context.Context) error {
			fmt.Printf("   并发任务 %d 开始执行\n", index+1)
			time.Sleep(500 * time.Millisecond) // 模拟任务执行
			fmt.Printf("   并发任务 %d 完成\n", index+1)
			return nil
		})
		taskIds[i] = taskId
	}
	
	// 等待所有任务完成
	time.Sleep(600 * time.Millisecond)
	fmt.Printf("   ✓ 总耗时: %v (并发执行)\n\n", time.Since(start))

	// 示例2：顺序执行模式
	fmt.Println("2. 顺序执行模式示例:")
	sequentialApp := bootstrap.NewApp(
		bootstrap.Tasks(
			bootstrap.WithSequentialMode(),
		),
	)

	fmt.Println("   提交3个任务，将按顺序执行...")
	start = time.Now()
	
	for i := 0; i < 3; i++ {
		index := i
		taskId := sequentialApp.RunTask(func(ctx context.Context) error {
			fmt.Printf("   顺序任务 %d 开始执行\n", index+1)
			time.Sleep(200 * time.Millisecond) // 模拟任务执行
			fmt.Printf("   顺序任务 %d 完成\n", index+1)
			return nil
		})
		taskIds[i] = taskId
	}
	
	// 等待所有任务完成
	time.Sleep(700 * time.Millisecond)
	fmt.Printf("   ✓ 总耗时: %v (顺序执行)\n\n", time.Since(start))

	// 示例3：流水线执行模式
	fmt.Println("3. 流水线执行模式示例:")
	pipelineApp := bootstrap.NewApp(
		bootstrap.Tasks(
			bootstrap.WithPipelineMode(),
		),
	)

	fmt.Println("   提交3个任务，将按流水线方式执行...")
	start = time.Now()
	
	for i := 0; i < 3; i++ {
		index := i
		taskId := pipelineApp.RunTask(func(ctx context.Context) error {
			fmt.Printf("   流水线任务 %d 开始执行\n", index+1)
			time.Sleep(200 * time.Millisecond) // 模拟任务执行
			fmt.Printf("   流水线任务 %d 完成\n", index+1)
			return nil
		})
		taskIds[i] = taskId
	}
	
	// 等待所有任务完成
	time.Sleep(700 * time.Millisecond)
	fmt.Printf("   ✓ 总耗时: %v (流水线执行)\n\n", time.Since(start))

	// 示例4：定时任务与执行模式
	fmt.Println("4. 定时任务与执行模式示例:")
	timedApp := bootstrap.NewApp(
		bootstrap.Tasks(
			bootstrap.WithSequentialMode(), // 顺序模式下的定时任务
		),
	)

	fmt.Println("   创建定时任务（2秒后执行）:")
	scheduledTime := time.Now().Add(2 * time.Second)
	timedTaskId := timedApp.RunTaskAt(scheduledTime, func(ctx context.Context) error {
		fmt.Println("   ✓ 定时任务执行完成")
		return nil
	})
	
	fmt.Printf("   ✓ 定时任务已创建，ID: %s\n", timedTaskId)
	fmt.Printf("   ✓ 将在 %s 执行\n", scheduledTime.Format("15:04:05"))
	
	// 等待定时任务执行
	time.Sleep(2500 * time.Millisecond)
	fmt.Println()

	// 示例5：重复任务与执行模式
	fmt.Println("5. 重复任务与执行模式示例:")
	recurringApp := bootstrap.NewApp(
		bootstrap.Tasks(
			bootstrap.WithConcurrentMode(), // 并发模式下的重复任务
		),
	)

	fmt.Println("   创建重复任务（每1秒执行一次）:")
	recurringTaskId := recurringApp.RunTaskRecurring(1*time.Second, func(ctx context.Context) error {
		fmt.Printf("   ✓ 重复任务执行 - %s\n", time.Now().Format("15:04:05"))
		return nil
	})
	
	fmt.Printf("   ✓ 重复任务已创建，ID: %s\n", recurringTaskId)
	fmt.Println("   ✓ 每1秒执行一次，观察3次执行...")
	
	// 观察重复任务执行3次
	time.Sleep(3500 * time.Millisecond)
	fmt.Println()

	fmt.Println("=== 执行模式对比总结 ===")
	fmt.Println("并发模式:")
	fmt.Println("  ✓ 多个任务同时执行，充分利用系统资源")
	fmt.Println("  ✓ 适用于CPU密集型或IO密集型任务")
	fmt.Println("  ✓ 总耗时约等于最长任务的耗时")
	fmt.Println()
	fmt.Println("顺序模式:")
	fmt.Println("  ✓ 任务严格按提交顺序执行")
	fmt.Println("  ✓ 适用于有依赖关系的任务")
	fmt.Println("  ✓ 总耗时等于所有任务耗时之和")
	fmt.Println()
	fmt.Println("流水线模式:")
	fmt.Println("  ✓ 任务按阶段串行执行")
	fmt.Println("  ✓ 适用于需要分阶段处理的任务")
	fmt.Println("  ✓ 总耗时等于所有任务耗时之和")
	fmt.Println()
	fmt.Println("=== 配置选项总结 ===")
	fmt.Println("✓ WithConcurrentMode() - 并发执行模式")
	fmt.Println("✓ WithSequentialMode() - 顺序执行模式") 
	fmt.Println("✓ WithPipelineMode() - 流水线执行模式")
	fmt.Println("✓ WithWorkerCount(n) - 设置工作协程数")
	fmt.Println("✓ WithMaxConcurrentTasks(n) - 设置最大并发任务数")
}