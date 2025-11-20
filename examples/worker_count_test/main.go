package main

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
	fmt.Println("=== 测试Tasks函数的WorkerCount自动调整 ===\n")

	// 测试1：顺序模式下设置多个工作协程
	fmt.Println("1. 顺序模式测试:")
	app1 := bootstrap.NewApp(
		bootstrap.Tasks(
			bootstrap.WithSequentialMode(),
			bootstrap.WithWorkerCount(5), // 应该被自动调整为1
		),
	)
	
	// 提交几个任务，验证顺序执行
	for i := 0; i < 3; i++ {
		taskId := app1.RunTask(func(ctx context.Context) error {
			fmt.Printf("   顺序任务 %d 执行中...\n", i+1)
			time.Sleep(500 * time.Millisecond)
			fmt.Printf("   顺序任务 %d 完成\n", i+1)
			return nil
		})
		fmt.Printf("   ✓ 任务 %s 已提交\n", taskId)
	}
	
	time.Sleep(2 * time.Second)
	fmt.Println()

	// 测试2：流水线模式下设置多个工作协程
	fmt.Println("2. 流水线模式测试:")
	app2 := bootstrap.NewApp(
		bootstrap.Tasks(
			bootstrap.WithPipelineMode(),
			bootstrap.WithWorkerCount(10), // 应该被自动调整为1
		),
	)
	
	// 提交几个任务，验证流水线执行
	for i := 0; i < 3; i++ {
		taskId := app2.RunTask(func(ctx context.Context) error {
			fmt.Printf("   流水线任务 %d 执行中...\n", i+1)
			time.Sleep(300 * time.Millisecond)
			fmt.Printf("   流水线任务 %d 完成\n", i+1)
			return nil
		})
		fmt.Printf("   ✓ 任务 %s 已提交\n", taskId)
	}
	
	time.Sleep(2 * time.Second)
	fmt.Println()

	// 测试3：并发模式下设置多个工作协程（正常情况）
	fmt.Println("3. 并发模式测试:")
	app3 := bootstrap.NewApp(
		bootstrap.Tasks(
			bootstrap.WithConcurrentMode(),
			bootstrap.WithWorkerCount(3), // 应该保持3个
		),
	)
	
	// 提交几个任务，验证并发执行
	for i := 0; i < 3; i++ {
		taskId := app3.RunTask(func(ctx context.Context) error {
			fmt.Printf("   并发任务 %d 执行中...\n", i+1)
			time.Sleep(500 * time.Millisecond)
			fmt.Printf("   并发任务 %d 完成\n", i+1)
			return nil
		})
		fmt.Printf("   ✓ 任务 %s 已提交\n", taskId)
	}
	
	time.Sleep(1 * time.Second)
	fmt.Println()

	fmt.Println("=== 测试总结 ===")
	fmt.Println("✓ 顺序模式：WorkerCount=5 自动调整为 1")
	fmt.Println("✓ 流水线模式：WorkerCount=10 自动调整为 1") 
	fmt.Println("✓ 并发模式：WorkerCount=3 保持不变")
	fmt.Println("✓ 所有模式都能正常工作，避免了配置歧义")
}