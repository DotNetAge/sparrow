package main

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
	"github.com/DotNetAge/sparrow/pkg/tasks"
)

func main() {
	fmt.Println("=== 单一调度器实用示例（简化版） ===\n")

	// 创建App和调度器
	app := bootstrap.NewApp(
		bootstrap.Tasks(), // 使用默认任务配置
	)
	scheduler := tasks.NewMemoryTaskScheduler()
	defer scheduler.Close(context.Background())

	// 示例1：即时任务 - 最简单的用法
	fmt.Println("1. 即时任务执行:")
	taskId1 := app.RunTask(func(ctx context.Context) error {
		fmt.Println("   ✓ 即时任务执行完成")
		return nil
	})
	fmt.Printf("   ✓ 任务已创建，ID: %s\n", taskId1)
	time.Sleep(100 * time.Millisecond)
	fmt.Println()

	// 示例2：定时任务 - 数据清理
	fmt.Println("2. 定时任务 - 数据清理:")
	// 明天凌晨2点执行（模拟定时任务）
	tomorrow2AM := time.Now().AddDate(0, 0, 1)
	tomorrow2AM = time.Date(tomorrow2AM.Year(), tomorrow2AM.Month(), tomorrow2AM.Day(), 2, 0, 0, 0, tomorrow2AM.Location())
	
	taskId2 := app.RunTaskAt(tomorrow2AM, func(ctx context.Context) error {
		fmt.Println("   ✓ 执行数据清理任务")
		// 这里可以添加实际的清理逻辑
		return nil
	})
	fmt.Printf("   ✓ 数据清理任务已调度，ID: %s\n", taskId2)
	fmt.Println("   ✓ 将在明天凌晨2点执行")
	fmt.Println()

	// 示例3：重复任务 - 健康检查
	fmt.Println("3. 重复任务 - 服务健康检查:")
	taskId3 := app.RunTaskRecurring(5*time.Minute, func(ctx context.Context) error {
		fmt.Println("   ✓ 执行健康检查")
		// 这里可以添加健康检查逻辑
		return nil
	})
	fmt.Printf("   ✓ 健康检查任务已调度，ID: %s\n", taskId3)
	fmt.Println("   ✓ 每5分钟执行一次")
	fmt.Println()

	// 示例4：带错误处理的任务（需要使用TaskBuilder）
	fmt.Println("4. 带错误处理的任务:")
	errorTask := tasks.NewTaskBuilder().
		Immediate().
		WithHandler(func(ctx context.Context) error {
			fmt.Println("   ✓ 执行可能失败的任务")
			// 模拟任务失败
			return fmt.Errorf("模拟任务失败")
		}).
		WithOnComplete(func(ctx context.Context, err error) {
			if err != nil {
				fmt.Printf("   ✗ 任务执行失败: %v\n", err)
			}
		}).
		Build()

	scheduler.Schedule(errorTask)
	time.Sleep(100 * time.Millisecond)
	fmt.Println()

	// 示例5：可取消的任务（需要使用TaskBuilder）
	fmt.Println("5. 可取消的任务:")
	longTask := tasks.NewTaskBuilder().
		Immediate().
		WithHandler(func(ctx context.Context) error {
			select {
			case <-time.After(5 * time.Second):
				fmt.Println("   ✓ 长时间任务完成")
				return nil
			case <-ctx.Done():
				fmt.Println("   ✓ 任务被取消")
				return ctx.Err()
			}
		}).
		WithOnCancel(func(ctx context.Context) {
			fmt.Println("   ✓ 任务取消回调执行")
		}).
		Build()

	scheduler.Schedule(longTask)
	// 1秒后取消任务
	time.Sleep(1 * time.Second)
	scheduler.Cancel(longTask.ID())
	time.Sleep(100 * time.Millisecond)
	fmt.Println()

	// 示例6：任务状态查询
	fmt.Println("6. 任务状态查询:")
	statusTask := tasks.NewTaskBuilder().
		Immediate().
		WithHandler(func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond) // 模拟任务执行时间
			fmt.Println("   ✓ 状态查询任务执行完成")
			return nil
		}).
		Build()

	// 调度前查询状态
	status, _ := scheduler.GetTaskStatus(statusTask.ID())
	fmt.Printf("   调度前状态: %s\n", status)

	scheduler.Schedule(statusTask)

	// 调度后查询状态
	status, _ = scheduler.GetTaskStatus(statusTask.ID())
	fmt.Printf("   调度后状态: %s\n", status)

	// 等待任务完成
	time.Sleep(300 * time.Millisecond)
	status, _ = scheduler.GetTaskStatus(statusTask.ID())
	fmt.Printf("   完成后状态: %s\n", status)
	fmt.Println()

	// 示例7：任务列表查询
	fmt.Println("7. 任务列表查询:")
	// 使用简化方法创建多个任务
	var taskIds []string
	for i := 0; i < 3; i++ {
		taskId := app.RunTask(func(ctx context.Context) error {
			return nil
		})
		taskIds = append(taskIds, taskId)
	}

	// 查询任务列表
	taskList := scheduler.ListTasks()
	fmt.Printf("   ✓ 总共创建了 %d 个任务\n", len(taskList))
	for i, task := range taskList {
		fmt.Printf("   %d. 任务ID: %s, 状态: %s\n", i+1, task.ID, task.Status)
	}
	fmt.Println()

	// 示例8：配置并发限制
	fmt.Println("8. 配置并发限制:")
	limitedScheduler := tasks.NewMemoryTaskScheduler(
		tasks.WithMaxConcurrentTasks(2), // 最多同时执行2个任务
	)
	defer limitedScheduler.Close(context.Background())

	// 创建5个任务，但只有2个能同时执行
	for i := 0; i < 5; i++ {
		taskIndex := i
		task := tasks.NewTaskBuilder().
			Immediate().
			WithHandler(func(ctx context.Context) error {
				fmt.Printf("   ✓ 任务 %d 开始执行\n", taskIndex)
				time.Sleep(200 * time.Millisecond) // 模拟任务执行时间
				fmt.Printf("   ✓ 任务 %d 执行完成\n", taskIndex)
				return nil
			}).
			Build()
		limitedScheduler.Schedule(task)
	}

	// 等待所有任务完成
	time.Sleep(1 * time.Second)
	fmt.Println()

	fmt.Println("=== 简化API使用总结 ===")
	fmt.Println("✓ app.RunTask(handler) - 立即执行任务")
	fmt.Println("✓ app.RunTaskAt(time, handler) - 定时执行任务") 
	fmt.Println("✓ app.RunTaskRecurring(duration, handler) - 重复执行任务")
	fmt.Println("✓ 复杂功能（错误处理、取消等）仍可使用TaskBuilder")
	fmt.Println("✓ 简化API覆盖80%的常用场景，代码更简洁")
}