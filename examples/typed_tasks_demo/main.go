package main

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
	fmt.Println("=== RunTypedTask完整方法集示例 ===\n")

	// 1. 创建混合调度器应用
	fmt.Println("1. 创建混合调度器应用:")
	app := bootstrap.NewApp(
		bootstrap.AdvancedTasks(
			bootstrap.WithSequentialType("email", "report"),     // 顺序执行
			bootstrap.WithConcurrentType("notification"),        // 并发执行
			bootstrap.WithPipelineType("cleanup"),               // 流水线执行
			bootstrap.WithConcurrentWorkers(5),                  // 5个并发工作协程
		),
	)

	fmt.Println("   ✓ 任务类型策略注册完成")
	fmt.Println("     - email: 顺序执行")
	fmt.Println("     - report: 顺序执行") 
	fmt.Println("     - notification: 并发执行")
	fmt.Println("     - cleanup: 流水线执行\n")

	// 2. RunTypedTask - 立即执行
	fmt.Println("2. RunTypedTask - 立即执行:")
	emailTaskID := app.RunTypedTask("email", func(ctx context.Context) error {
		fmt.Println("   📧 执行邮件任务")
		return nil
	})
	notificationTaskID := app.RunTypedTask("notification", func(ctx context.Context) error {
		fmt.Println("   🔔 执行通知任务")
		return nil
	})
	fmt.Printf("   ✓ 邮件任务ID: %s\n", emailTaskID)
	fmt.Printf("   ✓ 通知任务ID: %s\n\n", notificationTaskID)

	// 3. RunTypedTaskAt - 定时执行
	fmt.Println("3. RunTypedTaskAt - 定时执行:")
	futureTime := time.Now().Add(2 * time.Second)
	reportTaskID := app.RunTypedTaskAt("report", futureTime, func(ctx context.Context) error {
		fmt.Println("   📊 执行定时报告任务")
		return nil
	})
	fmt.Printf("   ✓ 报告任务将在 %s 执行\n", futureTime.Format("15:04:05"))
	fmt.Printf("   ✓ 报告任务ID: %s\n\n", reportTaskID)

	// 4. RunTypedTaskRecurring - 周期性执行
	fmt.Println("4. RunTypedTaskRecurring - 周期性执行:")
	cleanupTaskID := app.RunTypedTaskRecurring("cleanup", 1*time.Second, func(ctx context.Context) error {
		fmt.Println("   🧹 执行清理任务")
		return nil
	})
	fmt.Printf("   ✓ 清理任务每1秒执行一次\n")
	fmt.Printf("   ✓ 清理任务ID: %s\n\n", cleanupTaskID)

	// 5. 等待观察执行情况
	fmt.Println("5. 观察任务执行情况 (等待5秒):")
	time.Sleep(5 * time.Second)

	// 6. 任务管理
	fmt.Println("\n6. 任务管理:")
	fmt.Printf("   当前任务数量: %d\n", len(app.Scheduler.ListTasks()))
	
	if status, err := app.Scheduler.GetTaskStatus(cleanupTaskID); err == nil {
		fmt.Printf("   清理任务状态: %s\n", status)
	}

	// 清理资源
	app.CleanUp()
	fmt.Println("\n✅ 示例完成，资源已清理")
}