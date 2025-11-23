package tasks

import (
	"context"
	"errors"
	"sync"
	"time"
)

// sequentialScheduler 顺序执行模式的任务调度器实现
// 每次只能执行一个任务，上一个任务执行完成之后，下一个任务才能开始执行
// 适合定时任务和周期性任务的执行

type sequentialScheduler struct {
	TaskScheduler
	tasks         map[string]Task // 存储所有任务
	taskMutex     sync.RWMutex    // 保护任务映射的互斥锁
	wg            sync.WaitGroup  // 等待所有任务完成的等待组
	isRunning     bool            // 是否有任务正在运行
	stopChan      chan struct{}   // 停止信号通道
	startOnce     sync.Once       // 确保调度器只启动一次
	taskTTL       time.Duration   // 任务完成后的保存时间（默认5分钟）
	maxTaskCount  int             // 最大保存的任务数量（默认500个）
	pendingTasks  []string        // 待执行的任务ID队列
	pendingMutex  sync.Mutex      // 保护待执行任务队列的互斥锁
	taskDoneChan  chan struct{}   // 任务完成通知通道
	taskQueue     chan Task       // 任务队列
	taskQueueClosed bool          // 任务队列是否已关闭
}

// NewSequentialScheduler 创建一个新的顺序执行模式任务调度器
func NewSequentialScheduler() TaskScheduler {
	ss := &sequentialScheduler{
		tasks:         make(map[string]Task),
		stopChan:      make(chan struct{}),
		taskTTL:       5 * time.Minute, // 任务完成后保存5分钟
		maxTaskCount:  500,             // 最大保存500个任务
		pendingTasks:  make([]string, 0),
		taskDoneChan:  make(chan struct{}, 1),
		taskQueue:     make(chan Task, 100), // 带缓冲的任务队列
		taskQueueClosed: false,
	}
	return ss
}

// Start 启动任务调度器
func (s *sequentialScheduler) Start(ctx context.Context) error {
	s.startOnce.Do(func() {
		// 保存上下文用于后续操作
		// 启动任务执行器和监视器
		go s.runTaskExecutor()
		go s.runTaskMonitor()
	})
	return nil
}

// Stop 停止任务调度器
func (s *sequentialScheduler) Stop() error {
	close(s.stopChan)
	s.wg.Wait() // 等待所有任务完成
	return nil
}

// Close 优雅关闭调度器并清理资源
func (s *sequentialScheduler) Close(ctx context.Context) error {
	// 安全关闭stopChan
	select {
	case <-s.stopChan:
		// 通道已关闭，不做操作
	default:
		close(s.stopChan)
	}

	// 安全关闭taskQueue
	s.pendingMutex.Lock()
	if !s.taskQueueClosed {
		close(s.taskQueue)
		s.taskQueueClosed = true
	}
	s.pendingMutex.Unlock()

	// 等待工作协程完成
	waitChan := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(waitChan)
	}()

	// 使用context进行超时控制
	select {
	case <-waitChan:
		// 工作协程已完成
	case <-ctx.Done():
		// 上下文取消，返回超时错误
		return ctx.Err()
	}

	// 安全关闭taskDoneChan
	select {
	case <-s.taskDoneChan:
		// 通道已关闭或已有数据，清理后关闭
	default:
		// 通道为空，直接关闭
	}
	close(s.taskDoneChan)

	return nil
}

// GracefulClose 优雅关闭任务调度器
func (s *sequentialScheduler) GracefulClose(ctx context.Context) error {
	s.Stop()
	return nil
}

// Schedule 调度一个任务
func (s *sequentialScheduler) Schedule(task Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}

	taskID := task.ID()

	// 检查任务数量限制
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()

	// 如果任务数量超过限制，删除最早的已完成任务
	if len(s.tasks) >= s.maxTaskCount {
		s.cleanupOldTasks()
	}

	// 存储任务
	s.tasks[taskID] = task

	// 将任务添加到待执行队列
	s.pendingMutex.Lock()
	s.pendingTasks = append(s.pendingTasks, taskID)
	s.pendingMutex.Unlock()

	// 通知任务执行器有新任务
	select {
	case s.taskDoneChan <- struct{}{}:
	default:
		// 如果通道已满，忽略，因为任务执行器会在下一次循环中处理
	}

	// 尝试将任务添加到任务队列
	s.pendingMutex.Lock()
	if !s.taskQueueClosed {
		select {
		case s.taskQueue <- task:
		default:
			// 如果队列已满，忽略，pendingTasks仍然有效作为备份
		}
	}
	s.pendingMutex.Unlock()

	return nil
}

// Cancel 取消指定的任务
func (s *sequentialScheduler) Cancel(taskID string) error {
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return errors.New("task not found")
	}

	// 调用任务的取消回调
	if onCancel := task.OnCancel(); onCancel != nil {
		go onCancel(context.Background())
	}

	// 标记任务为已取消
	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		taskInfo := taskInfoProvider.TaskInfo()
		taskInfo.Status = TaskStatusCancelled
		taskInfo.UpdatedAt = time.Now()
	}

	// 从待执行队列中移除
	s.pendingMutex.Lock()
	for i, id := range s.pendingTasks {
		if id == taskID {
			s.pendingTasks = append(s.pendingTasks[:i], s.pendingTasks[i+1:]...)
			break
		}
	}
	s.pendingMutex.Unlock()

	return nil
}

// GetTaskStatus 获取任务状态
func (s *sequentialScheduler) GetTaskStatus(taskID string) (TaskStatus, error) {
	s.taskMutex.RLock()
	defer s.taskMutex.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return TaskStatusUnknown, errors.New("task not found")
	}

	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		return taskInfoProvider.TaskInfo().Status, nil
	}

	return TaskStatusUnknown, errors.New("cannot get task status")
}

// ListTasks 列出所有任务
func (s *sequentialScheduler) ListTasks() []TaskInfo {
	s.taskMutex.RLock()
	defer s.taskMutex.RUnlock()

	var result []TaskInfo
	for _, task := range s.tasks {
		if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
			result = append(result, *taskInfoProvider.TaskInfo())
		}
	}
	return result
}

// SetMaxConcurrentTasks 设置最大并发任务数
func (s *sequentialScheduler) SetMaxConcurrentTasks(max int) error {
	// 在顺序执行模式下，最大并发任务数始终为1
	// 此方法主要是为了实现TaskScheduler接口
	return nil
}

// runTaskExecutor 运行任务执行器，按顺序执行任务
func (s *sequentialScheduler) runTaskExecutor() {
	defer func() {
		// 确保在退出前重置运行状态
		s.isRunning = false
	}()

	for {
		// 检查是否有任务需要执行
		var taskID string
		var task Task

		// 查找下一个需要执行的任务
		taskFound := false
		s.taskMutex.RLock()
		for _, id := range s.pendingTasks {
			currentTask, exists := s.tasks[id]
			if exists {
				if taskInfoProvider, ok := currentTask.(TaskInfoProvider); ok {
					taskInfo := taskInfoProvider.TaskInfo()
					// 只执行等待中的任务，并且任务时间已到或立即执行
					if taskInfo.Status == TaskStatusWaiting &&
						(currentTask.Schedule().IsZero() || time.Now().After(currentTask.Schedule())) {
						taskID = id
						task = currentTask
						taskFound = true
						break
					}
				}
			}
		}
		s.taskMutex.RUnlock()

		// 如果找到任务且当前没有任务在运行，则执行任务
		if taskFound && !s.isRunning {
			// 从待执行队列中移除
			s.pendingMutex.Lock()
			for i, id := range s.pendingTasks {
				if id == taskID {
					s.pendingTasks = append(s.pendingTasks[:i], s.pendingTasks[i+1:]...)
					break
				}
			}
			s.pendingMutex.Unlock()

			// 标记有任务正在运行
			s.isRunning = true

			// 执行任务
			s.executeTask(task)

			// 标记任务执行完成
			s.isRunning = false

			// 通知有任务完成
			select {
			case s.taskDoneChan <- struct{}{}:
			default:
				// 如果通道已满，忽略
			}
		} else {
			// 如果没有找到可执行的任务，等待通知
			select {
			case <-s.taskDoneChan:
				// 有任务完成或有新任务添加，继续循环
			case <-s.stopChan:
				// 停止信号
				return
			case <-time.After(1 * time.Second):
				// 超时，重新检查，避免死锁
			}
		}
	}
}

// runTaskMonitor 运行任务监视器，处理定时任务和清理过期任务
func (s *sequentialScheduler) runTaskMonitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 清理过期任务
			s.cleanupExpiredTasks()

			// 检查是否有定时任务时间已到
			now := time.Now()
			s.taskMutex.RLock()
			for _, task := range s.tasks {
				if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
					taskInfo := taskInfoProvider.TaskInfo()
					// 检查是否是等待中的定时任务，且时间已到但未在待执行队列中
					if taskInfo.Status == TaskStatusWaiting &&
						!task.Schedule().IsZero() &&
						now.After(task.Schedule()) &&
						!s.isTaskInPending(task.ID()) {
						// 将任务添加到待执行队列
						s.pendingMutex.Lock()
						s.pendingTasks = append(s.pendingTasks, task.ID())
						s.pendingMutex.Unlock()

						// 通知任务执行器
						select {
						case s.taskDoneChan <- struct{}{}:
						default:
						}
					}
				}
			}
			s.taskMutex.RUnlock()

		case <-s.stopChan:
			// 停止信号
			return
		}
	}
}

// isTaskInPending 检查任务是否在待执行队列中
func (s *sequentialScheduler) isTaskInPending(taskID string) bool {
	s.pendingMutex.Lock()
	defer s.pendingMutex.Unlock()

	for _, id := range s.pendingTasks {
		if id == taskID {
			return true
		}
	}
	return false
}

// cleanupExpiredTasks 清理过期任务
func (s *sequentialScheduler) cleanupExpiredTasks() {
	now := time.Now()

	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()

	for taskID, task := range s.tasks {
		if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
			taskInfo := taskInfoProvider.TaskInfo()
			// 检查是否是已完成、已取消或失败的任务，且超过了TTL
			if (taskInfo.Status == TaskStatusCompleted || taskInfo.Status == TaskStatusCancelled || taskInfo.Status == TaskStatusFailed) &&
				!taskInfo.UpdatedAt.IsZero() && now.Sub(taskInfo.UpdatedAt) > s.taskTTL {
				delete(s.tasks, taskID)

				// 从待执行队列中移除
				s.pendingMutex.Lock()
				for i, id := range s.pendingTasks {
					if id == taskID {
						s.pendingTasks = append(s.pendingTasks[:i], s.pendingTasks[i+1:]...)
						break
					}
				}
				s.pendingMutex.Unlock()
			}
		}
	}
}

// cleanupOldTasks 清理旧任务以腾出空间
func (s *sequentialScheduler) cleanupOldTasks() {
	// 简单实现：删除最早更新的已完成任务
	var oldestTaskID string
	var oldestUpdateTime time.Time

	for taskID, task := range s.tasks {
		if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
			taskInfo := taskInfoProvider.TaskInfo()
			// 只清理已完成、已取消或失败的任务
			if taskInfo.Status == TaskStatusCompleted || taskInfo.Status == TaskStatusCancelled || taskInfo.Status == TaskStatusFailed {
				if oldestTaskID == "" || taskInfo.UpdatedAt.Before(oldestUpdateTime) {
					oldestTaskID = taskID
					oldestUpdateTime = taskInfo.UpdatedAt
				}
			}
		}
	}

	if oldestTaskID != "" {
		delete(s.tasks, oldestTaskID)

		// 从待执行队列中移除
		s.pendingMutex.Lock()
		for i, id := range s.pendingTasks {
			if id == oldestTaskID {
				s.pendingTasks = append(s.pendingTasks[:i], s.pendingTasks[i+1:]...)
				break
			}
		}
		s.pendingMutex.Unlock()
	}
}

// executeTask 执行任务
func (s *sequentialScheduler) executeTask(task Task) {
	s.wg.Add(1)
	defer s.wg.Done()

	// 标记任务为运行中
	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		taskInfo := taskInfoProvider.TaskInfo()
		taskInfo.Status = TaskStatusRunning
		taskInfo.UpdatedAt = time.Now()
	}

	// 处理任务超时
	ctx := context.Background()
	if timeout := task.GetTimeout(); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 执行任务
	err := task.Handler()(ctx)

	// 更新任务状态
	if taskInfoProvider, ok := task.(TaskInfoProvider); ok {
		taskInfo := taskInfoProvider.TaskInfo()
		taskInfo.UpdatedAt = time.Now()

		if err != nil {
			// 任务失败，处理重试
			taskInfo.LastError = err.Error()
			if taskInfo.MaxRetries > taskInfo.RetryCount {
				// 进行重试
				taskInfo.Status = TaskStatusRetrying
				taskInfo.RetryCount++
				// 指数退避重试
				retryDelay := time.Duration(1<<uint(taskInfo.RetryCount-1)) * time.Second
				if retryDelay > 1*time.Minute {
					retryDelay = 1 * time.Minute // 最大重试间隔为1分钟
				}
				taskInfo.NextRetryAt = time.Now().Add(retryDelay)

				// 使用新的调度时间重新调度任务
				if builtTask, ok := task.(*builtTask); ok {
					builtTask.SetSchedule(taskInfo.NextRetryAt)
				}

				// 将任务重新添加到待执行队列
				s.pendingMutex.Lock()
				s.pendingTasks = append(s.pendingTasks, task.ID())
				s.pendingMutex.Unlock()
			} else {
				// 达到最大重试次数
				taskInfo.Status = TaskStatusFailed
			}
		} else {
			// 任务成功完成
			taskInfo.Status = TaskStatusCompleted

			// 如果是周期性任务，安排下一次执行
			if task.IsRecurring() {
				nextTime := time.Now().Add(task.GetInterval())
				taskInfo.Status = TaskStatusWaiting
				taskInfo.Schedule = nextTime
				if builtTask, ok := task.(*builtTask); ok {
					builtTask.SetSchedule(nextTime)
				}

				// 将任务重新添加到待执行队列
				s.pendingMutex.Lock()
				s.pendingTasks = append(s.pendingTasks, task.ID())
				s.pendingMutex.Unlock()
			}
		}
	}

	// 调用完成回调
	if onComplete := task.OnComplete(); onComplete != nil {
		onComplete(context.Background(), err)
	}
}
