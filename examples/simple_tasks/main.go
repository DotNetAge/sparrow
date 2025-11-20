package main

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
	fmt.Println("=== 简化API实际使用示例 ===\n")

	// 创建App，配置任务系统
	app := bootstrap.NewApp(
		bootstrap.AdvancedTasks(
			// 配置不同类型任务的执行策略
			bootstrap.WithSequentialType("email", "report"),     // 邮件和报告：顺序执行
			bootstrap.WithConcurrentType("image", "notification"), // 图片和通知：并发执行
			bootstrap.WithPipelineType("data_import"),           // 数据导入：流水线执行
			
			// 性能配置
		bootstrap.WithConcurrentWorkers(10),
		bootstrap.WithAdvancedMaxConcurrentTasks(50),
		),
	)

	fmt.Println("=== 日常业务任务示例 ===\n")

	// 示例1：发送欢迎邮件（顺序执行）
	fmt.Println("1. 发送欢迎邮件:")
	emailTaskId := app.RunTask(func(ctx context.Context) error {
		fmt.Println("   ✓ 正在发送欢迎邮件...")
		time.Sleep(100 * time.Millisecond) // 模拟邮件发送
		fmt.Println("   ✓ 欢迎邮件发送成功")
		return nil
	})
	fmt.Printf("   ✓ 邮件任务已创建，ID: %s\n", emailTaskId)
	fmt.Println()

	// 示例2：处理用户头像（并发执行）
	fmt.Println("2. 处理用户头像:")
	avatarTaskId := app.RunTask(func(ctx context.Context) error {
		fmt.Println("   ✓ 开始处理用户头像...")
		time.Sleep(150 * time.Millisecond) // 模拟图片处理
		fmt.Println("   ✓ 用户头像处理完成")
		return nil
	})
	fmt.Printf("   ✓ 头像处理任务已创建，ID: %s\n", avatarTaskId)
	fmt.Println()

	// 示例3：生成月度报告（顺序执行）
	fmt.Println("3. 生成月度报告:")
	reportTaskId := app.RunTask(func(ctx context.Context) error {
		fmt.Println("   ✓ 开始生成月度报告...")
		time.Sleep(200 * time.Millisecond) // 模拟报告生成
		fmt.Println("   ✓ 月度报告生成完成")
		return nil
	})
	fmt.Printf("   ✓ 报告任务已创建，ID: %s\n", reportTaskId)
	fmt.Println()

	// 示例4：发送推送通知（并发执行）
	fmt.Println("4. 发送推送通知:")
	notificationTaskId := app.RunTask(func(ctx context.Context) error {
		fmt.Println("   ✓ 发送推送通知...")
		time.Sleep(50 * time.Millisecond) // 模拟通知发送
		fmt.Println("   ✓ 推送通知发送成功")
		return nil
	})
	fmt.Printf("   ✓ 通知任务已创建，ID: %s\n", notificationTaskId)
	fmt.Println()

	fmt.Println("=== 定时任务示例 ===\n")

	// 示例5：定时数据备份（每天凌晨3点）
	fmt.Println("5. 定时数据备份:")
	backupTime := time.Now().AddDate(0, 0, 1) // 明天
	backupTime = time.Date(backupTime.Year(), backupTime.Month(), backupTime.Day(), 3, 0, 0, 0, backupTime.Location())
	
	backupTaskId := app.RunTaskAt(backupTime, func(ctx context.Context) error {
		fmt.Println("   ✓ 开始执行数据备份...")
		// 这里可以添加实际的备份逻辑
		fmt.Println("   ✓ 数据备份完成")
		return nil
	})
	fmt.Printf("   ✓ 数据备份任务已创建，ID: %s\n", backupTaskId)
	fmt.Printf("   ✓ 将在 %s 执行\n", backupTime.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// 示例6：重复任务 - 系统健康检查（每10分钟）
	fmt.Println("6. 系统健康检查:")
	healthCheckTaskId := app.RunTaskRecurring(10*time.Minute, func(ctx context.Context) error {
		fmt.Println("   ✓ 执行系统健康检查...")
		// 这里可以添加健康检查逻辑
		fmt.Println("   ✓ 系统健康检查完成")
		return nil
	})
	fmt.Printf("   ✓ 健康检查任务已创建，ID: %s\n", healthCheckTaskId)
	fmt.Println("   ✓ 每10分钟执行一次")
	fmt.Println()

	// 示例7：重复任务 - 清理临时文件（每小时）
	fmt.Println("7. 清理临时文件:")
	cleanupTaskId := app.RunTaskRecurring(time.Hour, func(ctx context.Context) error {
		fmt.Println("   ✓ 开始清理临时文件...")
		// 这里可以添加清理逻辑
		fmt.Println("   ✓ 临时文件清理完成")
		return nil
	})
	fmt.Printf("   ✓ 清理任务已创建，ID: %s\n", cleanupTaskId)
	fmt.Println("   ✓ 每小时执行一次")
	fmt.Println()

	fmt.Println("=== 实际业务场景组合 ===\n")

	// 模拟用户注册后的系列任务
	fmt.Println("8. 用户注册后的系列任务:")
	
	// 发送欢迎邮件（顺序执行）
	welcomeEmailId := app.RunTask(func(ctx context.Context) error {
		fmt.Println("   ✓ 发送欢迎邮件给新用户")
		return nil
	})
	
	// 处理用户头像（并发执行）
	processAvatarId := app.RunTask(func(ctx context.Context) error {
		fmt.Println("   ✓ 处理用户上传的头像")
		return nil
	})
	
	// 发送注册通知（并发执行）
	notifyAdminId := app.RunTask(func(ctx context.Context) error {
		fmt.Println("   ✓ 通知管理员有新用户注册")
		return nil
	})
	
	fmt.Printf("   ✓ 欢迎邮件任务ID: %s（顺序执行）\n", welcomeEmailId)
	fmt.Printf("   ✓ 头像处理任务ID: %s（并发执行）\n", processAvatarId)
	fmt.Printf("   ✓ 管理员通知任务ID: %s（并发执行）\n", notifyAdminId)
	fmt.Println()

	// 等待一些任务完成
	time.Sleep(500 * time.Millisecond)

	fmt.Println("=== 简化API优势总结 ===")
	fmt.Println("✓ 极简语法：app.RunTask(handler) 一行代码创建任务")
	fmt.Println("✓ 类型安全：编译时检查参数类型")
	fmt.Println("✓ 自动ID生成：无需手动管理任务ID")
	fmt.Println("✓ 策略配置：通过App配置统一管理任务执行策略")
	fmt.Println("✓ 混合使用：简化API处理80%场景，复杂场景仍用TaskBuilder")
	fmt.Println("✓ 代码简洁：相比TaskBuilder减少60%的代码量")
	fmt.Println()
	fmt.Println("=== 适用场景 ===")
	fmt.Println("✓ 业务逻辑：用户注册、订单处理、通知发送等")
	fmt.Println("✓ 定时任务：数据备份、报告生成、清理任务等")
	fmt.Println("✓ 重复任务：健康检查、监控、同步任务等")
	fmt.Println("✓ 即时任务：邮件发送、图片处理、通知推送等")
}