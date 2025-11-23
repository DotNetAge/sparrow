package tasks

import (
	"context"
	"sync"
	"time"
)

// HybridScheduler 混合调度器实现
// 根据任务类型选择不同的执行模式（并发或顺序）
// 定时任务和周期任务默认使用顺序模式执行
// 可以通过配置指定任务类型与执行模式的映射关系

type HybridScheduler struct {
	TaskScheduler
	concurrentScheduler TaskScheduler            // 并发执行模式的调度器
	sequentialScheduler TaskScheduler            // 顺序执行模式的调度器
	executionModes      map[string]ExecutionMode // 任务类型到执行模式的映射
	modesMutex          sync.RWMutex             // 保护执行模式映射的互斥锁
}

// NewHybridScheduler 创建一个新的混合调度器
func NewHybridScheduler() *HybridScheduler {
	hs := &HybridScheduler{
		concurrentScheduler: NewConcurrentScheduler(),
		sequentialScheduler: NewSequentialScheduler(),
		executionModes:      make(map[string]ExecutionMode),
	}
	return hs
}

// WithConcurrent 设置指定任务类型使用并发执行模式
func (s *HybridScheduler) WithConcurrent(taskTypes ...string) *HybridScheduler {
	s.modesMutex.Lock()
	defer s.modesMutex.Unlock()
	for _, taskType := range taskTypes {
		s.executionModes[taskType] = ConcurrentMode
	}
	return s
}

// WithSequential 设置指定任务类型使用顺序执行模式
func (s *HybridScheduler) WithSequential(taskTypes ...string) *HybridScheduler {
	s.modesMutex.Lock()
	defer s.modesMutex.Unlock()
	for _, taskType := range taskTypes {
		s.executionModes[taskType] = Sequential
	}
	return s
}

// Start 启动混合调度器
func (s *HybridScheduler) Start(ctx context.Context) error {
	if err := s.concurrentScheduler.Start(ctx); err != nil {
		return err
	}
	return s.sequentialScheduler.Start(ctx)
}

// Stop 停止混合调度器
func (s *HybridScheduler) Stop() error {
	if err := s.concurrentScheduler.Stop(); err != nil {
		return err
	}
	return s.sequentialScheduler.Stop()
}

// Close 优雅关闭混合调度器并清理资源
func (s *HybridScheduler) Close(ctx context.Context) error {
	// 关闭并发调度器
	if err := s.concurrentScheduler.Close(ctx); err != nil {
		return err
	}

	// 关闭顺序调度器
	if err := s.sequentialScheduler.Close(ctx); err != nil {
		return err
	}

	return nil
}

// Schedule 调度一个任务
// 根据任务类型选择合适的调度器
func (s *HybridScheduler) Schedule(task Task) error {
	// 确定使用哪个调度器
	var scheduler TaskScheduler

	// 定时任务和周期任务默认使用顺序模式
	if task.Schedule().After(time.Now()) || task.IsRecurring() {
		scheduler = s.sequentialScheduler
	} else {
		// 根据任务类型选择执行模式
		mode := s.getExecutionMode(task.Type())
		if mode == Sequential {
			scheduler = s.sequentialScheduler
		} else {
			// 默认使用并发模式
			scheduler = s.concurrentScheduler
		}
	}

	// 调度任务
	return scheduler.Schedule(task)
}

// Cancel 取消指定的任务
// 需要在两个调度器中都尝试取消
func (s *HybridScheduler) Cancel(taskID string) error {
	// 先尝试在并发调度器中取消
	err1 := s.concurrentScheduler.Cancel(taskID)
	if err1 == nil {
		return nil
	}

	// 如果并发调度器中没有找到任务，尝试在顺序调度器中取消
	return s.sequentialScheduler.Cancel(taskID)
}

// GetTaskStatus 获取任务状态
// 需要在两个调度器中都尝试获取
func (s *HybridScheduler) GetTaskStatus(taskID string) (TaskStatus, error) {
	// 先尝试在并发调度器中获取
	status, err := s.concurrentScheduler.GetTaskStatus(taskID)
	if err == nil {
		return status, nil
	}

	// 如果并发调度器中没有找到任务，尝试在顺序调度器中获取
	return s.sequentialScheduler.GetTaskStatus(taskID)
}

// ListTasks 列出所有任务
// 合并两个调度器的任务列表
func (s *HybridScheduler) ListTasks() []TaskInfo {
	concurrentTasks := s.concurrentScheduler.ListTasks()
	sequentialTasks := s.sequentialScheduler.ListTasks()

	// 合并任务列表
	return append(concurrentTasks, sequentialTasks...)
}

// SetMaxConcurrentTasks 设置最大并发任务数
// 只对并发调度器有效
func (s *HybridScheduler) SetMaxConcurrentTasks(max int) error {
	return s.concurrentScheduler.SetMaxConcurrentTasks(max)
}

// getExecutionMode 获取指定任务类型的执行模式
func (s *HybridScheduler) getExecutionMode(taskType string) ExecutionMode {
	s.modesMutex.RLock()
	defer s.modesMutex.RUnlock()

	if mode, exists := s.executionModes[taskType]; exists {
		return mode
	}

	// 默认使用并发模式
	return ConcurrentMode
}
