package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/logger"
)

// TaskExecutionPolicy 任务执行策略
type TaskExecutionPolicy string

const (
	// PolicyConcurrent 并发执行策略
	PolicyConcurrent TaskExecutionPolicy = "concurrent"
	// PolicySequential 顺序执行策略  
	PolicySequential TaskExecutionPolicy = "sequential"
	// PolicyPipeline 流水线执行策略
	PolicyPipeline TaskExecutionPolicy = "pipeline"
)

// HybridTaskScheduler 混合任务调度器
// 根据任务类型自动选择合适的执行策略
type HybridTaskScheduler struct {
	logger *logger.Logger
	
	// 不同策略的调度器
	concurrentScheduler TaskScheduler
	sequentialScheduler TaskScheduler  
	pipelineScheduler   TaskScheduler
	
	// 任务类型到策略的映射
	policyMap map[string]TaskExecutionPolicy
	
	// 同步控制
	mu sync.RWMutex
	
	// 统计信息
	stats map[TaskExecutionPolicy]*ExecutionStats
}

// ExecutionStats 执行统计信息
type ExecutionStats struct {
	TotalTasks     int64     `json:"total_tasks"`
	CompletedTasks int64     `json:"completed_tasks"`
	FailedTasks    int64     `json:"failed_tasks"`
	RunningTasks   int64     `json:"running_tasks"`
	LastExecution  time.Time `json:"last_execution"`
}

// HybridSchedulerOption 混合调度器配置选项
type HybridSchedulerOption func(*HybridTaskSchedulerConfig)

// HybridTaskSchedulerConfig 混合调度器配置
type HybridTaskSchedulerConfig struct {
	ConcurrentWorkers int
	SequentialWorkers int
	PipelineWorkers   int
	MaxConcurrentTasks int
	Logger           *logger.Logger
}

// WithHybridLogger 设置日志记录器
func WithHybridLogger(logger *logger.Logger) HybridSchedulerOption {
	return func(c *HybridTaskSchedulerConfig) {
		c.Logger = logger
	}
}

// WithHybridWorkerCount 设置各策略的工作协程数
func WithHybridWorkerCount(concurrent, sequential, pipeline int) HybridSchedulerOption {
	return func(c *HybridTaskSchedulerConfig) {
		c.ConcurrentWorkers = concurrent
		c.SequentialWorkers = sequential
		c.PipelineWorkers = pipeline
	}
}

// WithHybridMaxConcurrentTasks 设置最大并发任务数
func WithHybridMaxConcurrentTasks(max int) HybridSchedulerOption {
	return func(c *HybridTaskSchedulerConfig) {
		c.MaxConcurrentTasks = max
	}
}

// NewHybridTaskScheduler 创建混合任务调度器
func NewHybridTaskScheduler(opts ...HybridSchedulerOption) *HybridTaskScheduler {
	config := &HybridTaskSchedulerConfig{
		ConcurrentWorkers:  5,
		SequentialWorkers:  0,  // 默认不启用顺序执行
		PipelineWorkers:    0,  // 默认不启用流水线执行
		MaxConcurrentTasks: 10,
	}
	
	for _, opt := range opts {
		opt(config)
	}
	
	scheduler := &HybridTaskScheduler{
		logger: config.Logger,
		policyMap: make(map[string]TaskExecutionPolicy),
		stats: map[TaskExecutionPolicy]*ExecutionStats{
			PolicyConcurrent: {LastExecution: time.Now()},
			PolicySequential: {LastExecution: time.Now()},
			PolicyPipeline:   {LastExecution: time.Now()},
		},
	}
	
	// 创建并发调度器
	scheduler.concurrentScheduler = NewMemoryTaskScheduler(
		WithLogger(config.Logger),
		WithWorkerCount(config.ConcurrentWorkers),
		WithMaxConcurrentTasks(config.MaxConcurrentTasks),
	)
	
	// 只有在配置了工作协程数时才创建顺序和流水线调度器
	if config.SequentialWorkers > 0 {
		scheduler.sequentialScheduler = NewMemoryTaskScheduler(
			WithLogger(config.Logger),
			WithWorkerCount(1),  // 顺序执行只需要1个工作协程
			WithMaxConcurrentTasks(1), // 顺序执行只能有一个并发
		)
		scheduler.sequentialScheduler.SetExecutionMode(ExecutionModeSequential)
	}
	
	if config.PipelineWorkers > 0 {
		scheduler.pipelineScheduler = NewMemoryTaskScheduler(
			WithLogger(config.Logger),
			WithWorkerCount(1),  // 流水线执行只需要1个工作协程
			WithMaxConcurrentTasks(1), // 流水线执行只能有一个并发
		)
		scheduler.pipelineScheduler.SetExecutionMode(ExecutionModePipeline)
	}
	
	return scheduler
}

// RegisterTaskPolicy 注册任务类型的执行策略
func (h *HybridTaskScheduler) RegisterTaskPolicy(taskType string, policy TaskExecutionPolicy) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if policy != PolicyConcurrent && policy != PolicySequential && policy != PolicyPipeline {
		return fmt.Errorf("无效的执行策略: %s", policy)
	}
	
	h.policyMap[taskType] = policy
	
	if h.logger != nil {
		h.logger.Infof("任务类型 %s 注册执行策略: %s", taskType, policy)
	}
	
	return nil
}

// Schedule 调度任务
func (h *HybridTaskScheduler) Schedule(task Task) error {
	policy := h.getTaskPolicy(task.Type())
	
	scheduler, err := h.getSchedulerByPolicy(policy)
	if err != nil {
		return err
	}
	
	// 更新统计信息
	h.updateStats(policy, true)
	
	// 包装任务以完成统计
	wrappedTask := h.wrapTaskWithStats(task, policy)
	
	return scheduler.Schedule(wrappedTask)
}

// Start 启动所有调度器
func (h *HybridTaskScheduler) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// 启动并发调度器（总是存在）
	if err := h.concurrentScheduler.Start(ctx); err != nil {
		return fmt.Errorf("启动并发调度器失败: %w", err)
	}
	
	// 启动顺序调度器（如果存在）
	if h.sequentialScheduler != nil {
		if err := h.sequentialScheduler.Start(ctx); err != nil {
			return fmt.Errorf("启动顺序调度器失败: %w", err)
		}
	}
	
	// 启动流水线调度器（如果存在）
	if h.pipelineScheduler != nil {
		if err := h.pipelineScheduler.Start(ctx); err != nil {
			return fmt.Errorf("启动流水线调度器失败: %w", err)
		}
	}
	
	if h.logger != nil {
		h.logger.Info("混合任务调度器已启动")
	}
	
	return nil
}

// Stop 停止所有调度器
func (h *HybridTaskScheduler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	var errors []error
	
	// 停止并发调度器（总是存在）
	if err := h.concurrentScheduler.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("停止并发调度器失败: %w", err))
	}
	
	// 停止顺序调度器（如果存在）
	if h.sequentialScheduler != nil {
		if err := h.sequentialScheduler.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("停止顺序调度器失败: %w", err))
		}
	}
	
	// 停止流水线调度器（如果存在）
	if h.pipelineScheduler != nil {
		if err := h.pipelineScheduler.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("停止流水线调度器失败: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("停止调度器时发生错误: %v", errors)
	}
	
	if h.logger != nil {
		h.logger.Info("混合任务调度器已停止")
	}
	
	return nil
}

// Close 优雅关闭所有调度器
func (h *HybridTaskScheduler) Close(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	var errors []error
	
	// 关闭并发调度器（总是存在）
	if err := h.concurrentScheduler.Close(ctx); err != nil {
		errors = append(errors, fmt.Errorf("关闭并发调度器失败: %w", err))
	}
	
	// 关闭顺序调度器（如果存在）
	if h.sequentialScheduler != nil {
		if err := h.sequentialScheduler.Close(ctx); err != nil {
			errors = append(errors, fmt.Errorf("关闭顺序调度器失败: %w", err))
		}
	}
	
	// 关闭流水线调度器（如果存在）
	if h.pipelineScheduler != nil {
		if err := h.pipelineScheduler.Close(ctx); err != nil {
			errors = append(errors, fmt.Errorf("关闭流水线调度器失败: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("关闭调度器时发生错误: %v", errors)
	}
	
	if h.logger != nil {
		h.logger.Info("混合任务调度器已优雅关闭")
	}
	
	return nil
}

// Cancel 取消任务
func (h *HybridTaskScheduler) Cancel(taskID string) error {
	// 尝试从所有调度器中取消任务
	var errors []error
	
	if err := h.concurrentScheduler.Cancel(taskID); err == nil {
		return nil
	} else {
		errors = append(errors, err)
	}
	
	if h.sequentialScheduler != nil {
		if err := h.sequentialScheduler.Cancel(taskID); err == nil {
			return nil
		} else {
			errors = append(errors, err)
		}
	}
	
	if h.pipelineScheduler != nil {
		if err := h.pipelineScheduler.Cancel(taskID); err == nil {
			return nil
		} else {
			errors = append(errors, err)
		}
	}
	
	return fmt.Errorf("任务未找到或取消失败: %v", errors)
}

// GetTaskStatus 获取任务状态
func (h *HybridTaskScheduler) GetTaskStatus(taskID string) (TaskStatus, error) {
	// 尝试从所有调度器中获取任务状态
	if status, err := h.concurrentScheduler.GetTaskStatus(taskID); err == nil {
		return status, nil
	}
	
	if h.sequentialScheduler != nil {
		if status, err := h.sequentialScheduler.GetTaskStatus(taskID); err == nil {
			return status, nil
		}
	}
	
	if h.pipelineScheduler != nil {
		if status, err := h.pipelineScheduler.GetTaskStatus(taskID); err == nil {
			return status, nil
		}
	}
	
	return "", fmt.Errorf("任务未找到: %s", taskID)
}

// ListTasks 列出所有任务
func (h *HybridTaskScheduler) ListTasks() []TaskInfo {
	var allTasks []TaskInfo
	
	allTasks = append(allTasks, h.concurrentScheduler.ListTasks()...)
	
	if h.sequentialScheduler != nil {
		allTasks = append(allTasks, h.sequentialScheduler.ListTasks()...)
	}
	
	if h.pipelineScheduler != nil {
		allTasks = append(allTasks, h.pipelineScheduler.ListTasks()...)
	}
	
	return allTasks
}

// SetMaxConcurrentTasks 设置最大并发任务数
func (h *HybridTaskScheduler) SetMaxConcurrentTasks(max int) error {
	// 设置并发调度器的最大并发数
	if err := h.concurrentScheduler.SetMaxConcurrentTasks(max); err != nil {
		return fmt.Errorf("设置并发调度器最大并发数失败: %w", err)
	}
	
	// 顺序和流水线调度器保持单线程，不需要设置
	return nil
}

// SetExecutionMode 设置执行模式（混合调度器不支持此方法）
func (h *HybridTaskScheduler) SetExecutionMode(mode ExecutionMode) error {
	return fmt.Errorf("混合调度器不支持设置单一执行模式，请使用RegisterTaskPolicy方法注册任务类型策略")
}

// GetExecutionMode 获取当前执行模式（混合调度器返回混合模式）
func (h *HybridTaskScheduler) GetExecutionMode() ExecutionMode {
	return ExecutionModeConcurrent // 混合调度器默认返回并发模式
}

// GetStats 获取执行统计信息
func (h *HybridTaskScheduler) GetStats() map[TaskExecutionPolicy]*ExecutionStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	// 返回统计信息的副本
	stats := make(map[TaskExecutionPolicy]*ExecutionStats)
	for policy, stat := range h.stats {
		stats[policy] = &ExecutionStats{
			TotalTasks:     stat.TotalTasks,
			CompletedTasks: stat.CompletedTasks,
			FailedTasks:    stat.FailedTasks,
			RunningTasks:   stat.RunningTasks,
			LastExecution:  stat.LastExecution,
		}
	}
	
	return stats
}

// getTaskPolicy 获取任务类型的执行策略
func (h *HybridTaskScheduler) getTaskPolicy(taskType string) TaskExecutionPolicy {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if policy, exists := h.policyMap[taskType]; exists {
		return policy
	}
	
	// 默认使用并发策略
	return PolicyConcurrent
}

// getSchedulerByPolicy 根据策略获取对应的调度器
func (h *HybridTaskScheduler) getSchedulerByPolicy(policy TaskExecutionPolicy) (TaskScheduler, error) {
	switch policy {
	case PolicyConcurrent:
		return h.concurrentScheduler, nil
	case PolicySequential:
		if h.sequentialScheduler == nil {
			return nil, fmt.Errorf("顺序调度器未启用")
		}
		return h.sequentialScheduler, nil
	case PolicyPipeline:
		if h.pipelineScheduler == nil {
			return nil, fmt.Errorf("流水线调度器未启用")
		}
		return h.pipelineScheduler, nil
	default:
		return nil, fmt.Errorf("不支持的执行策略: %s", policy)
	}
}

// wrapTaskWithStats 包装任务以收集统计信息
func (h *HybridTaskScheduler) wrapTaskWithStats(task Task, policy TaskExecutionPolicy) Task {
	originalHandler := task.Handler()
	originalOnComplete := task.OnComplete()
	
	return &statsTaskWrapper{
		task: task,
		handler: func(ctx context.Context) error {
			err := originalHandler(ctx)
			
			// 更新统计信息
			h.mu.Lock()
			stats := h.stats[policy]
			if err != nil {
				stats.FailedTasks++
			} else {
				stats.CompletedTasks++
			}
			stats.LastExecution = time.Now()
			h.mu.Unlock()
			
			return err
		},
		onComplete: func(ctx context.Context, err error) {
			if originalOnComplete != nil {
				originalOnComplete(ctx, err)
			}
		},
	}
}

// statsTaskWrapper 用于统计的任务包装器
type statsTaskWrapper struct {
	task       Task
	handler    func(ctx context.Context) error
	onComplete func(ctx context.Context, err error)
}

func (t *statsTaskWrapper) ID() string {
	return t.task.ID()
}

func (t *statsTaskWrapper) Type() string {
	return t.task.Type()
}

func (t *statsTaskWrapper) Schedule() time.Time {
	return t.task.Schedule()
}

func (t *statsTaskWrapper) Handler() func(ctx context.Context) error {
	return t.handler
}

func (t *statsTaskWrapper) OnComplete() func(ctx context.Context, err error) {
	return t.onComplete
}

func (t *statsTaskWrapper) OnCancel() func(ctx context.Context) {
	return t.task.OnCancel()
}

func (t *statsTaskWrapper) IsRecurring() bool {
	return t.task.IsRecurring()
}

func (t *statsTaskWrapper) GetInterval() time.Duration {
	return t.task.GetInterval()
}

// updateStats 更新统计信息
func (h *HybridTaskScheduler) updateStats(policy TaskExecutionPolicy, isNewTask bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	stats := h.stats[policy]
	if isNewTask {
		stats.TotalTasks++
	}
}