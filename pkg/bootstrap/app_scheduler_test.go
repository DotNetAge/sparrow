package bootstrap

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/tasks"
	"github.com/stretchr/testify/assert"
)

// TestAppScheduler_ImmediateTask 测试即时执行任务
func TestAppScheduler_ImmediateTask(t *testing.T) {
	// 创建应用实例，使用HybridTasks配置任务系统
	app := NewApp(Tasks())
	defer app.CleanUp()

	// 任务执行状态标记
	executed := false
	taskID, err := app.Scheduler.RunTask(func(ctx context.Context) error {
		executed = true
		return nil
	})
	// 启动应用以初始化调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 等待任务执行
	time.Sleep(100 * time.Millisecond)

	// 验证任务已执行
	assert.True(t, executed)
}

// TestAppScheduler_ScheduledTask 测试定时执行任务
func TestAppScheduler_ScheduledTask(t *testing.T) {
	// 创建应用实例，使用HybridTasks配置任务系统
	app := NewApp(Tasks(
		tasks.WithSequential("sequential"),
	))
	defer app.CleanUp()

	// 设置任务在100ms后执行
	scheduleTime := time.Now().Add(100 * time.Millisecond)

	// 任务执行状态标记
	executed := false
	taskID, err := app.Scheduler.RunTaskAt(scheduleTime, func(ctx context.Context) error {
		executed = true
		app.Logger.Info("测试任务执行成功")
		return nil
	})

	// 启动应用以初始化调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 立即检查，任务不应已执行
	assert.False(t, executed)

	// 等待任务执行
	time.Sleep(200 * time.Millisecond)

	// 验证任务已执行
	assert.True(t, executed)
}

// TestAppScheduler_RecurringTask 测试重复执行任务
func TestAppScheduler_RecurringTask(t *testing.T) {
	// 创建应用实例，使用HybridTasks配置任务系统
	app := NewApp(Tasks())
	defer app.CleanUp()

	// 计数器
	var count int

	// 创建每100ms重复执行的任务
	// RunTaskRecurring 应该是以顺序方式以单协程非并发的方式执行任务，否则是否会出现协程争夺而导致疯狂地像死循环一样执行任务？
	taskID, err := app.Scheduler.RunTaskRecurring(100*time.Millisecond, func(ctx context.Context) error {
		count++
		return nil
	})

	// 启动应用以初始化调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 等待多个周期
	time.Sleep(350 * time.Millisecond)

	// 取消任务
	if ts, ok := app.Scheduler.Instance.(tasks.TaskScheduler); ok {
		err = ts.Cancel(taskID)
		assert.NoError(t, err)
	}

	// 等待取消生效
	time.Sleep(50 * time.Millisecond)

	// 验证任务执行了多次 - 任务每100ms执行一次，等待350ms应该执行约3次
	assert.Equal(t, 3, count)
}

// TestAppScheduler_CancelTask 测试取消任务
func TestAppScheduler_CancelTask(t *testing.T) {
	// 创建应用实例，使用HybridTasks配置任务系统
	app := NewApp(Tasks())
	defer app.CleanUp()

	// 设置任务在1秒后执行
	scheduleTime := time.Now().Add(1 * time.Second)

	// 任务执行状态标记
	executed := false
	taskID, err := app.Scheduler.RunTaskAt(scheduleTime, func(ctx context.Context) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 取消任务
	err = app.Scheduler.CancelTask(taskID)
	assert.NoError(t, err)

	// 等待足够时间，验证任务未执行
	time.Sleep(1100 * time.Millisecond)
	assert.False(t, executed)
}

// TestAppScheduler_FailedTaskWithRetry 测试失败任务的重试机制
func TestAppScheduler_FailedTaskWithRetry(t *testing.T) {
	// 创建应用实例，使用HybridTasks配置任务系统
	app := NewApp(Tasks())
	defer app.CleanUp()

	// 计数器，用于模拟失败
	var attempts int

	// 创建一个前两次执行会失败，第三次成功的任务
	taskID, err := app.Scheduler.RunTask(func(ctx context.Context) error {
		attempts++
		fmt.Printf("任务执行第 %d 次\n", attempts)
		if attempts <= 2 {
			return assert.AnError // 返回错误
		}
		return nil // 第三次执行成功
	})

	// 启动应用以初始化调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 增加等待时间，确保重试机制有足够时间完成所有重试
	// 考虑到指数退避算法，需要等待更长时间
	fmt.Println("等待任务重试...")
	time.Sleep(10 * time.Second) // 增加到10秒，确保有足够时间完成重试

	// 验证任务尝试了3次
	fmt.Printf("最终尝试次数: %d\n", attempts)
	assert.Equal(t, 3, attempts)
}

// TestAppScheduler_HybridScheduler 测试混合调度器
func TestAppScheduler_HybridScheduler(t *testing.T) {
	// 创建应用实例，并注册混合调度器
	// 通过NeedCleanup注册，App.Start()会自动启动它
	app := NewApp(Tasks(), ServerPort(8081)) // 使用不同端口避免冲突
	defer app.CleanUp()

	// 在goroutine中启动app.Start()，这样它会自动启动调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	// 等待一段时间确保调度器已完全启动
	time.Sleep(500 * time.Millisecond)

	// 任务执行状态标记
	executed := false
	taskID, err := app.Scheduler.RunTask(func(ctx context.Context) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 增加等待时间确保任务执行完成
	time.Sleep(300 * time.Millisecond)

	// 验证任务已执行
	assert.True(t, executed)
}

// TestAppScheduler_SingleScheduler 测试单一调度器
func TestAppScheduler_SingleScheduler(t *testing.T) {
	// 创建应用实例，并注册单一调度器
	// 通过NeedCleanup注册，App.Start()会自动启动它
	app := NewApp(Tasks(), ServerPort(8082)) // 使用不同端口避免冲突
	defer app.CleanUp()

	// 在goroutine中启动app.Start()，这样它会自动启动调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	// 等待一段时间确保调度器已完全启动
	time.Sleep(500 * time.Millisecond)

	// 任务执行状态标记
	executed := false
	taskID, err := app.Scheduler.RunTask(func(ctx context.Context) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 增加等待时间确保任务执行完成
	time.Sleep(300 * time.Millisecond)

	// 验证任务已执行
	assert.True(t, executed)
}

// TestAppScheduler_TaskTypePolicy 测试任务类型执行策略
func TestAppScheduler_TaskTypePolicy(t *testing.T) {
	// 创建应用实例，配置混合调度器并注册任务类型策略
	app := NewApp(
		Tasks(
			tasks.WithConcurrent("email", "notification"),
			tasks.WithSequential("payment", "order"),
		),
		ServerPort(8083),
	)
	defer app.CleanUp()

	// 启动应用以初始化调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	// 等待调度器初始化
	time.Sleep(500 * time.Millisecond)

	// 验证并发任务类型的执行
	concurrentExecuted := false
	_, err := app.Scheduler.RunTypedTask("email", func(ctx context.Context) error {
		concurrentExecuted = true
		return nil
	})
	assert.NoError(t, err)

	// 验证顺序任务类型的执行
	sequentialExecuted := false
	_, err = app.Scheduler.RunTypedTask("payment", func(ctx context.Context) error {
		sequentialExecuted = true
		return nil
	})
	assert.NoError(t, err)

	// 等待任务执行完成
	time.Sleep(300 * time.Millisecond)

	// 验证所有任务都已执行
	assert.True(t, concurrentExecuted)
	assert.True(t, sequentialExecuted)
}

// TestAppScheduler_ConcurrentVsSequentialExecution 测试并发和顺序任务的执行模式差异
func TestAppScheduler_ConcurrentVsSequentialExecution(t *testing.T) {
	// 创建应用实例，配置混合调度器并注册任务类型策略
	app := NewApp(
		Tasks(
			tasks.WithConcurrent("concurrent_task"),
			tasks.WithSequential("sequential_task"),
		),
		ServerPort(8084), // 使用不同端口避免冲突
	)
	defer app.CleanUp()

	// 启动应用以初始化调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	// 等待调度器初始化
	time.Sleep(500 * time.Millisecond)

	// 测试并发任务的执行模式
	concurrentStartTimes := make([]time.Time, 3)
	concurrentChan := make(chan struct{}, 3)
	concurrentExecutionTime := 200 * time.Millisecond

	// 提交3个并发任务，它们应该几乎同时开始执行
	for i := 0; i < 3; i++ {
		idx := i // 捕获循环变量
		_, err := app.Scheduler.RunTypedTask("concurrent_task", func(ctx context.Context) error {
			concurrentStartTimes[idx] = time.Now()
			time.Sleep(concurrentExecutionTime) // 模拟工作负载
			concurrentChan <- struct{}{}
			return nil
		})
		assert.NoError(t, err)
	}

	// 测试顺序任务的执行模式
	sequentialStartTimes := make([]time.Time, 3)
	sequentialChan := make(chan struct{}, 3)
	sequentialExecutionTime := 200 * time.Millisecond

	// 提交3个顺序任务，它们应该按顺序执行
	for i := 0; i < 3; i++ {
		idx := i // 捕获循环变量
		_, err := app.Scheduler.RunTypedTask("sequential_task", func(ctx context.Context) error {
			sequentialStartTimes[idx] = time.Now()
			time.Sleep(sequentialExecutionTime) // 模拟工作负载
			sequentialChan <- struct{}{}
			return nil
		})
		assert.NoError(t, err)
	}

	// 等待所有任务完成
	for i := 0; i < 3; i++ {
		<-concurrentChan
	}
	for i := 0; i < 3; i++ {
		<-sequentialChan
	}

	// 验证并发任务的执行模式
	// 并发任务的启动时间差应该很小（远小于执行时间）
	concurrentTimeDiff1 := concurrentStartTimes[1].Sub(concurrentStartTimes[0])
	concurrentTimeDiff2 := concurrentStartTimes[2].Sub(concurrentStartTimes[1])
	fmt.Printf("并发任务启动时间差: %v, %v\n", concurrentTimeDiff1, concurrentTimeDiff2)

	// 并发任务的启动时间差异应该远小于任务执行时间
	assert.Less(t, concurrentTimeDiff1, concurrentExecutionTime/2)
	assert.Less(t, concurrentTimeDiff2, concurrentExecutionTime/2)

	// 验证顺序任务的执行模式
	// 顺序任务的启动时间差应该接近或大于任务执行时间
	sequentialTimeDiff1 := sequentialStartTimes[1].Sub(sequentialStartTimes[0])
	sequentialTimeDiff2 := sequentialStartTimes[2].Sub(sequentialStartTimes[1])
	fmt.Printf("顺序任务启动时间差: %v, %v\n", sequentialTimeDiff1, sequentialTimeDiff2)

	// 顺序任务的启动时间差异应该接近或大于任务执行时间
	// 允许一定的误差范围
	minExpectedDiff := sequentialExecutionTime * 9 / 10 // 允许10%的误差
	assert.GreaterOrEqual(t, sequentialTimeDiff1, minExpectedDiff)
	assert.GreaterOrEqual(t, sequentialTimeDiff2, minExpectedDiff)
}

// TestAppScheduler_MixedTaskTypesExecution 测试同时注册顺序和并发任务并验证执行模式
func TestAppScheduler_MixedTaskTypesExecution(t *testing.T) {
	// 创建应用实例，同时注册多种任务类型策略
	app := NewApp(
		Tasks(
			tasks.WithConcurrent("email", "notification"),
			tasks.WithSequential("payment", "order"),
		),
		ServerPort(8085), // 使用不同端口避免冲突
	)
	defer app.CleanUp()

	// 启动应用以初始化调度器
	go func() {
		if err := app.Start(); err != nil {
			t.Errorf("app.Start() failed: %v", err)
		}
	}()

	// 等待调度器初始化
	time.Sleep(500 * time.Millisecond)

	// 记录任务执行时间和顺序的通道
	resultsChan := make(chan struct {
		TaskType  string
		TaskID    int
		StartTime time.Time
		EndTime   time.Time
	}, 8)

	// 定义任务执行时间
	taskExecutionTime := 150 * time.Millisecond

	// 提交2个并发类型的任务(email)
	for i := 0; i < 2; i++ {
		taskID := i
		_, err := app.Scheduler.RunTypedTask("email", func(ctx context.Context) error {
			startTime := time.Now()
			time.Sleep(taskExecutionTime) // 模拟工作负载
			endTime := time.Now()
			resultsChan <- struct {
				TaskType  string
				TaskID    int
				StartTime time.Time
				EndTime   time.Time
			}{"email", taskID, startTime, endTime}
			return nil
		})
		assert.NoError(t, err)
	}

	// 提交2个并发类型的任务(notification)
	for i := 0; i < 2; i++ {
		taskID := i
		_, err := app.Scheduler.RunTypedTask("notification", func(ctx context.Context) error {
			startTime := time.Now()
			time.Sleep(taskExecutionTime) // 模拟工作负载
			endTime := time.Now()
			resultsChan <- struct {
				TaskType  string
				TaskID    int
				StartTime time.Time
				EndTime   time.Time
			}{"notification", taskID, startTime, endTime}
			return nil
		})
		assert.NoError(t, err)
	}

	// 提交2个顺序类型的任务(payment)
	for i := 0; i < 2; i++ {
		taskID := i
		_, err := app.Scheduler.RunTypedTask("payment", func(ctx context.Context) error {
			startTime := time.Now()
			time.Sleep(taskExecutionTime) // 模拟工作负载
			endTime := time.Now()
			resultsChan <- struct {
				TaskType  string
				TaskID    int
				StartTime time.Time
				EndTime   time.Time
			}{"payment", taskID, startTime, endTime}
			return nil
		})
		assert.NoError(t, err)
	}

	// 提交2个顺序类型的任务(order)
	for i := 0; i < 2; i++ {
		taskID := i
		_, err := app.Scheduler.RunTypedTask("order", func(ctx context.Context) error {
			startTime := time.Now()
			time.Sleep(taskExecutionTime) // 模拟工作负载
			endTime := time.Now()
			resultsChan <- struct {
				TaskType  string
				TaskID    int
				StartTime time.Time
				EndTime   time.Time
			}{"order", taskID, startTime, endTime}
			return nil
		})
		assert.NoError(t, err)
	}

	// 收集所有任务执行结果
	results := make(map[string][]struct {
		TaskID    int
		StartTime time.Time
		EndTime   time.Time
	})

	// 初始化结果map的键
	for _, taskType := range []string{"email", "notification", "payment", "order"} {
		results[taskType] = make([]struct {
			TaskID    int
			StartTime time.Time
			EndTime   time.Time
		}, 0, 2)
	}

	// 从通道接收结果
	for i := 0; i < 8; i++ {
		result := <-resultsChan
		results[result.TaskType] = append(results[result.TaskType], struct {
			TaskID    int
			StartTime time.Time
			EndTime   time.Time
		}{result.TaskID, result.StartTime, result.EndTime})
	}

	// 验证并发任务类型(email)的执行模式
	if len(results["email"]) >= 2 {
		emailTimeDiff := results["email"][1].StartTime.Sub(results["email"][0].StartTime)
		fmt.Printf("email任务启动时间差: %v\n", emailTimeDiff)
		// 并发任务的启动时间差应该很小（远小于执行时间）
		assert.Less(t, emailTimeDiff, taskExecutionTime/2)
	}

	// 验证并发任务类型(notification)的执行模式
	if len(results["notification"]) >= 2 {
		notificationTimeDiff := results["notification"][1].StartTime.Sub(results["notification"][0].StartTime)
		fmt.Printf("notification任务启动时间差: %v\n", notificationTimeDiff)
		// 并发任务的启动时间差应该很小（远小于执行时间）
		assert.Less(t, notificationTimeDiff, taskExecutionTime/2)
	}

	// 验证顺序任务类型(payment)的执行模式
	if len(results["payment"]) >= 2 {
		paymentTimeDiff := results["payment"][1].StartTime.Sub(results["payment"][0].StartTime)
		fmt.Printf("payment任务启动时间差: %v\n", paymentTimeDiff)
		// 顺序任务的启动时间差应该接近或大于任务执行时间
		minExpectedDiff := taskExecutionTime * 9 / 10 // 允许10%的误差
		assert.GreaterOrEqual(t, paymentTimeDiff, minExpectedDiff)
	}

	// 验证顺序任务类型(order)的执行模式
	if len(results["order"]) >= 2 {
		orderTimeDiff := results["order"][1].StartTime.Sub(results["order"][0].StartTime)
		fmt.Printf("order任务启动时间差: %v\n", orderTimeDiff)
		// 顺序任务的启动时间差应该接近或大于任务执行时间
		minExpectedDiff := taskExecutionTime * 9 / 10 // 允许10%的误差
		assert.GreaterOrEqual(t, orderTimeDiff, minExpectedDiff)
	}

	// 验证不同类型任务之间的并发执行 - 确认至少有一些不同类型的任务重叠执行
	allTasks := make([]struct {
		TaskType  string
		StartTime time.Time
		EndTime   time.Time
	}, 0, 8)

	// 收集所有任务的时间信息
	for taskType, tasks := range results {
		for _, task := range tasks {
			allTasks = append(allTasks, struct {
				TaskType  string
				StartTime time.Time
				EndTime   time.Time
			}{taskType, task.StartTime, task.EndTime})
		}
	}

	// 查找是否有不同类型任务重叠执行的情况
	foundOverlap := false
	for i := 0; i < len(allTasks); i++ {
		for j := i + 1; j < len(allTasks); j++ {
			// 检查不同类型的任务是否重叠执行
			if allTasks[i].TaskType != allTasks[j].TaskType {
				// 任务i开始于任务j结束前，且任务i结束于任务j开始后
				if allTasks[i].StartTime.Before(allTasks[j].EndTime) && allTasks[i].EndTime.After(allTasks[j].StartTime) {
					fmt.Printf("发现不同类型任务重叠执行: %s 与 %s\n", allTasks[i].TaskType, allTasks[j].TaskType)
					foundOverlap = true
					break
				}
			}
		}
		if foundOverlap {
			break
		}
	}

	// 验证存在不同类型任务的重叠执行
	assert.True(t, foundOverlap, "应该存在不同类型任务的重叠执行")
}
