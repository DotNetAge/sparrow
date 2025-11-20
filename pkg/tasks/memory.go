package tasks

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// Options 调度器配置选项
type Options struct {
	WorkerCount        int
	MaxConcurrentTasks int
	Logger             *logger.Logger
	CleanupPolicy      *CleanupPolicy
}

// Option 配置函数类型
type Option func(*Options)

// WithLogger 设置日志记录器
func WithLogger(logger *logger.Logger) Option {
	return func(o *Options) {
		o.Logger = logger
	}
}

// WithCleanupPolicy 设置清理策略
func WithCleanupPolicy(policy *CleanupPolicy) Option {
	return func(o *Options) {
		o.CleanupPolicy = policy
	}
}

// WithWorkerCount 设置工作协程数量
func WithWorkerCount(count int) Option {
	return func(o *Options) {
		o.WorkerCount = count
	}
}

// WithMaxConcurrentTasks 设置最大并发任务数
func WithMaxConcurrentTasks(max int) Option {
	return func(o *Options) {
		o.MaxConcurrentTasks = max
	}
}

// MemoryTaskScheduler 简化的内存任务调度器
type MemoryTaskScheduler struct {
	usecase.GracefulClose
	usecase.Startable
	Logger             *logger.Logger
	tasks              map[string]*taskWrapper       // 存储所有任务
	taskQueue          *priorityQueue                // 优先队列，按执行时间排序
	mu                 sync.RWMutex                  // 互斥锁，保证并发安全
	stopChan           chan struct{}                 // 停止信号通道
	wg                 sync.WaitGroup                // 等待组，用于优雅关闭
	started            bool                          // 调度器状态标志
	workerCount        int                           // 工作协程数量
	workerPool         chan struct{}                 // 工作池，控制并发执行
	maxConcurrentTasks int                           // 最大并发任务数
	runningTasks       int                           // 当前运行的任务数
	cancels            map[string]context.CancelFunc // 存储执行中任务的取消函数
	retryMonitor       *RetryMonitor                 // 重试监控器
	executionMode      ExecutionMode                 // 执行模式
	
	// 清理机制相关字段
	cleanupPolicy      *CleanupPolicy
	cleanupStopChan    chan struct{}
	cleanupStarted     bool
}

// taskWrapper 任务包装器
type taskWrapper struct {
	task       Task               // 原始任务
	status     TaskStatus         // 任务状态
	cancelFunc context.CancelFunc // 取消函数
	createdAt  time.Time          // 创建时间
	updatedAt  time.Time          // 更新时间
}

// priorityQueue 优先队列
type priorityQueue struct {
	items []*taskWrapper
}

// NewMemoryTaskScheduler 创建新的内存任务调度器
func NewMemoryTaskScheduler(opts ...Option) *MemoryTaskScheduler {
	options := &Options{
		WorkerCount:        5,
		MaxConcurrentTasks: 10,
		CleanupPolicy:      DefaultCleanupPolicy(),
	}

	for _, opt := range opts {
		opt(options)
	}

	return &MemoryTaskScheduler{
		tasks:              make(map[string]*taskWrapper),
		taskQueue:          &priorityQueue{items: []*taskWrapper{}},
		stopChan:           make(chan struct{}),
		workerCount:        options.WorkerCount,
		maxConcurrentTasks: options.MaxConcurrentTasks,
		Logger:             options.Logger,
		workerPool:         make(chan struct{}, options.WorkerCount),
		cancels:            make(map[string]context.CancelFunc),
		retryMonitor:       NewRetryMonitor(),
		cleanupPolicy:      options.CleanupPolicy,
		cleanupStopChan:    make(chan struct{}),
		executionMode:      ExecutionModeConcurrent, // 默认并发模式
	}
}

// Start 启动任务调度器
func (s *MemoryTaskScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("调度器已经启动")
	}

	s.started = true
	s.stopChan = make(chan struct{})
	s.cancels = make(map[string]context.CancelFunc)

	// 启动多个工作协程
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.run()
		}()
	}

	// 启动清理协程
	if s.cleanupPolicy != nil && s.cleanupPolicy.EnableAutoCleanup {
		s.cleanupStarted = true
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runCleanup()
		}()
	}

	return nil
}

// Stop 停止任务调度器
func (s *MemoryTaskScheduler) Stop() error {
	s.mu.Lock()

	if !s.started {
		s.mu.Unlock()
		return fmt.Errorf("调度器未启动")
	}

	s.started = false

	// 停止清理协程
	if s.cleanupStarted {
		select {
		case <-s.cleanupStopChan:
		default:
			close(s.cleanupStopChan)
		}
		s.cleanupStarted = false
	}

	// 取消所有执行中的任务
	for taskID, cancel := range s.cancels {
		cancel()
		if task, exists := s.tasks[taskID]; exists {
			task.status = TaskStatusCancelled
			task.updatedAt = time.Now()
		}
		delete(s.cancels, taskID)
	}

	// 清空任务队列
	for len(s.taskQueue.items) > 0 {
		task := s.taskQueue.Pop()
		task.status = TaskStatusCancelled
		task.updatedAt = time.Now()
		if onComplete := task.task.OnComplete(); onComplete != nil {
			ctx := context.Background()
			go onComplete(ctx, errors.New("task cancelled during shutdown"))
		}
	}

	select {
	case <-s.stopChan:
	default:
		close(s.stopChan)
	}

	s.mu.Unlock()
	return nil
}

// Close 优雅关闭任务调度器
func (s *MemoryTaskScheduler) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	
	if err := s.Stop(); err != nil {
		return err
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		if s.Logger != nil {
			s.Logger.Warn("任务调度器关闭超时，强制退出")
		}
		return fmt.Errorf("任务调度器关闭超时: %w", ctx.Err())
	}
}

// Schedule 调度一个任务
func (s *MemoryTaskScheduler) Schedule(task Task) error {
	if task == nil {
		return fmt.Errorf("任务不能为空")
	}
	if task.Handler() == nil {
		return fmt.Errorf("任务处理函数不能为空")
	}

	wrapper := &taskWrapper{
		task:      task,
		status:    TaskStatusWaiting,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID()]; exists {
		return fmt.Errorf("任务ID已存在: %s", task.ID())
	}

	s.tasks[task.ID()] = wrapper
	s.taskQueue.Push(wrapper)

	return nil
}

// Cancel 取消指定的任务
func (s *MemoryTaskScheduler) Cancel(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wrapper, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	if wrapper.status == TaskStatusCompleted || wrapper.status == TaskStatusFailed {
		return fmt.Errorf("任务已完成或失败，无法取消")
	}

	if cancel, exists := s.cancels[taskID]; exists {
		cancel()
		delete(s.cancels, taskID)
	}

	oldStatus := wrapper.status
	wrapper.status = TaskStatusCancelled
	wrapper.updatedAt = time.Now()

	if oldStatus == TaskStatusWaiting {
		newItems := make([]*taskWrapper, 0, len(s.taskQueue.items))
		for _, item := range s.taskQueue.items {
			if item.task.ID() != taskID {
				newItems = append(newItems, item)
			}
		}
		s.taskQueue.items = newItems
	}

	if onCancel := wrapper.task.OnCancel(); onCancel != nil {
		ctx := context.Background()
		onCancel(ctx)
	}

	return nil
}

// GetTaskStatus 获取任务状态
func (s *MemoryTaskScheduler) GetTaskStatus(taskID string) (TaskStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wrapper, exists := s.tasks[taskID]
	if !exists {
		return TaskStatusUnknown, fmt.Errorf("任务不存在: %s", taskID)
	}

	return wrapper.status, nil
}

// run 调度器主循环
func (s *MemoryTaskScheduler) run() {
	for {
		select {
		case <-s.stopChan:
			return
		default:
			s.processNextTask()
		}
	}
}

// processNextTask 处理下一个任务
func (s *MemoryTaskScheduler) processNextTask() {
	s.mu.Lock()

	// 检查当前运行的任务数是否达到最大并发限制
	if s.runningTasks >= s.maxConcurrentTasks {
		s.mu.Unlock()
		time.Sleep(10 * time.Millisecond) // 短暂等待
		return
	}

	// 检查是否有到期的任务
	now := time.Now()
	if len(s.taskQueue.items) == 0 || s.taskQueue.items[0].task.Schedule().After(now) {
		s.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		return
	}

	wrapper := s.taskQueue.Pop()
	s.runningTasks++ // 增加运行任务计数
	s.mu.Unlock()

	// 执行任务
	s.executeTask(wrapper)
}

// executeTask 执行任务
func (s *MemoryTaskScheduler) executeTask(wrapper *taskWrapper) {
	// 获取工作池令牌
	s.workerPool <- struct{}{}
	defer func() { 
		<-s.workerPool
		// 减少运行任务计数
		s.mu.Lock()
		s.runningTasks--
		s.mu.Unlock()
	}()

	// 创建任务上下文
	taskCtx, cancel := context.WithCancel(context.Background())
	
	s.mu.Lock()
	s.cancels[wrapper.task.ID()] = cancel
	wrapper.status = TaskStatusRunning
	wrapper.updatedAt = time.Now()
	s.mu.Unlock()

	startTime := time.Now()
	
	// 添加panic恢复机制
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				// 将panic转换为error
				switch v := r.(type) {
				case error:
					err = v
				case string:
					err = errors.New(v)
				default:
					err = fmt.Errorf("panic: %v", v)
				}
				if s.Logger != nil {
					s.Logger.Error(fmt.Sprintf("任务 %s 发生panic: %v", wrapper.task.ID(), r))
				}
			}
		}()
		
		err = wrapper.task.Handler()(taskCtx)
	}()
	
	duration := time.Since(startTime)

	s.mu.Lock()
	delete(s.cancels, wrapper.task.ID())
	
	// 检查任务是否已被取消
	if wrapper.status == TaskStatusCancelled {
		// 任务已被取消，不需要进一步处理
		s.mu.Unlock()
	} else if err != nil {
		if retryableTask, ok := wrapper.task.(RetryableTask); ok {
			// 处理可重试任务
			s.handleRetryableTask(retryableTask, wrapper, err, duration)
		} else {
			// 普通任务失败
			wrapper.status = TaskStatusFailed
			wrapper.updatedAt = time.Now()
			if s.retryMonitor != nil {
				s.retryMonitor.RecordRetry(false, duration)
			}
		}
		s.mu.Unlock()
	} else {
		// 任务成功
		if wrapper.task.IsRecurring() {
			// 循环任务，重新调度下一次执行
			nextExecTime := time.Now().Add(wrapper.task.GetInterval())
			
			// 更新任务的执行时间
			if schedulableTask, ok := wrapper.task.(interface{ SetSchedule(time.Time) }); ok {
				schedulableTask.SetSchedule(nextExecTime)
			}
			
			wrapper.status = TaskStatusWaiting
			wrapper.updatedAt = time.Now()
			s.taskQueue.Push(wrapper)
			
			if s.Logger != nil {
				s.Logger.Info(fmt.Sprintf("循环任务 %s 已调度下次执行，时间: %v", wrapper.task.ID(), nextExecTime))
			}
		} else {
			// 普通任务成功
			wrapper.status = TaskStatusCompleted
			wrapper.updatedAt = time.Now()
		}
		
		if s.retryMonitor != nil {
			s.retryMonitor.RecordRetry(true, duration)
		}
		s.mu.Unlock()
	}

	// 执行完成回调
	if onComplete := wrapper.task.OnComplete(); onComplete != nil {
		go onComplete(context.Background(), err)
	}
}

// handleRetryableTask 处理可重试任务
func (s *MemoryTaskScheduler) handleRetryableTask(task RetryableTask, wrapper *taskWrapper, err error, duration time.Duration) {
	retryInfo := task.GetRetryInfo()
	policy := task.GetRetryPolicy()
	
	if retryInfo == nil {
		retryInfo = NewRetryInfo()
		task.SetRetryInfo(retryInfo)
	}

	// 增加重试计数
	nextAttempt := retryInfo.CurrentRetry + 1
	retryInfo.AddRetryAttempt(nextAttempt, err, duration)

	// 判断是否应该重试
	if policy.ShouldRetry(retryInfo.CurrentRetry, err) {
		// 计算下次重试时间
		backoffTime := policy.CalculateBackoff(retryInfo.CurrentRetry, BackoffStrategyExponential)
		nextRetryAt := time.Now().Add(backoffTime)
		
		retryInfo.NextRetryAt = nextRetryAt
		
		// 重新调度任务
		wrapper.status = TaskStatusRetrying
		wrapper.updatedAt = time.Now()
		
		// 更新任务的调度时间
		if schedulableTask, ok := wrapper.task.(interface{ SetSchedule(time.Time) }); ok {
			schedulableTask.SetSchedule(nextRetryAt)
		}
		
		s.taskQueue.Push(wrapper)
		
		if s.Logger != nil {
			s.Logger.Info(fmt.Sprintf("任务 %s 将在 %v 后重试 (第%d次)", 
				task.ID(), backoffTime, retryInfo.CurrentRetry))
		}
	} else {
		// 不再重试，标记为死信
		wrapper.status = TaskStatusDeadLetter
		wrapper.updatedAt = time.Now()
		
		if s.retryMonitor != nil {
			s.retryMonitor.RecordDeadLetter()
		}
		
		if s.Logger != nil {
			s.Logger.Error(fmt.Sprintf("任务 %s 重试失败，达到最大重试次数", task.ID()))
		}
	}
}

// GetRetryStats 获取重试统计
func (s *MemoryTaskScheduler) GetRetryStats() map[string]interface{} {
	if s.retryMonitor != nil {
		return s.retryMonitor.GetStats()
	}
	return nil
}

// ListTasks 列出所有任务
func (s *MemoryTaskScheduler) ListTasks() []TaskInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]TaskInfo, 0, len(s.tasks))
	for _, wrapper := range s.tasks {
		tasks = append(tasks, TaskInfo{
			ID:        wrapper.task.ID(),
			Type:      wrapper.task.Type(),
			Status:    wrapper.status,
			Schedule:  wrapper.task.Schedule(),
			CreatedAt: wrapper.createdAt,
			UpdatedAt: wrapper.updatedAt,
		})
	}

	return tasks
}

// SetMaxConcurrentTasks 设置最大并发任务数
func (s *MemoryTaskScheduler) SetMaxConcurrentTasks(max int) error {
	if max <= 0 {
		return fmt.Errorf("最大并发任务数必须大于0")
	}
	
	s.mu.Lock()
	s.maxConcurrentTasks = max
	s.mu.Unlock()
	
	// 如果当前运行的任务数超过新的限制，不会立即停止现有任务
	// 但新任务会被限制
	return nil
}

// SetExecutionMode 设置执行模式
func (s *MemoryTaskScheduler) SetExecutionMode(mode ExecutionMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.executionMode = mode
	
	// 根据执行模式调整工作协程数
	switch mode {
	case ExecutionModeSequential, ExecutionModePipeline:
		// 顺序和流水线模式只需要1个工作协程
		s.workerCount = 1
		s.maxConcurrentTasks = 1
		// 重新创建工作池
		s.workerPool = make(chan struct{}, 1)
	case ExecutionModeConcurrent:
		// 并发模式恢复原始设置
		s.workerCount = 10 // 默认值
		s.maxConcurrentTasks = 100 // 默认值
		// 重新创建工作池
		s.workerPool = make(chan struct{}, s.workerCount)
	}
	
	if s.Logger != nil {
		modeStr := map[ExecutionMode]string{
			ExecutionModeConcurrent: "并发",
			ExecutionModeSequential: "顺序",
			ExecutionModePipeline:  "流水线",
		}[mode]
		s.Logger.Info(fmt.Sprintf("执行模式已设置为: %s", modeStr))
	}
	
	return nil
}

// GetExecutionMode 获取当前执行模式
func (s *MemoryTaskScheduler) GetExecutionMode() ExecutionMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.executionMode
}

// 优先队列方法
func (pq *priorityQueue) Push(wrapper *taskWrapper) {
	pq.items = append(pq.items, wrapper)
	sort.Slice(pq.items, func(i, j int) bool {
		return pq.items[i].task.Schedule().Before(pq.items[j].task.Schedule())
	})
}

func (pq *priorityQueue) Pop() *taskWrapper {
	if len(pq.items) == 0 {
		return nil
	}
	
	item := pq.items[0]
	pq.items = pq.items[1:]
	return item
}

// runCleanup 运行清理协程
func (s *MemoryTaskScheduler) runCleanup() {
	ticker := time.NewTicker(s.cleanupPolicy.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanupStopChan:
			return
		case <-ticker.C:
			s.cleanupExpiredTasks()
		}
	}
}

// cleanupExpiredTasks 清理过期任务
func (s *MemoryTaskScheduler) cleanupExpiredTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cleanupPolicy == nil {
		return
	}

	var tasksToDelete []string

	// 统计各状态任务数量
	statusCounts := make(map[TaskStatus]int)
	for _, wrapper := range s.tasks {
		statusCounts[wrapper.status]++
	}

	// 找出需要清理的任务
	for taskID, wrapper := range s.tasks {
		if s.cleanupPolicy.ShouldCleanup(wrapper.status, wrapper.createdAt, wrapper.updatedAt, statusCounts[wrapper.status]) {
			tasksToDelete = append(tasksToDelete, taskID)
		}
	}

	// 执行清理
	deletedCount := 0
	for _, taskID := range tasksToDelete {
		if wrapper, exists := s.tasks[taskID]; exists {
			delete(s.tasks, taskID)
			deletedCount++
			
			if s.Logger != nil {
				s.Logger.Debug(fmt.Sprintf("清理过期任务: %s (状态: %s, 创建时间: %v)", 
					taskID, wrapper.status, wrapper.createdAt))
			}
		}
	}

	if deletedCount > 0 && s.Logger != nil {
		s.Logger.Info(fmt.Sprintf("清理了 %d 个过期任务，剩余任务数: %d", deletedCount, len(s.tasks)))
	}
}

// CleanupTasks 手动清理任务
func (s *MemoryTaskScheduler) CleanupTasks() int {
	s.cleanupExpiredTasks()
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}

// SetCleanupPolicy 设置清理策略
func (s *MemoryTaskScheduler) SetCleanupPolicy(policy *CleanupPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupPolicy = policy

	// 如果调度器已运行且策略启用自动清理，启动清理协程
	if s.started && policy.EnableAutoCleanup && !s.cleanupStarted {
		s.cleanupStarted = true
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runCleanup()
		}()
	}

	// 如果策略禁用自动清理且清理协程正在运行，停止它
	if s.cleanupStarted && (!policy.EnableAutoCleanup) {
		select {
		case <-s.cleanupStopChan:
		default:
			close(s.cleanupStopChan)
		}
		s.cleanupStarted = false
		// 重新创建stopChan以备下次启用
		s.cleanupStopChan = make(chan struct{})
	}

	return nil
}

// GetCleanupStats 获取清理统计信息
func (s *MemoryTaskScheduler) GetCleanupStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statusCounts := make(map[TaskStatus]int)
	oldestTask := make(map[TaskStatus]time.Time)
	newestTask := make(map[TaskStatus]time.Time)

	for _, wrapper := range s.tasks {
		statusCounts[wrapper.status]++
		
		if oldest, exists := oldestTask[wrapper.status]; !exists || wrapper.createdAt.Before(oldest) {
			oldestTask[wrapper.status] = wrapper.createdAt
		}
		
		if newest, exists := newestTask[wrapper.status]; !exists || wrapper.updatedAt.After(newest) {
			newestTask[wrapper.status] = wrapper.updatedAt
		}
	}

	return map[string]interface{}{
		"total_tasks":     len(s.tasks),
		"status_counts":   statusCounts,
		"oldest_tasks":    oldestTask,
		"newest_tasks":    newestTask,
		"cleanup_policy": map[string]interface{}{
			"enable_auto_cleanup":  s.cleanupPolicy.EnableAutoCleanup,
			"completed_task_ttl":   s.cleanupPolicy.CompletedTaskTTL.String(),
			"failed_task_ttl":      s.cleanupPolicy.FailedTaskTTL.String(),
			"cancelled_task_ttl":   s.cleanupPolicy.CancelledTaskTTL.String(),
			"max_completed_tasks":  s.cleanupPolicy.MaxCompletedTasks,
			"max_failed_tasks":     s.cleanupPolicy.MaxFailedTasks,
			"cleanup_interval":     s.cleanupPolicy.CleanupInterval.String(),
		},
	}
}
