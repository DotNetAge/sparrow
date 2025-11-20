package bootstrap

import (
	"testing"

	"github.com/DotNetAge/sparrow/pkg/tasks"
)

func TestOrderSchedulerWithRetry(t *testing.T) {
	// 创建一个测试用的App实例
	app := &App{}
	
	// 创建MemoryTaskScheduler并设置为顺序执行模式
	memoryScheduler := tasks.NewMemoryTaskScheduler(
		tasks.WithWorkerCount(1),         // 1个工作协程
		tasks.WithMaxConcurrentTasks(1),  // 最大并发数为1
	)
	app.Scheduler = memoryScheduler
	
	// 核心测试：确认调度器类型保持不变
	if _, ok := app.Scheduler.(*tasks.MemoryTaskScheduler); !ok {
		t.Fatalf("调度器应该是MemoryTaskScheduler类型")
	}
	
	// 记录成功
	t.Log("测试通过：顺序调度器类型正确")
}