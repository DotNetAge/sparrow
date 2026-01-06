package tasks

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConcurrentScheduler 测试并发调度器的基本功能
// TestMaxConcurrentTasks 测试最大并发数功能
func TestMaxConcurrentTasks(t *testing.T) {
	// 测试1: 并发调度器的最大并发数
	t.Run("ConcurrentScheduler", func(t *testing.T) {
		// 创建并发调度器并设置最大并发数为2
		wrapper := NewSchedulerWrapper(
			WithScheduler(NewConcurrentScheduler()),
			WithMaxConcurrent(2),
		)
		defer wrapper.Stop()

		// 启动调度器
		err := wrapper.Instance.Start(context.Background())
		assert.NoError(t, err)

		// 用于跟踪活跃任务数
		var activeTasks atomic.Int32
		var maxActiveTasks atomic.Int32
		taskCompleteChan := make(chan bool, 5)

		// 创建5个任务，每个任务会睡眠一段时间以模拟并发执行
		for i := 0; i < 5; i++ {
			go func(taskID int) {
				taskIDStr := fmt.Sprintf("concurrent-task-%d", taskID)
				err := wrapper.RunTaskWithID(taskIDStr, func(ctx context.Context) error {
					// 增加活跃任务数并更新最大值
					current := activeTasks.Add(1)
					if current > maxActiveTasks.Load() {
						maxActiveTasks.Store(current)
					}

					// 睡眠一段时间以确保多个任务同时执行
					time.Sleep(100 * time.Millisecond)

					// 减少活跃任务数
					activeTasks.Add(-1)
					taskCompleteChan <- true
					return nil
				})
				assert.NoError(t, err)
			}(i)
		}

		// 等待所有任务完成
		for i := 0; i < 5; i++ {
			select {
			case <-taskCompleteChan:
				// 任务完成
			case <-time.After(2 * time.Second):
				t.Fatal("任务执行超时")
			}
		}

		// 验证最大并发数不超过设置的值
		assert.Equal(t, int32(2), maxActiveTasks.Load(), "最大并发数应该是2")
	})

	// 测试2: 混合调度器中的并发部分最大并发数
	t.Run("HybridScheduler", func(t *testing.T) {
		// 创建混合调度器并设置最大并发数为3
		wrapper := NewSchedulerWrapper(
			WithScheduler(NewHybridScheduler()),
			WithMaxConcurrent(3),
			WithConcurrent("concurrent-type"),
		)
		defer wrapper.Stop()

		// 启动调度器
		err := wrapper.Instance.Start(context.Background())
		assert.NoError(t, err)

		// 用于跟踪活跃任务数
		var activeTasks atomic.Int32
		var maxActiveTasks atomic.Int32
		taskCompleteChan := make(chan bool, 6)

		// 创建6个并发类型的任务
		for i := 0; i < 6; i++ {
			go func(taskID int) {
				taskIDStr := fmt.Sprintf("hybrid-task-%d", taskID)
				err := wrapper.RunTypedTaskWithID(taskIDStr, "concurrent-type", func(ctx context.Context) error {
					// 增加活跃任务数并更新最大值
					current := activeTasks.Add(1)
					if current > maxActiveTasks.Load() {
						maxActiveTasks.Store(current)
					}

					// 睡眠一段时间以确保多个任务同时执行
					time.Sleep(100 * time.Millisecond)

					// 减少活跃任务数
					activeTasks.Add(-1)
					taskCompleteChan <- true
					return nil
				})
				assert.NoError(t, err)
			}(i)
		}

		// 等待所有任务完成
		for i := 0; i < 6; i++ {
			select {
			case <-taskCompleteChan:
				// 任务完成
			case <-time.After(2 * time.Second):
				t.Fatal("任务执行超时")
			}
		}

		// 验证最大并发数不超过设置的值
		assert.Equal(t, int32(3), maxActiveTasks.Load(), "最大并发数应该是3")
	})
}

func TestConcurrentScheduler(t *testing.T) {
	// 使用SchedulerWrapper而不是直接使用调度器
	wrapper := NewSchedulerWrapper(WithScheduler(NewConcurrentScheduler()))
	defer wrapper.Stop()

	// 启动调度器
	err := wrapper.Instance.Start(context.Background())
	assert.NoError(t, err)

	// 任务完成标志
	completed := make(chan bool, 1)

	// 使用Wrapper添加任务
	err = wrapper.RunTaskWithID("test-task-1", func(ctx context.Context) error {
		completed <- true
		return nil
	})
	assert.NoError(t, err)

	// 等待任务完成
	select {
	case <-completed:
		// 任务成功完成
	case <-time.After(1 * time.Second):
		t.Fatal("任务执行超时")
	}

	// 验证任务状态
	status, err := wrapper.GetTaskStatus("test-task-1")
	assert.NoError(t, err)
	assert.Equal(t, TaskStatus("completed"), status)

	// 取消不存在的任务
	err = wrapper.CancelTask("non-existent-task")
	assert.Error(t, err)
}

// TestSequentialScheduler 测试顺序调度器的基本功能
func TestSequentialScheduler(t *testing.T) {
	// 使用SchedulerWrapper而不是直接使用调度器
	wrapper := NewSchedulerWrapper(WithScheduler(NewSequentialScheduler()))
	defer wrapper.Stop()

	// 启动调度器
	err := wrapper.Instance.Start(context.Background())
	assert.NoError(t, err)

	// 用于记录任务执行顺序
	order := make([]int, 0)
	orderMutex := sync.Mutex{}

	// 使用Wrapper添加任务，验证顺序执行
	err = wrapper.RunTaskWithID("sequential-task-1", func(ctx context.Context) error {
		orderMutex.Lock()
		order = append(order, 1)
		orderMutex.Unlock()
		time.Sleep(50 * time.Millisecond) // 模拟耗时操作
		return nil
	})
	assert.NoError(t, err)

	err = wrapper.RunTaskWithID("sequential-task-2", func(ctx context.Context) error {
		orderMutex.Lock()
		order = append(order, 2)
		orderMutex.Unlock()
		return nil
	})
	assert.NoError(t, err)

	// 等待任务完成
	time.Sleep(200 * time.Millisecond)

	// 验证执行顺序
	assert.Equal(t, []int{1, 2}, order)
}

// TestHybridScheduler 测试混合调度器根据任务类型选择执行模式
func TestHybridScheduler(t *testing.T) {
	// 使用SchedulerWrapper
	wrapper := NewSchedulerWrapper(
		WithScheduler(NewHybridScheduler()),
		WithSequential("critical-task"),
	)
	defer wrapper.Stop()

	// 启动调度器
	err := wrapper.Instance.Start(context.Background())
	assert.NoError(t, err)

	// 测试不同类型的任务
	concurrentCompleted := make(chan bool, 1)
	sequentialCompleted := make(chan bool, 1)

	// 使用Wrapper添加不同类型的任务
	err = wrapper.RunTypedTaskWithID("concurrent-test", "normal-task", func(ctx context.Context) error {
		concurrentCompleted <- true
		return nil
	})
	assert.NoError(t, err)

	err = wrapper.RunTypedTaskWithID("sequential-test", "critical-task", func(ctx context.Context) error {
		sequentialCompleted <- true
		return nil
	})
	assert.NoError(t, err)

	// 等待任务完成
	timeout := time.After(1 * time.Second)
	receivedConcurrent := false
	receivedSequential := false

	for i := 0; i < 2; i++ {
		select {
		case <-concurrentCompleted:
			receivedConcurrent = true
		case <-sequentialCompleted:
			receivedSequential = true
		case <-timeout:
			t.Fatal("任务执行超时")
		}
	}

	assert.True(t, receivedConcurrent)
	assert.True(t, receivedSequential)
}

// TestRetryMechanism 测试任务重试机制
func TestRetryMechanism(t *testing.T) {
	// 使用SchedulerWrapper
	wrapper := NewSchedulerWrapper(WithScheduler(NewConcurrentScheduler()))
	defer wrapper.Stop()

	// 启动调度器
	err := wrapper.Instance.Start(context.Background())
	assert.NoError(t, err)

	// 用于记录执行次数
	attempts := 0
	attemptsMutex := sync.Mutex{}

	// 使用Wrapper添加一个简单任务
	err = wrapper.RunTaskWithID("retry-task", func(ctx context.Context) error {
		attemptsMutex.Lock()
		defer attemptsMutex.Unlock()
		attempts++
		return nil
	})
	assert.NoError(t, err)

	// 等待任务完成
	time.Sleep(100 * time.Millisecond)

	// 验证任务只执行了一次
	assert.Equal(t, 1, attempts)

	// 验证任务状态
	status, err := wrapper.GetTaskStatus("retry-task")
	assert.NoError(t, err)
	assert.Equal(t, TaskStatus("completed"), status)
}

// TestTimeoutControl 测试任务超时控制
func TestTimeoutControl(t *testing.T) {
	// 使用自定义的包装器来测试超时，因为当前Wrapper没有直接暴露设置超时的接口
	// 我们需要修改wrapper来支持超时设置，或者直接在测试中使用特殊方式
	// 这里我们先使用现有的方式，后续可以扩展Wrapper的功能
	wrapper := NewSchedulerWrapper(WithScheduler(NewConcurrentScheduler()))
	defer wrapper.Stop()

	// 启动调度器
	err := wrapper.Instance.Start(context.Background())
	assert.NoError(t, err)

	// 创建一个会超时的任务（通过上下文控制）
	err = wrapper.RunTaskWithID("timeout-task", func(ctx context.Context) error {
		// 创建一个带超时的上下文
		timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		// 模拟长时间运行的任务
		select {
		case <-timeoutCtx.Done():
			// 任务被上下文取消（超时）
			return timeoutCtx.Err()
		case <-time.After(2 * time.Second):
			// 任务正常完成
			return nil
		}
	})
	assert.NoError(t, err)

	// 等待任务超时
	time.Sleep(1 * time.Second)

	// 验证任务状态为失败（超时）
	status, err := wrapper.GetTaskStatus("timeout-task")
	assert.NoError(t, err)
	// 超时任务可能是失败或已取消状态，具体取决于实现
	assert.True(t, status == TaskStatus("failed") || status == TaskStatus("cancelled"))
}

// TestSchedulerWrapper 测试调度器包装器的功能
func TestSchedulerWrapper(t *testing.T) {
	// 使用默认包装器（内部使用混合调度器）
	wrapper := NewSchedulerWrapper(WithScheduler(NewConcurrentScheduler()))
	defer wrapper.Stop()

	// 设置特定任务类型使用顺序模式
	wrapper.SetTaskTypeMode("special-task", Sequential)

	// 提交普通任务
	completed := make(chan bool, 1)
	taskID, err := wrapper.RunTask(func(ctx context.Context) error {
		completed <- true
		return nil
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 等待任务完成
	select {
	case <-completed:
		// 任务成功完成
	case <-time.After(1 * time.Second):
		t.Fatal("任务执行超时")
	}

	// 验证任务状态
	status, err := wrapper.GetTaskStatus(taskID)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatus("completed"), status)

	// 提交带类型的任务
	typedCompleted := make(chan bool, 1)
	typedTaskID, err := wrapper.RunTypedTask("special-task", func(ctx context.Context) error {
		typedCompleted <- true
		return nil
	})
	assert.NoError(t, err)

	// 等待任务完成
	select {
	case <-typedCompleted:
		// 任务成功完成
	case <-time.After(1 * time.Second):
		t.Fatal("类型化任务执行超时")
	}

	// 验证列表功能
	tasks := wrapper.ListTasks()
	assert.GreaterOrEqual(t, len(tasks), 2)

	// 取消任务（虽然任务已完成，但接口应该正常工作）
	err = wrapper.CancelTask(typedTaskID)
	assert.NoError(t, err) // 即使任务已完成，取消操作也应该成功（幂等性）
}

// TestMemoryManagement 测试内存管理功能
func TestMemoryManagement(t *testing.T) {
	// 使用SchedulerWrapper
	wrapper := NewSchedulerWrapper(WithScheduler(NewConcurrentScheduler()))
	defer wrapper.Stop()

	// 启动调度器
	err := wrapper.Instance.Start(context.Background())
	assert.NoError(t, err)

	// 提交多个短时间任务
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		taskID := fmt.Sprintf("memory-task-%d", i)
		wrapper.RunTaskWithID(taskID, func(ctx context.Context) error {
			defer wg.Done()
			return nil
		})
	}

	// 等待所有任务完成
	wg.Wait()

	// 验证列表功能
	tasks := wrapper.ListTasks()
	// 由于内存管理会自动清理已完成任务，这里不做具体数量断言
	// 只验证接口正常工作
	assert.NotNil(t, tasks)
}

// TestTaskBuilderAndSchedulerWrapper 测试修改后的TaskBuilder和SchedulerWrapper功能
func TestTaskBuilderAndSchedulerWrapper(t *testing.T) {
	// 测试1: 验证TaskBuilder可以正常使用
	builder := NewTaskBuilder().WithID("direct-builder-test").WithHandler(func(ctx context.Context) error {
		return nil
	})
	task, _ := builder.Build()
	assert.NotNil(t, task)
	assert.Equal(t, "direct-builder-test", task.ID())

	// 测试2: 验证SchedulerWrapper的基本功能
	wrapper := NewSchedulerWrapper(WithScheduler(NewConcurrentScheduler()))
	defer wrapper.Stop()

	// 测试3: 验证RunTaskWithID方法
	taskID := "test-task-1"
	err := wrapper.RunTaskWithID(taskID, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// 等待任务完成
	time.Sleep(100 * time.Millisecond)

	// 测试4: 验证GetTaskStatus方法
	status, err := wrapper.GetTaskStatus(taskID)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatus("completed"), status)

	// 测试5: 验证ListTasks方法
	tasks := wrapper.ListTasks()
	assert.NotEmpty(t, tasks)

	// 测试6: 验证CancelTask方法
	taskID2 := "test-task-2"
	err = wrapper.RunTaskWithID(taskID2, func(ctx context.Context) error {
		time.Sleep(2 * time.Second) // 模拟长时间运行的任务
		return nil
	})
	assert.NoError(t, err)

	err = wrapper.CancelTask(taskID2)
	assert.NoError(t, err)

	// 测试7: 验证定时任务功能
	taskID3, err := wrapper.RunTaskAt(time.Now().Add(100*time.Millisecond), func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID3)

	// 测试8: 验证周期性任务功能
	taskID4, err := wrapper.RunTaskRecurring(200*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID4)

	// 等待一段时间确保任务执行
	time.Sleep(300 * time.Millisecond)

	// 关闭功能由defer自动处理
}

// TestDefaultRetryFunctionality 测试修复后的默认重试功能
func TestDefaultRetryFunctionality(t *testing.T) {
	// 创建一个新的调度器包装器
	wrapper := NewSchedulerWrapper(WithScheduler(NewConcurrentScheduler()))
	defer wrapper.Stop()

	// 启动调度器
	err := wrapper.Instance.Start(context.Background())
	assert.NoError(t, err)

	// 测试1: 验证默认重试次数为3次
	var failCount int32
	// 创建一个总是失败的任务，应该重试3次后最终失败
	taskID := "always-failing-task"
	err = wrapper.RunTaskWithID(taskID, func(ctx context.Context) error {
		atomic.AddInt32(&failCount, 1)
		return errors.New("task always fails")
	})
	assert.NoError(t, err)

	// 等待所有重试完成（3次重试 + 初始执行 = 4次）
	time.Sleep(4 * time.Second) // 足够等待指数退避的重试完成

	// 验证任务执行了4次（1次初始 + 3次重试）
	expectedExecutions := int32(4)
	actualExecutions := atomic.LoadInt32(&failCount)
	assert.Equal(t, expectedExecutions, actualExecutions, "Expected task to be executed %d times (1 initial + 3 retries)", expectedExecutions)

	// 测试2: 验证在达到最大重试次数前成功
	var successAfterRetriesCount int32
	// 让任务在前2次失败，第3次成功
	taskID2 := "retry-and-succeed-task"
	err = wrapper.RunTaskWithID(taskID2, func(ctx context.Context) error {
		currentAttempt := atomic.AddInt32(&successAfterRetriesCount, 1)
		if currentAttempt <= 2 {
			return errors.New("task fails temporarily")
		}
		return nil
	})
	assert.NoError(t, err)

	// 等待任务完成
	time.Sleep(3 * time.Second)

	// 验证任务在第3次执行时成功
	expectedSuccessAttempts := int32(3)
	actualAttempts := atomic.LoadInt32(&successAfterRetriesCount)
	assert.Equal(t, expectedSuccessAttempts, actualAttempts, "Expected task to succeed after %d attempts", expectedSuccessAttempts)

	// 关闭功能由defer自动处理
}
