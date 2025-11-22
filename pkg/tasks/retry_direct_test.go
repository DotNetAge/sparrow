package tasks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMemorySchedulerRetry 直接测试内存调度器的重试功能
func TestMemorySchedulerRetry(t *testing.T) {
	// 创建内存调度器
	scheduler := NewMemoryTaskScheduler()
	defer scheduler.Close(context.Background())

	// 启动调度器
	if err := scheduler.Start(context.Background()); err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}

	// 计数器，用于模拟失败
	var attempts int

	// 创建一个前两次执行会失败，第三次成功的任务
	task := NewTaskBuilder().
		WithHandler(func(ctx context.Context) error {
			attempts++
			fmt.Printf("直接测试 - 任务执行第 %d 次\n", attempts)
			if attempts <= 2 {
				return fmt.Errorf("模拟失败 %d", attempts)
			}
			return nil
		}).
		WithRetry(3). // 设置最大重试次数为3次
		Build()

	// 调度任务
	err := scheduler.Schedule(task)
	assert.NoError(t, err)

	// 等待足够长的时间，确保重试完成
	fmt.Println("直接测试 - 等待任务重试...")
	time.Sleep(10 * time.Second)

	// 验证任务尝试了3次
	fmt.Printf("直接测试 - 最终尝试次数: %d\n", attempts)
	assert.Equal(t, 3, attempts)
}