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

	"github.com/DotNetAge/sparrow/pkg/tasks"
)

func main() {
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建调度器
	scheduler := tasks.NewMemoryTaskScheduler()
	defer scheduler.Close(ctx)

	// 创建包装器
	wrapper := tasks.NewSchedulerWrapper(scheduler, nil)

	// 启动调度器
	if err := scheduler.Start(ctx); err != nil {
		log.Fatalf("启动调度器失败: %v", err)
	}

	// 用于记录执行次数
	var executionCount int
	var mu sync.Mutex
	var lastExecutionTime time.Time
	const maxExecutions = 5 // 设置最大执行次数，确保至少执行3次
	const executionInterval = 10 * time.Second // 改为10秒间隔

	fmt.Println("循环任务演示程序已启动...")
	fmt.Printf("将每%d秒执行一次循环任务并输出信息，共执行%d次\n", executionInterval/time.Second, maxExecutions)
	fmt.Println("按 Ctrl+C 可以提前退出程序")

	// 创建循环任务，每10秒执行一次
	taskID, err := wrapper.RunTaskRecurring(executionInterval, func(ctx context.Context) error {
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
		
		// 当达到最大执行次数时，取消任务
		if count >= maxExecutions {
			fmt.Printf("\n[重要] 已达到最大执行次数 %d，程序将在1秒后退出\n", maxExecutions)
			go func() {
				time.Sleep(1 * time.Second)
				cancel()
			}()
		}
		
		return nil
	})

	if err != nil {
		log.Fatalf("创建循环任务失败: %v", err)
	}

	fmt.Printf("循环任务已创建，ID: %s，间隔: %d秒\n", taskID, executionInterval/time.Second)
	fmt.Printf("等待任务执行 %d 次...\n", maxExecutions)

	// 等待信号以优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-sigChan

	fmt.Println("\n接收到退出信号，正在停止程序...")
	cancel()

	// 给调度器一点时间来清理
	time.Sleep(1 * time.Second)
	fmt.Println("程序已退出")
}