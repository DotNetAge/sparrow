package tasks

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Options 调度器配置选项
type Options struct {
	WorkerCount        int
	MaxConcurrentTasks int
}

// Option 配置函数类型
type Option func(*Options)

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

// MemoryTaskScheduler 基于内存的任务调度器实现
type MemoryTaskScheduler struct {
	tasks              map[string]*taskWrapper       // 存储所有任务
	taskQueue          *priorityQueue                // 优先队列，按执行时间排序
	mu                 sync.RWMutex                  // 互斥锁，保证并发安全
	stopChan           chan struct{}                 // 停止信号通道
	wg                 sync.WaitGroup                // 等待组，用于优雅关闭
	started            bool                          // 调度器状态标志
	workerCount        int                           // 工作协程数量
	workerPool         chan struct{}                 // 工作池，控制并发执行
	maxConcurrentTasks int                           // 最大并发任务数
	cancels            map[string]context.CancelFunc // 存储执行中任务的取消函数
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
		workerPool:         make(chan struct{}, options.MaxConcurrentTasks),
		cancels:            make(map[string]context.CancelFunc),
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
	// 重置取消函数映射
	s.cancels = make(map[string]context.CancelFunc)

	// 启动多个工作协程
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.run()
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

	// 标记为已停止，不再接受新任务
	s.started = false

	// 先取消所有执行中的任务，不等待它们完成
	for taskID, cancel := range s.cancels {
		cancel()
		// 更新任务状态为取消
		if task, exists := s.tasks[taskID]; exists {
			task.status = TaskStatusCancelled
			task.updatedAt = time.Now()
		}
		// 移除已取消的任务
		delete(s.cancels, taskID)
	}

	// 清空任务队列，确保不再调度新任务
	for len(s.taskQueue.items) > 0 {
		task := s.taskQueue.Pop()
		// 更新任务状态为取消
		task.status = TaskStatusCancelled
		task.updatedAt = time.Now()
		// 执行取消回调
		if onComplete := task.task.OnComplete(); onComplete != nil {
			ctx := context.Background()
			go onComplete(ctx, errors.New("task cancelled during shutdown"))
		}
	}

	// 关闭停止信号通道
	close(s.stopChan)

	// 释放锁，让工作协程能够退出
	s.mu.Unlock()

	return nil
}

// 停止时清空worker池
func (s *MemoryTaskScheduler) drainWorkerPool() {
	// 释放所有正在等待的worker
	for len(s.workerPool) > 0 {
		<-s.workerPool
	}
}

// Close 优雅关闭任务调度器
func (s *MemoryTaskScheduler) Close() error {
	if err := s.Stop(); err != nil {
		return err
	}

	// 等待所有工作协程退出，但已取消的任务不会等待它们完成
	s.wg.Wait()
	return nil
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

	// 如果调度器已停止，拒绝新任务
	if !s.started {
		return fmt.Errorf("调度器已停止，无法调度新任务")
	}

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

	// 取消正在运行的任务
	if cancel, exists := s.cancels[taskID]; exists {
		cancel()
		delete(s.cancels, taskID)
	}

	// 更新状态
	oldStatus := wrapper.status
	wrapper.status = TaskStatusCancelled
	wrapper.updatedAt = time.Now()

	// 从队列中移除等待中的任务
	if oldStatus == TaskStatusWaiting {
		newItems := make([]*taskWrapper, 0, len(s.taskQueue.items))
		for _, item := range s.taskQueue.items {
			if item.task.ID() != taskID {
				newItems = append(newItems, item)
			}
		}
		s.taskQueue.items = newItems
	}

	// 触发取消回调
	if onCancel := wrapper.task.OnCancel(); onCancel != nil {
		ctx := context.Background()
		// 立即执行回调，不使用goroutine以确保测试中能正确检测
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
		return "", fmt.Errorf("任务不存在: %s", taskID)
	}

	return wrapper.status, nil
}

// ListTasks 列出所有任务
func (s *MemoryTaskScheduler) ListTasks() []TaskInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]TaskInfo, 0, len(s.tasks))
	for _, wrapper := range s.tasks {
		info := TaskInfo{
			ID:        wrapper.task.ID(),
			Type:      wrapper.task.Type(),
			Status:    wrapper.status,
			Schedule:  wrapper.task.Schedule(),
			CreatedAt: wrapper.createdAt,
			UpdatedAt: wrapper.updatedAt,
		}
		result = append(result, info)
	}

	return result
}

// SetMaxConcurrentTasks 设置最大并发任务数
func (s *MemoryTaskScheduler) SetMaxConcurrentTasks(max int) error {
	if max <= 0 {
		return fmt.Errorf("最大并发任务数必须大于0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建新的工作池
	newPool := make(chan struct{}, max)
	currentRunning := len(s.workerPool)

	// 复制当前运行的任务令牌
	for i := 0; i < currentRunning; i++ {
		newPool <- struct{}{}
	}

	s.maxConcurrentTasks = max
	s.workerPool = newPool

	return nil
}

// run 任务调度核心逻辑
func (s *MemoryTaskScheduler) run() {
	for {
		// 先检查是否收到停止信号
		select {
		case <-s.stopChan:
			return
		default:
			// 继续执行
		}

		s.mu.Lock()
		// 检查调度器是否已停止
		if !s.started {
			s.mu.Unlock()
			return
		}

		// 检查任务队列是否为空
		if s.taskQueue.Len() == 0 {
			s.mu.Unlock()
			// 等待一段时间后再次检查
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// 获取下一个要执行的任务
		nextTask := s.taskQueue.Peek()
		waitTime := time.Until(nextTask.task.Schedule())

		if waitTime <= 0 {
			// 时间到，执行任务
			task := s.taskQueue.Pop()
			task.status = TaskStatusRunning
			task.updatedAt = time.Now()
			s.mu.Unlock()
			// 执行任务
			s.executeTask(task)
		} else {
			// 还没到执行时间，释放锁并等待
			s.mu.Unlock()
			// 等待直到任务执行时间或停止信号
			select {
			case <-time.After(waitTime):
				// 时间到，继续循环检查
			case <-s.stopChan:
				// 收到停止信号，退出循环
				return
			}
		}
	}
}

// executeTask 任务执行函数
func (s *MemoryTaskScheduler) executeTask(t *taskWrapper) {
	// 获取工作池中的令牌，限制并发数
	s.workerPool <- struct{}{}
	s.wg.Add(1)

	taskID := t.task.ID()

	go func(task *taskWrapper, taskID string) {
		defer func() {
			<-s.workerPool
			s.wg.Done()

			// 任务完成后，从cancels映射中移除
			s.mu.Lock()
			delete(s.cancels, taskID)
			s.mu.Unlock()

			if r := recover(); r != nil {
				// 检查任务是否已经被取消
				s.mu.RLock()
				currentStatus := task.status
				s.mu.RUnlock()

				if currentStatus != TaskStatusCancelled {
					s.updateTaskStatus(task, TaskStatusFailed)

					if onComplete := task.task.OnComplete(); onComplete != nil {
						ctx := context.Background()
						onComplete(ctx, fmt.Errorf("task panic: %v", r))
					}
				}
			}
		}()

		// 检查任务是否已经被取消
		s.mu.RLock()
		if task.status == TaskStatusCancelled || !s.started {
			s.mu.RUnlock()
			return
		}
		s.mu.RUnlock()

		// 创建可取消的上下文
		ctx, cancel := context.WithCancel(context.Background())

		// 保存取消函数，以便在停止时能够取消执行中的任务
		s.mu.Lock()
		// 再次检查状态，防止在获取锁的过程中状态发生变化
		if task.status == TaskStatusCancelled || !s.started {
			s.mu.Unlock()
			cancel()
			return
		}
		s.cancels[taskID] = cancel
		task.cancelFunc = cancel
		s.mu.Unlock()

		// 执行任务处理器
		err := task.task.Handler()(ctx)

		// 检查是否是上下文取消错误，如果是，直接返回
		if errors.Is(err, context.Canceled) {
			// 已经在Stop方法中更新了状态，这里不需要再次更新
			return
		}

		// 检查任务是否已经被取消
		s.mu.RLock()
		if task.status == TaskStatusCancelled {
			s.mu.RUnlock()
			return
		}
		s.mu.RUnlock()

		// 根据执行结果设置任务状态
		status := TaskStatusCompleted
		if err != nil {
			status = TaskStatusFailed
		}

		// 更新任务状态
		s.updateTaskStatus(task, status)

		// 触发完成回调
		if onComplete := task.task.OnComplete(); onComplete != nil {
			onComplete(ctx, err)
		}

		// 处理周期性任务的重新调度
		if task.task.IsRecurring() && status != TaskStatusCancelled {
			// 检查调度器是否仍在运行
			s.mu.RLock()
			if !s.started {
				s.mu.RUnlock()
				return
			}
			s.mu.RUnlock()

			nextSchedule := time.Now().Add(task.task.GetInterval())

			// 创建新的任务实例
			newTask := &builtTask{
				id:         uuid.New().String(), // 生成新的ID
				typeName:   task.task.Type(),
				execTime:   nextSchedule,
				handler:    task.task.Handler(),
				onComplete: task.task.OnComplete(),
				onCancel:   task.task.OnCancel(),
				schedule:   task.task.(*builtTask).schedule,
			}

			// 重新调度任务
			s.Schedule(newTask)
		}
	}(t, taskID)
}

// updateTaskStatus 更新任务状态
func (s *MemoryTaskScheduler) updateTaskStatus(task *taskWrapper, status TaskStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if wrapper, exists := s.tasks[task.task.ID()]; exists {
		wrapper.status = status
		wrapper.updatedAt = time.Now()
	}
}

// Len 返回队列长度
func (q *priorityQueue) Len() int {
	return len(q.items)
}

// Push 添加任务到队列
func (q *priorityQueue) Push(task *taskWrapper) {
	q.items = append(q.items, task)
	// 按执行时间排序
	sort.Slice(q.items, func(i, j int) bool {
		return q.items[i].task.Schedule().Before(q.items[j].task.Schedule())
	})
}

// Pop 取出队首任务
func (q *priorityQueue) Pop() *taskWrapper {
	if len(q.items) == 0 {
		return nil
	}

	task := q.items[0]
	q.items = q.items[1:]
	return task
}

// Peek 查看队首任务但不取出
func (q *priorityQueue) Peek() *taskWrapper {
	if len(q.items) == 0 {
		return nil
	}
	return q.items[0]
}
