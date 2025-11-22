package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
	"github.com/DotNetAge/sparrow/pkg/tasks"
)

func main() {
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 使用混合任务系统配置，确保正确启用顺序执行模式
	// 关键修复：直接使用HybridTasks配置，确保顺序执行器有正确的工作协程数
	app := bootstrap.NewApp(
		bootstrap.HybridTasks(),
		bootstrap.WithRetry(),
	)
	
	// 确保任务类型被正确注册到顺序执行策略
	// 直接从调度器实例获取混合调度器并注册任务策略
	if hybridScheduler, ok := app.Scheduler.Instance.(*tasks.HybridTaskScheduler); ok {
		hybridScheduler.RegisterTaskPolicy("sequential-task", tasks.PolicySequential)
		fmt.Println("任务类型 'sequential-task' 已正确注册到顺序执行策略")
	}

	// 用于记录执行次数
	var executionCount int
	var mu sync.Mutex
	var lastExecutionTime time.Time
	const maxExecutions = 5                    // 设置最大执行次数，确保至少执行3次
	const executionInterval = 10 * time.Second // 10秒间隔

	fmt.Println("循环任务演示程序已启动...")
	fmt.Printf("将每%d秒执行一次循环任务并输出信息，共执行%d次\n", executionInterval/time.Second, maxExecutions)
	fmt.Println("按 Ctrl+C 可以提前退出程序")

	// 创建循环任务，每10秒执行一次，使用顺序执行模式
	// 关键修复：使用正确的API并确保任务类型正确映射到顺序执行策略
	taskID, err := app.Scheduler.RunTypedTaskRecurring("sequential-task", executionInterval, func(ctx context.Context) error {
		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return ctx.Err()
		}

		now := time.Now()
		mu.Lock()
		prevTime := lastExecutionTime
		lastExecutionTime = now
		executionCount++
		count := executionCount
		mu.Unlock()

		fmt.Printf("\n[重要] 循环任务执行时间: %s\n", now.Format(time.RFC3339))
		fmt.Printf("[重要] 执行次数: %d/%d\n", count, maxExecutions)

		if count > 1 {
			elapsed := now.Sub(prevTime)
			fmt.Printf("[重要] 与上次执行的时间间隔: %.2f 秒\n", elapsed.Seconds())
		}

		// 当达到最大执行次数时，取消上下文并返回错误以阻止任务重新调度
		if count >= maxExecutions {
			fmt.Printf("\n[重要] 已达到最大执行次数 %d，程序将在1秒后退出\n", maxExecutions)
			// 使用非阻塞方式取消上下文
			go func() {
				time.Sleep(1 * time.Second)
				cancel()
			}()
			// 返回错误以阻止任务重新调度
			return fmt.Errorf("reached max execution count")
		}

		return nil
	})

	if err != nil {
		log.Fatalf("创建循环任务失败: %v", err)
	}

	fmt.Printf("循环任务已创建，ID: %s，间隔: %d秒\n", taskID, executionInterval/time.Second)
	fmt.Printf("等待任务执行 %d 次...\n", maxExecutions)

	// 启动应用程序
	go func() {
		if err := app.Start(); err != nil {
			log.Printf("应用程序退出: %v", err)
		}
	}()

	// 等待上下文取消或信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		fmt.Println("\n任务完成，正在停止程序...")
	case <-sigChan:
		fmt.Println("\n接收到退出信号，正在停止程序...")
		cancel()
	}

	// 给调度器一点时间来清理
	time.Sleep(1 * time.Second)
	fmt.Println("程序已退出")
}
