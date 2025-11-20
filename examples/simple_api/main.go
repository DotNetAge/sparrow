package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
	fmt.Println("=== 简洁API示例 - 符合Go哲学 ===\n")

	// 示例1：最简洁的单一调度器用法
	fmt.Println("1. 最简洁用法 - 单一调度器:")
	app1 := bootstrap.NewApp(bootstrap.Tasks(
		bootstrap.WithWorkerCount(3),           // 3个工作协程
		bootstrap.WithMaxConcurrentTasks(10),   // 最大10个并发任务
	))

	if err := app1.Start(); err != nil {
		log.Fatalf("启动应用失败: %v", err)
	}
	defer app1.CleanUp()

	// 使用最简洁的app.RunTask方式
	taskID1 := app1.RunTask(func(ctx context.Context) error {
		fmt.Println("   执行任务1：发送邮件")
		return nil
	})

	taskID2 := app1.RunTask(func(ctx context.Context) error {
		fmt.Println("   执行任务2：处理图片")
		return nil
	})

	fmt.Printf("   ✓ 任务已提交: %s, %s\n\n", taskID1, taskID2)

	// 示例2：混合调度器 - 简洁的类型化任务
	fmt.Println("2. 混合调度器 - 类型化任务:")
	app2 := bootstrap.NewApp(bootstrap.AdvancedTasks(
		bootstrap.WithSequentialType("email", "report"),     // 顺序执行
		bootstrap.WithConcurrentType("image", "video"),       // 并发执行
		bootstrap.WithPipelineType("data_processing"),        // 流水线执行
		bootstrap.WithConcurrentWorkers(5),                   // 5个并发工作协程
	))

	if err := app2.Start(); err != nil {
		log.Fatalf("启动应用失败: %v", err)
	}
	defer app2.CleanUp()

	// 使用简洁的app.RunTypedTask方式
	emailTaskID := app2.RunTypedTask("email", func(ctx context.Context) error {
		fmt.Println("   执行邮件任务（顺序执行）")
		return nil
	})

	imageTaskID := app2.RunTypedTask("image", func(ctx context.Context) error {
		fmt.Println("   执行图片任务（并发执行）")
		return nil
	})

	dataTaskID := app2.RunTypedTask("data_processing", func(ctx context.Context) error {
		fmt.Println("   执行数据处理任务（流水线执行）")
		return nil
	})

	fmt.Printf("   ✓ 类型化任务已提交: 邮件=%s, 图片=%s, 数据=%s\n\n", 
		emailTaskID, imageTaskID, dataTaskID)

	// 示例3：实际业务场景
	fmt.Println("3. 实际业务场景:")
	app3 := bootstrap.NewApp(bootstrap.AdvancedTasks(
		// 邮件相关：顺序执行，避免发送冲突
		bootstrap.WithSequentialType("email", "sms", "push_notification"),
		// 媒体处理：并发执行，提高处理速度
		bootstrap.WithConcurrentType("image_resize", "video_transcode", "audio_convert"),
		// 数据处理：流水线执行，支持分阶段处理
		bootstrap.WithPipelineType("data_import", "report_generation", "backup_process"),
		// 性能配置
		bootstrap.WithConcurrentWorkers(10),
		bootstrap.WithAdvancedMaxConcurrentTasks(50),
	))

	if err := app3.Start(); err != nil {
		log.Fatalf("启动应用失败: %v", err)
	}
	defer app3.CleanUp()

	// 批量提交业务任务
	businessTasks := []struct {
		taskType string
		name     string
		handler  func(ctx context.Context) error
	}{
		{"email", "发送欢迎邮件", func(ctx context.Context) error {
			fmt.Println("   发送欢迎邮件给新用户")
			return nil
		}},
		{"image_resize", "调整用户头像", func(ctx context.Context) error {
			fmt.Println("   调整用户头像尺寸")
			return nil
		}},
		{"data_import", "导入用户数据", func(ctx context.Context) error {
			fmt.Println("   导入用户数据到系统")
			return nil
		}},
	}

	for _, task := range businessTasks {
		taskID := app3.RunTypedTask(task.taskType, task.handler)
		fmt.Printf("   ✓ %s任务已提交: %s\n", task.name, taskID)
	}

	// 等待任务执行完成
	time.Sleep(2 * time.Second)

	// 示例4：任务管理
	fmt.Println("\n4. 任务管理:")
	// 查询任务状态
	if status, err := app3.Scheduler.GetTaskStatus(emailTaskID); err == nil {
		fmt.Printf("   邮件任务状态: %s\n", status)
	}

	// 列出所有任务
	tasks := app3.Scheduler.ListTasks()
	fmt.Printf("   当前任务总数: %d\n", len(tasks))

	// 动态调整配置
	if err := app3.Scheduler.SetMaxConcurrentTasks(100); err == nil {
		fmt.Println("   ✓ 最大并发任务数已调整为100")
	}

	fmt.Println("\n=== 简洁API优势 ===")
	fmt.Println("✓ 符合Go简洁哲学：一行代码完成任务提交")
	fmt.Println("✓ 自动处理ID生成：无需手动管理任务ID")
	fmt.Println("✓ 类型安全：编译时检查参数类型")
	fmt.Println("✓ 语义清晰：函数名直接表达意图")
	fmt.Println("✓ 减少样板代码：专注业务逻辑")

	fmt.Println("\n=== 推荐使用方式 ===")
	fmt.Println("// ✅ 推荐 - 最简洁")
	fmt.Println(`taskID := app.RunTask(func(ctx context.Context) error {`)
	fmt.Println(`    return doSomething()`)
	fmt.Println(`})`)
	fmt.Println("")
	fmt.Println("// ✅ 推荐 - 类型化任务")
	fmt.Println(`taskID := app.RunTypedTask("email", func(ctx context.Context) error {`)
	fmt.Println(`    return sendEmail()`)
	fmt.Println(`})`)

	fmt.Println("\n=== 避免的复杂写法 ===")
	fmt.Println("// ❌ 避免 - 除非有特殊需求")
	fmt.Println(`task := tasks.NewTaskBuilder().`)
	fmt.Println(`    WithID("task-1").`)
	fmt.Println(`    WithHandler(func(ctx context.Context) error {`)
	fmt.Println(`        return doSomething()`)
	fmt.Println(`    }).`)
	fmt.Println(`    WithImmediateExecution().`)
	fmt.Println(`    Build()`)
	fmt.Println("")
	fmt.Println(`if err := app.Scheduler.Schedule(task); err != nil {`)
	fmt.Println(`    log.Printf("提交任务失败: %v", err)`)
	fmt.Println(`}`)
}
