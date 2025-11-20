package main

import (
	"fmt"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
	fmt.Println("=== 高级任务系统Option模式示例 ===\n")

	// 示例1：最简单的配置 - 使用所有默认值
	fmt.Println("1. 最简单的配置（使用默认值）:")
	app1 := bootstrap.NewApp()
	app1.Use(bootstrap.AdvancedTasks())
	fmt.Println("   ✓ 默认配置: 5个并发worker, 1个顺序worker, 1个流水线worker")
	fmt.Println("   ✓ 最大并发任务数: 10")
	fmt.Println()

	// 示例2：只修改工作协程数
	fmt.Println("2. 只修改工作协程数:")
	app2 := bootstrap.NewApp()
	app2.Use(bootstrap.AdvancedTasks(
		bootstrap.WithConcurrentWorkers(10),
		bootstrap.EnableSequentialExecution(),
		bootstrap.EnablePipelineExecution(),
	))
	fmt.Println("   ✓ 自定义: 10个并发worker, 启用顺序执行, 启用流水线执行")
	fmt.Println("   ✓ 其他参数使用默认值")
	fmt.Println()

	// 示例3：使用新的简洁API指定任务策略
	fmt.Println("3. 使用新的简洁API指定任务策略:")
	app3 := bootstrap.NewApp()
	app3.Use(bootstrap.AdvancedTasks(
		bootstrap.WithSequentialType("email", "report"),      // 顺序执行
		bootstrap.WithConcurrentType("notification"),        // 并发执行
		bootstrap.WithPipelineType("report"),                 // 流水线执行
	))
	fmt.Println("   ✓ 邮件任务: 顺序执行")
	fmt.Println("   ✓ 通知任务: 并发执行")
	fmt.Println("   ✓ 报告任务: 流水线执行")
	fmt.Println()

	// 示例4：批量添加任务策略
	fmt.Println("4. 批量添加任务策略:")
	app4 := bootstrap.NewApp()
	app4.Use(bootstrap.AdvancedTasks(
		bootstrap.WithSequentialType("data_import", "cleanup"),  // 顺序执行
		bootstrap.WithConcurrentType("image_process"),          // 并发执行
		bootstrap.WithPipelineType("analytics"),                 // 流水线执行
	))
	fmt.Println("   ✓ 批量配置了4种任务类型的执行策略")
	fmt.Println()

	// 示例5：完整配置
	fmt.Println("5. 完整配置:")
	app5 := bootstrap.NewApp()
	app5.Use(bootstrap.AdvancedTasks(
		bootstrap.WithConcurrentWorkers(15),
		bootstrap.EnableSequentialExecution(),
		bootstrap.EnablePipelineExecution(),
		bootstrap.WithAdvancedMaxConcurrentTasks(50),
		bootstrap.WithSequentialType("email", "data_import"),  // 顺序执行
		bootstrap.WithConcurrentType("notification", "image_process"), // 并发执行
		bootstrap.WithPipelineType("report"),                   // 流水线执行
	))
	fmt.Println("   ✓ 15个并发worker, 启用顺序执行, 启用流水线执行")
	fmt.Println("   ✓ 最大并发任务数: 50")
	fmt.Println("   ✓ 配置了5种任务类型的执行策略")
	fmt.Println()

	// 示例6：最新API的最佳实践
	fmt.Println("6. 最新API的最佳实践:")
	fmt.Println("   推荐写法（简洁优雅）:")
	fmt.Println("   bootstrap.AdvancedTasks(")
	fmt.Println("       bootstrap.WithConcurrentWorkers(10),")
	fmt.Println("       bootstrap.EnableSequentialExecution(),")
	fmt.Println("       bootstrap.EnablePipelineExecution(),")
	fmt.Println("       bootstrap.WithMaxConcurrentTasks(20),")
	fmt.Println("       bootstrap.WithSequentialType('email', 'report'),")
	fmt.Println("       bootstrap.WithConcurrentType('image', 'video'),")
	fmt.Println("       bootstrap.WithPipelineType('data_processing'),")
	fmt.Println("   )")
	fmt.Println()

	fmt.Println("=== Option模式的优势总结 ===")
	fmt.Println("✓ 默认值友好: 无需配置所有参数")
	fmt.Println("✓ 可读性强: 每个Option都有明确语义")
	fmt.Println("✓ 扩展性好: 新增Option不破坏现有代码")
	fmt.Println("✓ 组合灵活: 按需选择配置选项")
	fmt.Println("✓ 类型安全: 编译时检查参数类型")
	fmt.Println("✓ 自解释: 代码即文档")
}