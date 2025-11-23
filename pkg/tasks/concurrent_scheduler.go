package tasks

import (
	"context"
	"errors"
	"sync"
	"time"
)

// concurrentScheduler 并发执行模式的任务调度器实现
// 可以同时执行多个任务，任务的执行顺序与注册顺序无关
// 支持任务的调度、取消、状态查询等功能
// 实现了内存管理、超时控制和重试机制

type concurrentScheduler struct {
	TaskScheduler
	tasks     map[string]Task // 存储所有任务
	taskMutex sync.RWMutex    // 保护任务映射的互斥锁
	// wg              sync.WaitGroup  // 等待所有任务完成的等待组
	maxConcurrent   int           // 最大并发任务数
	activeTasks     int           // 当前活跃任务数
	activeTasksCond *sync.Cond    // 控制并发任务数的条件变量
	stopChan        chan struct{} // 停止信号通道
	startOnce       sync.Once     // 确保调度器只启动一次
	taskTTL         time.Duration // 任务完成后的保存时间（默认5分钟）
	maxTaskCount    int           // 最大保存的任务数量（默认500个）
}

// NewConcurrentScheduler 创建一个新的并发执行模式任务调度器
func NewConcurrentScheduler() TaskScheduler {
	cs := &concurrentScheduler{
		tasks:         make(map[string]Task),
		maxConcurrent: 10, // 默认最大并发数为10
		stopChan:      make(chan struct{}),
		taskTTL:       5 * time.Minute, // 任务完成后保存5分钟
		maxTaskCount:  500,             // 最大保存500个任务
	}
	cs.activeTasksCond = sync.NewCond(&cs.taskMutex)
	return cs
}

// Start 启动任务调度器
func (s *concurrentScheduler) Start(ctx context.Context) error {
	s.startOnce.Do(func() {
		// 启动一个协程来处理定时任务和过期任务清理
		go s.runTaskMonitor()
	})
	return nil
}

// runTaskMonitor 运行任务监视器
func (s *concurrentScheduler) runTaskMonitor() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.processScheduledTasks()
			s.cleanupExpiredTasks()
		}
	}
}

// Schedule 调度一个任务
func (s *concurrentScheduler) Schedule(task Task) error {
	if task == nil {
		return errors.New("任务不能为空")
	}

	taskID := task.ID()
	if taskID == "" {
		return errors.New("任务ID不能为空")
	}

	// 检查任务数量限制
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()

	if len(s.tasks) >= s.maxTaskCount {
		// 如果达到最大任务数量，删除最老的已完成任务
		if !s.removeOldestCompletedTask() {
			return errors.New("任务数量已达到上限")
		}
	}

	// 设置任务信息
	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		taskInfo := taskInfoProvider.TaskInfo()
		taskInfo.Status = TaskStatusWaiting
		taskInfo.CreatedAt = time.Now()
		taskInfo.UpdatedAt = time.Now()
		taskInfo.TTL = time.Now().Add(s.taskTTL)
	}

	// 存储任务
	s.tasks[taskID] = task

	// 如果是即时执行的任务，立即执行
	if task.Schedule().IsZero() || task.Schedule().Before(time.Now()) {
		go s.executeTask(task)
	}

	return nil
}

// Cancel 取消一个任务
func (s *concurrentScheduler) Cancel(taskID string) error {
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return errors.New("任务不存在")
	}

	// 更新任务状态为取消
	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		taskInfo := taskInfoProvider.TaskInfo()
		taskInfo.Status = TaskStatusCancelled
		taskInfo.UpdatedAt = time.Now()
	}

	// 执行取消回调
	if cancelFunc := task.OnCancel(); cancelFunc != nil {
		go cancelFunc(context.Background())
	}

	// 从任务列表中移除
	delete(s.tasks, taskID)

	return nil
}

// Stop 停止任务调度器
func (s *concurrentScheduler) Stop() error {
	close(s.stopChan)

	// 等待所有活跃任务完成
	s.taskMutex.Lock()
	for s.activeTasks > 0 {
		s.activeTasksCond.Wait()
	}
	s.taskMutex.Unlock()

	return nil
}

// GetTaskStatus 获取任务状态
func (s *concurrentScheduler) GetTaskStatus(taskID string) (TaskStatus, error) {
	s.taskMutex.RLock()
	defer s.taskMutex.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return TaskStatusUnknown, errors.New("任务不存在")
	}

	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		return taskInfoProvider.TaskInfo().Status, nil
	}

	return TaskStatusUnknown, errors.New("无法获取任务状态")
}

// ListTasks 列出所有任务
func (s *concurrentScheduler) ListTasks() []TaskInfo {
	s.taskMutex.RLock()
	defer s.taskMutex.RUnlock()

	result := make([]TaskInfo, 0, len(s.tasks))
	for _, task := range s.tasks {
		if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
			result = append(result, *taskInfoProvider.TaskInfo())
		}
	}

	return result
}

// SetMaxConcurrentTasks 设置最大并发任务数
func (s *concurrentScheduler) SetMaxConcurrentTasks(max int) error {
	if max <= 0 {
		return errors.New("最大并发任务数必须大于0")
	}

	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()

	s.maxConcurrent = max
	return nil
}

// Close 优雅关闭调度器并清理资源（实现GracefulClose接口）
func (s *concurrentScheduler) Close(ctx context.Context) error {
	// 使用现有的Stop方法实现关闭逻辑
	// 如果需要，可以根据ctx实现超时控制
	return s.Stop()
}

// removeOldestCompletedTask 移除最老的已完成任务
func (s *concurrentScheduler) removeOldestCompletedTask() bool {
	var oldestTaskID string
	var oldestTime time.Time

	for taskID, task := range s.tasks {
		if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
			taskInfo := taskInfoProvider.TaskInfo()
			if taskInfo.Status == TaskStatusCompleted || taskInfo.Status == TaskStatusCancelled || taskInfo.Status == TaskStatusDeadLetter {
				if oldestTaskID == "" || taskInfo.UpdatedAt.Before(oldestTime) {
					oldestTaskID = taskID
					oldestTime = taskInfo.UpdatedAt
				}
			}
		}
	}

	if oldestTaskID != "" {
		delete(s.tasks, oldestTaskID)
		return true
	}
	return false
}

// processScheduledTasks 处理定时任务和重试任务
func (s *concurrentScheduler) processScheduledTasks() {
	now := time.Now()
	tasksToExecute := make([]Task, 0)

	s.taskMutex.RLock()
	for _, task := range s.tasks {
		if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
			taskInfo := taskInfoProvider.TaskInfo()
			// 检查是否是定时任务或重试任务
			isScheduledTask := taskInfo.Status == TaskStatusWaiting && !task.Schedule().IsZero() && task.Schedule().Before(now)
			isRetryTask := taskInfo.Status == TaskStatusRetrying && !taskInfo.NextRetryAt.IsZero() && taskInfo.NextRetryAt.Before(now)

			if isScheduledTask || isRetryTask {
				// 创建任务副本以避免数据竞争
				taskCopy := task
				tasksToExecute = append(tasksToExecute, taskCopy)
			}
		}
	}
	s.taskMutex.RUnlock()

	// 执行符合条件的任务
	for _, task := range tasksToExecute {
		go s.executeTask(task)
	}
}

// cleanupExpiredTasks 清理过期任务
func (s *concurrentScheduler) cleanupExpiredTasks() {
	now := time.Now()
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()

	for taskID, task := range s.tasks {
		if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
			taskInfo := taskInfoProvider.TaskInfo()
			// 清理已完成且过期的任务
			if (taskInfo.Status == TaskStatusCompleted || taskInfo.Status == TaskStatusCancelled || taskInfo.Status == TaskStatusDeadLetter) && taskInfo.TTL.Before(now) {
				delete(s.tasks, taskID)
			}
		}
	}
}

// executeTask 执行任务
func (s *concurrentScheduler) executeTask(task Task) {
	// 简化的并发控制实现，使用轮询代替条件变量复杂的等待
	waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查并等待可用的并发槽位
	for {
		s.taskMutex.Lock()
		// 如果有可用槽位，直接执行
		if s.activeTasks < s.maxConcurrent {
			s.activeTasks++
			s.taskMutex.Unlock()
			break
		}
		s.taskMutex.Unlock()

		// 短暂睡眠后重试，避免CPU过度占用
		select {
		case <-time.After(100 * time.Millisecond):
		case <-waitCtx.Done():
			return
		}
	}

	// 任务完成后减少活跃任务数
	defer func() {
		s.taskMutex.Lock()
		s.activeTasks--
		s.activeTasksCond.Signal()
		s.taskMutex.Unlock()
	}()

	// 标记任务为运行中
	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		taskInfo := taskInfoProvider.TaskInfo()
		taskInfo.Status = TaskStatusRunning
		taskInfo.UpdatedAt = time.Now()
	}

	// 创建任务上下文，支持超时控制
	taskCtx := context.Background()
	timeout := task.GetTimeout()
	if timeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(taskCtx, timeout)
		defer cancel()
	}

	// 执行任务
	err := task.Handler()(taskCtx)

	// 处理任务执行结果
	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		taskInfo := taskInfoProvider.TaskInfo()
		taskInfo.UpdatedAt = time.Now()

		if err == nil {
			// 任务成功完成
			taskInfo.Status = TaskStatusCompleted

			// 处理周期性任务
			if task.IsRecurring() {
				interval := task.GetInterval()
				if interval > 0 {
					// 重新调度任务的逻辑将在后续实现
				}
			}
		} else {
			// 任务执行失败
			isContextCancelled := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)

			if isContextCancelled {
				// 超时或取消，不重试
				taskInfo.Status = TaskStatusCancelled
				taskInfo.LastError = "任务执行超时或被取消"
			} else if taskInfo.MaxRetries > 0 && taskInfo.RetryCount < taskInfo.MaxRetries {
				// 可以重试
				taskInfo.RetryCount++
				taskInfo.LastError = err.Error()
				taskInfo.Status = TaskStatusRetrying

				// 计算重试延迟（线性增长，最大500毫秒）
				retryDelay := time.Duration(100+taskInfo.RetryCount*100) * time.Millisecond
				if retryDelay > 500*time.Millisecond {
					retryDelay = 500 * time.Millisecond
				}
				taskInfo.NextRetryAt = time.Now().Add(retryDelay)
			} else {
				// 达到最大重试次数或不能重试
				taskInfo.Status = TaskStatusDeadLetter
				taskInfo.LastError = err.Error()
			}
		}
	}

	// 执行完成回调
	if completeFunc := task.OnComplete(); completeFunc != nil {
		go completeFunc(taskCtx, err)
	}
}
