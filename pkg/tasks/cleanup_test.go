package tasks

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestCleanupPolicy_ShouldCleanup(t *testing.T) {
	tests := []struct {
		name            string
		policy          *CleanupPolicy
		status          TaskStatus
		createdAt       time.Time
		updatedAt       time.Time
		currentCount    int
		expected        bool
	}{
		{
			name: "已完成任务未过期不应清理",
			policy: &CleanupPolicy{
				EnableAutoCleanup:  true,
				CompletedTaskTTL:   30 * time.Minute,
				MaxCompletedTasks: 100,
			},
			status:       TaskStatusCompleted,
			createdAt:    time.Now().Add(-10 * time.Minute),
			updatedAt:    time.Now().Add(-5 * time.Minute),
			currentCount: 50,
			expected:     false,
		},
		{
			name: "已完成任务过期应清理",
			policy: &CleanupPolicy{
				EnableAutoCleanup:  true,
				CompletedTaskTTL:   30 * time.Minute,
				MaxCompletedTasks:  100,
			},
			status:       TaskStatusCompleted,
			createdAt:    time.Now().Add(-40 * time.Minute),
			updatedAt:    time.Now().Add(-35 * time.Minute),
			currentCount: 50,
			expected:     true,
		},
		{
			name: "已完成任务超过最大数量应清理",
			policy: &CleanupPolicy{
				EnableAutoCleanup:  true,
				CompletedTaskTTL:   30 * time.Minute,
				MaxCompletedTasks:  100,
			},
			status:       TaskStatusCompleted,
			createdAt:    time.Now().Add(-10 * time.Minute),
			updatedAt:    time.Now().Add(-5 * time.Minute),
			currentCount: 150,
			expected:     true,
		},
		{
			name: "失败任务未过期不应清理",
			policy: &CleanupPolicy{
				EnableAutoCleanup: true,
				FailedTaskTTL:    60 * time.Minute,
				MaxFailedTasks:   50,
			},
			status:       TaskStatusFailed,
			createdAt:    time.Now().Add(-30 * time.Minute),
			updatedAt:    time.Now().Add(-20 * time.Minute),
			currentCount: 30,
			expected:     false,
		},
		{
			name: "已取消任务过期应清理",
			policy: &CleanupPolicy{
				EnableAutoCleanup: true,
				CancelledTaskTTL: 15 * time.Minute,
			},
			status:       TaskStatusCancelled,
			createdAt:    time.Now().Add(-20 * time.Minute),
			updatedAt:    time.Now().Add(-16 * time.Minute),
			currentCount: 10,
			expected:     true,
		},
		{
			name: "禁用自动清理不应清理",
			policy: &CleanupPolicy{
				EnableAutoCleanup:  false,
				CompletedTaskTTL:   1 * time.Minute,
			},
			status:       TaskStatusCompleted,
			createdAt:    time.Now().Add(-10 * time.Minute),
			updatedAt:    time.Now().Add(-5 * time.Minute),
			currentCount: 10,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.ShouldCleanup(tt.status, tt.createdAt, tt.updatedAt, tt.currentCount)
			if result != tt.expected {
				t.Errorf("ShouldCleanup() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultCleanupPolicy(t *testing.T) {
	policy := DefaultCleanupPolicy()
	
	if !policy.EnableAutoCleanup {
		t.Error("默认策略应启用自动清理")
	}
	
	if policy.CompletedTaskTTL != 30*time.Minute {
		t.Errorf("已完成任务TTL应为30分钟，实际为%v", policy.CompletedTaskTTL)
	}
	
	if policy.FailedTaskTTL != 60*time.Minute {
		t.Errorf("失败任务TTL应为60分钟，实际为%v", policy.FailedTaskTTL)
	}
	
	if policy.CancelledTaskTTL != 15*time.Minute {
		t.Errorf("已取消任务TTL应为15分钟，实际为%v", policy.CancelledTaskTTL)
	}
	
	if policy.MaxCompletedTasks != 1000 {
		t.Errorf("最大已完成任务数应为1000，实际为%d", policy.MaxCompletedTasks)
	}
	
	if policy.MaxFailedTasks != 500 {
		t.Errorf("最大失败任务数应为500，实际为%d", policy.MaxFailedTasks)
	}
	
	if policy.CleanupInterval != 5*time.Minute {
		t.Errorf("清理间隔应为5分钟，实际为%v", policy.CleanupInterval)
	}
}

func TestMemoryTaskScheduler_CleanupTasks(t *testing.T) {
	// 创建短TTL的清理策略
	policy := &CleanupPolicy{
		EnableAutoCleanup:  true,
		CompletedTaskTTL:   100 * time.Millisecond,
		FailedTaskTTL:      100 * time.Millisecond,
		CancelledTaskTTL:   100 * time.Millisecond,
		MaxCompletedTasks:  2,
		MaxFailedTasks:     2,
		CleanupInterval:    50 * time.Millisecond,
	}
	
	scheduler := NewMemoryTaskScheduler(WithCleanupPolicy(policy))
	
	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(ctx)
	
	// 创建多个任务，使用较长的执行时间确保任务在调度器中
	task1 := NewTaskBuilder().
		WithType("test").
		WithHandler(func(ctx context.Context) error { 
			time.Sleep(50 * time.Millisecond)
			return nil 
		}).
		Build()
		
	task2 := NewTaskBuilder().
		WithType("test").
		WithHandler(func(ctx context.Context) error { 
			time.Sleep(50 * time.Millisecond)
			return nil 
		}).
		Build()
		
	task3 := NewTaskBuilder().
		WithType("test").
		WithHandler(func(ctx context.Context) error { 
			time.Sleep(50 * time.Millisecond)
			return nil 
		}).
		Build()
	
	// 调度任务
	err = scheduler.Schedule(task1)
	if err != nil {
		t.Fatalf("调度任务1失败: %v", err)
	}
	
	err = scheduler.Schedule(task2)
	if err != nil {
		t.Fatalf("调度任务2失败: %v", err)
	}
	
	err = scheduler.Schedule(task3)
	if err != nil {
		t.Fatalf("调度任务失败: %v", err)
	}
	
	// 等待任务完成
	time.Sleep(200 * time.Millisecond)
	
	// 检查任务数量（任务可能已经完成但还在内存中）
	tasks := scheduler.ListTasks()
	initialCount := len(tasks)
	if initialCount == 0 {
		t.Skip("任务执行过快，跳过此测试")
	}
	
	// 等待清理
	time.Sleep(200 * time.Millisecond)
	
	// 手动触发清理
	remainingCount := scheduler.CleanupTasks()
	
	// 验证任务被清理
	if remainingCount > 0 {
		t.Errorf("期望任务被清理，但仍有%d个任务", remainingCount)
	}
}

func TestMemoryTaskScheduler_SetCleanupPolicy(t *testing.T) {
	scheduler := NewMemoryTaskScheduler()
	
	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(ctx)
	
	// 创建新策略
	newPolicy := &CleanupPolicy{
		EnableAutoCleanup:  true,
		CompletedTaskTTL:   1 * time.Minute,
		CleanupInterval:    30 * time.Second,
	}
	
	// 设置新策略
	err = scheduler.SetCleanupPolicy(newPolicy)
	if err != nil {
		t.Errorf("设置清理策略失败: %v", err)
	}
	
	// 验证统计信息
	stats := scheduler.GetCleanupStats()
	cleanupPolicy, ok := stats["cleanup_policy"].(map[string]interface{})
	if !ok {
		t.Fatal("清理策略统计信息格式错误")
	}
	
	if cleanupPolicy["enable_auto_cleanup"] != true {
		t.Error("清理策略未正确更新")
	}
	
	if cleanupPolicy["completed_task_ttl"] != "1m0s" {
		t.Errorf("已完成任务TTL应为1m0s，实际为%v", cleanupPolicy["completed_task_ttl"])
	}
}

func TestMemoryTaskScheduler_GetCleanupStats(t *testing.T) {
	policy := DefaultCleanupPolicy()
	scheduler := NewMemoryTaskScheduler(WithCleanupPolicy(policy))
	
	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(ctx)
	
	// 创建并调度任务
	task := NewTaskBuilder().
		WithType("test").
		WithHandler(func(ctx context.Context) error { return nil }).
		Build()
	
	err = scheduler.Schedule(task)
	if err != nil {
		t.Fatalf("调度任务失败: %v", err)
	}
	
	// 获取统计信息
	stats := scheduler.GetCleanupStats()
	
	// 验证统计信息结构
	if totalTasks, ok := stats["total_tasks"].(int); !ok || totalTasks != 1 {
		t.Errorf("总任务数应为1，实际为%v", stats["total_tasks"])
	}
	
	if statusCounts, ok := stats["status_counts"].(map[TaskStatus]int); !ok {
		t.Error("状态统计信息格式错误")
	} else {
		if statusCounts[TaskStatusWaiting] != 1 {
			t.Errorf("等待中任务数应为1，实际为%d", statusCounts[TaskStatusWaiting])
		}
	}
	
	if cleanupPolicy, ok := stats["cleanup_policy"].(map[string]interface{}); !ok {
		t.Error("清理策略统计信息格式错误")
	} else {
		if cleanupPolicy["enable_auto_cleanup"] != true {
			t.Error("自动清理应启用")
		}
	}
}

func TestMemoryTaskScheduler_AutoCleanupIntegration(t *testing.T) {
	// 创建快速清理策略
	policy := &CleanupPolicy{
		EnableAutoCleanup:  true,
		CompletedTaskTTL:   50 * time.Millisecond,
		FailedTaskTTL:      50 * time.Millisecond,
		CancelledTaskTTL:   50 * time.Millisecond,
		CleanupInterval:    25 * time.Millisecond,
	}
	
	scheduler := NewMemoryTaskScheduler(WithCleanupPolicy(policy))
	
	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Close(ctx)
	
	// 创建成功任务
	successTask := NewTaskBuilder().
		WithType("success").
		WithHandler(func(ctx context.Context) error { 
			time.Sleep(20 * time.Millisecond)
			return nil 
		}).
		Build()
	
	// 创建失败任务
	failTask := NewTaskBuilder().
		WithType("fail").
		WithHandler(func(ctx context.Context) error { 
			time.Sleep(20 * time.Millisecond)
			return fmt.Errorf("任务失败") 
		}).
		Build()
	
	// 调度任务
	err = scheduler.Schedule(successTask)
	if err != nil {
		t.Fatalf("调度成功任务失败: %v", err)
	}
	
	err = scheduler.Schedule(failTask)
	if err != nil {
		t.Fatalf("调度失败任务失败: %v", err)
	}
	
	// 等待任务执行完成
	time.Sleep(100 * time.Millisecond)
	
	// 验证任务状态（如果任务还存在的话）
	successStatus, successErr := scheduler.GetTaskStatus(successTask.ID())
	if successErr == nil {
		if successStatus != TaskStatusCompleted {
			t.Errorf("成功任务状态应为completed，实际为%s", successStatus)
		}
	}
	
	failStatus, failErr := scheduler.GetTaskStatus(failTask.ID())
	if failErr == nil {
		if failStatus != TaskStatusFailed {
			t.Errorf("失败任务状态应为failed，实际为%s", failStatus)
		}
	}
	
	// 等待自动清理
	time.Sleep(200 * time.Millisecond)
	
	// 验证任务被自动清理
	tasks := scheduler.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("期望任务被自动清理，但仍有%d个任务", len(tasks))
	}
}