# 任务调度器设计

## 设计概述

本文档详细设计了一个灵活、高效的任务调度器，支持即时执行、定时执行和周期性执行等多种调度方式，并提供了任务状态管理、错误处理、重试机制和并行度控制等功能。设计遵循了依赖倒置原则和接口隔离原则，使系统具有良好的扩展性和可维护性。

## 1. 接口设计

### 设计思路

接口设计是任务调度器的核心，采用了面向接口编程的思想，将调度器和任务的行为抽象为接口，实现了高内聚低耦合的设计目标。通过分离接口和实现，使得系统可以轻松替换不同的调度器实现而不影响客户端代码。

### 1.1 任务调度器接口

```go
// GracefulClose 定义优雅关闭接口
type GracefulClose interface {
	// Close 优雅关闭资源
	Close() error
}

type TaskScheduler interface {
	// GracefulClose 优雅关闭任务调度器
	GracefulClose
	// Schedule 调度一个任务
	Schedule(task Task) error
	// Start 启动任务调度器
	Start() error
	// Stop 停止任务调度器
	Stop() error
	// Cancel 取消指定的任务
	Cancel(taskID string) error
	// GetTaskStatus 获取任务状态
	GetTaskStatus(taskID string) (TaskStatus, error)
	// ListTasks 列出所有任务
	ListTasks() []TaskInfo
	// SetMaxConcurrentTasks 设置最大并发任务数
	SetMaxConcurrentTasks(max int) error
}

// TaskInfo 任务信息
type TaskInfo struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Status    TaskStatus `json:"status"`
	Schedule  time.Time  `json:"schedule"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
```

**设计理由**：
- 继承`GracefulClose`接口实现资源的优雅释放
- `Schedule`方法用于添加任务到调度器
- `Start`和`Stop`方法控制调度器的生命周期
- `Cancel`方法支持任务的取消操作
- `GetTaskStatus`和`ListTasks`方法提供任务状态查询能力
- `SetMaxConcurrentTasks`方法支持动态调整并发执行的任务数量
- `TaskInfo`结构体用于返回任务的元数据信息，方便监控和管理

**功能作用**：
- 定义任务调度器的核心功能集
- 提供任务的全生命周期管理
- 支持任务状态查询和管理
- 允许动态控制并发执行的任务数量

**使用示例**：
```go
// 创建并使用任务调度器
var scheduler TaskScheduler = NewMemoryTaskScheduler(logger)
if err := scheduler.Start(); err != nil {
	log.Fatal(err)
}

task := NewTaskBuilder().Immediate().Build()
if err := scheduler.Schedule(task); err != nil {
	log.Fatal(err)
}

// 查询任务状态
status, err := scheduler.GetTaskStatus(task.ID())

// 动态调整并发数
scheduler.SetMaxConcurrentTasks(20)

// 优雅关闭
scheduler.Close()
```

### 1.2 任务接口

```go
type Task interface {
	// ID 返回任务的唯一标识符
	ID() string
	// Type 返回任务的类型
	Type() string
	// Schedule 返回任务的执行时间
	Schedule() time.Time
	// Handler 返回任务的处理方法
	Handler() func(ctx context.Context) error
	// OnComplete 返回任务完成后的回调方法
	OnComplete() func(ctx context.Context, err error)
	// OnCancel 返回任务取消后的回调方法
	OnCancel() func(ctx context.Context)
	// IsRecurring 判断任务是否为周期性任务
	IsRecurring() bool
	// GetInterval 获取任务的执行间隔
	GetInterval() time.Duration
}
```

**设计理由**：
- 提供`ID`方法返回唯一标识符，用于任务追踪和管理
- `Type`方法用于区分不同类型的任务
- `Schedule`方法确定任务的执行时间
- `Handler`方法定义任务的核心执行逻辑，并支持上下文控制
- `OnComplete`和`OnCancel`回调方法实现任务生命周期事件的处理
- `IsRecurring`和`GetInterval`方法支持周期性任务的实现

**功能作用**：
- 定义任务的核心属性和行为
- 支持任务的执行、回调和状态管理
- 提供周期性任务的识别和间隔控制
- 允许通过上下文控制任务执行

**使用示例**：
```go
// 实现Task接口的自定义任务
type MyTask struct {
	id          string
	taskType    string
	execTime    time.Time
	handlerFunc func(ctx context.Context) error
}

func (t *MyTask) ID() string { return t.id }
func (t *MyTask) Type() string { return t.taskType }
func (t *MyTask) Schedule() time.Time { return t.execTime }
func (t *MyTask) Handler() func(ctx context.Context) error { return t.handlerFunc }
func (t *MyTask) OnComplete() func(ctx context.Context, error) { return nil }
func (t *MyTask) OnCancel() func(ctx context.Context) { return nil }
func (t *MyTask) IsRecurring() bool { return false }
func (t *MyTask) GetInterval() time.Duration { return 0 }
```

### 1.3 任务状态定义

```go
type TaskStatus string

const (
	TaskStatusWaiting   TaskStatus = "waiting"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusFailed    TaskStatus = "failed"
)
```

**设计理由**：
- 使用枚举类型明确定义任务的五种状态
- `Waiting`状态标识任务已添加但尚未执行
- `Running`状态标识任务正在执行中
- `Completed`状态标识任务成功执行完毕
- `Cancelled`状态标识任务被主动取消
- `Failed`状态标识任务执行失败

**功能作用**：
- 提供清晰的任务生命周期状态定义
- 支持任务执行过程的状态追踪
- 便于监控和管理任务执行情况
- 为任务处理提供状态决策依据

**使用示例**：
```go
// 根据任务状态执行不同操作
status, _ := scheduler.GetTaskStatus(taskID)
switch status {
case TaskStatusWaiting:
	fmt.Println("任务等待执行")
case TaskStatusRunning:
	fmt.Println("任务正在执行")
case TaskStatusCompleted:
	fmt.Println("任务执行成功")
case TaskStatusFailed:
	fmt.Println("任务执行失败")
case TaskStatusCancelled:
	fmt.Println("任务已取消")
}
```

## 2. 任务执行时间定义

### 设计思路

任务执行时间是调度器的关键属性，设计了三种调度类型（即时执行、一次性执行和周期性执行），通过统一的接口和结构体来表示不同类型的执行时间，使得调度器可以一致地处理各种任务。

### 实现方案

```go
// TaskScheduleType 任务调度类型
type TaskScheduleType int

const (
	// ScheduleTypeImmediate 即时执行
	ScheduleTypeImmediate TaskScheduleType = iota
	// ScheduleTypeOnce 一次性执行
	ScheduleTypeOnce
	// ScheduleTypeRecurring 周期性执行
	ScheduleTypeRecurring
)

// TaskSchedule 任务调度配置
type TaskSchedule struct {
	Type     TaskScheduleType
	At       time.Time // 一次性执行的时间点
	Interval time.Duration // 周期性执行的间隔
}

// 任务创建时设置执行时间的辅助函数
func ImmediateExecution() TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeImmediate}
}

func ScheduleAt(at time.Time) TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeOnce, At: at}
}

func ScheduleRecurring(interval time.Duration) TaskSchedule {
	return TaskSchedule{Type: ScheduleTypeRecurring, Interval: interval}
}
```

**设计理由**：
- 使用枚举类型清晰区分不同的调度类型
- 通过结构体封装调度相关参数，便于扩展
- 提供辅助函数简化任务调度时间的设置
- 支持三种常见的调度场景需求

**功能作用**：
- 统一表示不同类型的任务执行时间
- 简化任务创建时的调度配置
- 支持即时、定时和周期性三种执行模式
- 为任务构建器提供灵活的时间配置接口

**使用示例**：
```go
// 创建不同类型的调度配置
immediate := ImmediateExecution()              // 立即执行
once := ScheduleAt(time.Now().Add(2 * time.Hour)) // 2小时后执行一次
recurring := ScheduleRecurring(15 * time.Minute) // 每15分钟执行一次

// 在任务构建中使用
task1 := NewTaskBuilder().Immediate().Build()
task2 := NewTaskBuilder().ScheduleAt(time.Now().Add(10 * time.Second)).Build()
task3 := NewTaskBuilder().ScheduleRecurring(5 * time.Second).Build()
```

## 3. 任务调度器实现

### 设计思路

采用基于内存的优先队列实现任务调度，支持按时间顺序执行任务，并通过工作池控制并发度。设计考虑了线程安全、错误处理、任务重试和优雅关闭等关键因素，确保系统在各种场景下的稳定性和可靠性。

### 3.1 内存实现

```go
// MemoryTaskScheduler 基于内存的任务调度器实现
type MemoryTaskScheduler struct {
	tasks        map[string]*taskWrapper // 存储所有任务
	taskQueue    *priorityQueue          // 优先队列，按执行时间排序
	mu           sync.RWMutex            // 互斥锁，保证并发安全
	stopChan     chan struct{}           // 停止信号通道
	wg           sync.WaitGroup          // 等待组，用于优雅关闭
	started      bool                    // 调度器状态标志
	workerCount  int                     // 工作协程数量
	logger       *logger.Logger          // 日志器
	workerPool   chan struct{}           // 工作池，控制并发执行
	maxConcurrentTasks int               // 最大并发任务数
}

type taskWrapper struct {
	task       Task                 // 原始任务
	status     TaskStatus           // 任务状态
	cancelFunc context.CancelFunc   // 取消函数
	createdAt  time.Time            // 创建时间
	updatedAt  time.Time            // 更新时间
}

type priorityQueue struct {
	items []*taskWrapper
}
```

**设计理由**：
- 使用`map`存储所有任务，提供快速查找能力
- 采用优先队列实现按时间顺序调度任务
- 通过互斥锁保证并发安全
- 工作池机制控制并发执行的任务数量
- 支持优雅关闭，确保资源正确释放
- 提供状态追踪和错误处理机制

**功能作用**：
- 实现任务的存储和管理
- 按时间顺序调度和执行任务
- 控制任务的并发执行数量
- 提供任务状态追踪和管理
- 支持优雅关闭和资源释放

### 3.2 核心执行逻辑

```go
// 任务调度核心逻辑
func (s *MemoryTaskScheduler) run() {
	for {
		select {
		case <-s.stopChan:
			return // 收到停止信号，退出循环
		default:
			// 检查是否有任务需要执行
			s.mu.Lock()
			if s.taskQueue.Len() == 0 {
				s.mu.Unlock()
				time.Sleep(100 * time.Millisecond) // 避免CPU空转
				continue
			}

			nextTask := s.taskQueue.Peek()
			if nextTask == nil || time.Until(nextTask.task.Schedule()) > 0 {
				s.mu.Unlock()
				time.Sleep(100 * time.Millisecond) // 未到执行时间，等待
				continue
			}

			// 取出任务
			task := s.taskQueue.Pop()
			task.status = TaskStatusRunning // 更新任务状态
			task.updatedAt = time.Now()
			s.mu.Unlock()

			// 执行任务
			s.executeTask(task)
		}
	}
}

// 任务执行函数
func (s *MemoryTaskScheduler) executeTask(t *taskWrapper) {
	// 获取工作池中的令牌，限制并发数
	s.workerPool <- struct{}{}
	s.wg.Add(1)
	
	go func(task *taskWrapper) {
		defer func() {
			<-s.workerPool // 释放令牌
			s.wg.Done()    // 标记任务完成
			
			// 启用panic恢复，提高系统稳定性
			if r := recover(); r != nil {
				s.logger.Error("任务执行发生panic", "task_id", task.task.ID(), "error", r)
				s.updateTaskStatus(task, TaskStatusFailed)
				
				// 触发完成回调
				if onComplete := task.task.OnComplete(); onComplete != nil {
					ctx := context.Background()
					onComplete(ctx, fmt.Errorf("task panic: %v", r))
				}
			}
		}()

		// 创建可取消的上下文
		ctx, cancel := context.WithCancel(context.Background())
		task.cancelFunc = cancel
		
		// 执行任务处理器
		err := task.task.Handler()(ctx)
		
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
			nextSchedule := time.Now().Add(task.task.GetInterval())
			// 重新创建任务并调度...
		}
	}(t)
}
```

**设计理由**：
- 采用无限循环监听任务执行需求
- 使用优先队列确保任务按时间顺序执行
- 工作池控制并发执行的任务数量
- panic恢复机制提高系统稳定性
- 上下文控制支持任务取消
- 状态更新和回调机制完善任务生命周期管理

**功能作用**：
- 实现按时间顺序调度任务的核心逻辑
- 支持任务的并发执行，并控制并发数量
- 提供panic恢复机制，增强系统稳定性
- 支持任务状态更新和回调触发
- 处理周期性任务的重新调度

## 4. 任务构建器

### 设计思路

采用构建器模式创建任务，提供流式API，简化任务的创建和配置过程。构建器模式允许我们灵活地配置任务的各个属性，同时保持代码的可读性和可维护性。

### 实现方案

```go
// TaskBuilder 任务构建器
type TaskBuilder struct {
	id          string
	typeName    string
	schedule    TaskSchedule
	handler     func(ctx context.Context) error
	onComplete  func(ctx context.Context, err error)
	onCancel    func(ctx context.Context)
}

// NewTaskBuilder 创建新的任务构建器
func NewTaskBuilder() *TaskBuilder {
	return &TaskBuilder{
		id:       utils.GenerateUUID(), // 使用项目中的ID生成工具
		typeName: "default",
	}
}

// WithID 设置任务ID
func (b *TaskBuilder) WithID(id string) *TaskBuilder {
	b.id = id
	return b
}

// WithType 设置任务类型
func (b *TaskBuilder) WithType(taskType string) *TaskBuilder {
	b.typeName = taskType
	return b
}

// Immediate 设置为即时执行
func (b *TaskBuilder) Immediate() *TaskBuilder {
	b.schedule = ImmediateExecution()
	return b
}

// ScheduleAt 设置为在指定时间执行
func (b *TaskBuilder) ScheduleAt(at time.Time) *TaskBuilder {
	b.schedule = ScheduleAt(at)
	return b
}

// ScheduleRecurring 设置为周期性执行
func (b *TaskBuilder) ScheduleRecurring(interval time.Duration) *TaskBuilder {
	b.schedule = ScheduleRecurring(interval)
	return b
}

// WithHandler 设置任务处理函数
func (b *TaskBuilder) WithHandler(handler func(ctx context.Context) error) *TaskBuilder {
	b.handler = handler
	return b
}

// WithOnComplete 设置任务完成回调
func (b *TaskBuilder) WithOnComplete(callback func(ctx context.Context, err error)) *TaskBuilder {
	b.onComplete = callback
	return b
}

// WithOnCancel 设置任务取消回调
func (b *TaskBuilder) WithOnCancel(callback func(ctx context.Context)) *TaskBuilder {
	b.onCancel = callback
	return b
}

// Build 构建任务
func (b *TaskBuilder) Build() Task {
	if b.handler == nil {
		panic("任务处理函数不能为空") // 强制检查必要参数
	}
	
	// 计算执行时间
	execTime := time.Time{}
	switch b.schedule.Type {
	case ScheduleTypeImmediate:
		execTime = time.Now()
	case ScheduleTypeOnce:
		execTime = b.schedule.At
	case ScheduleTypeRecurring:
		execTime = time.Now().Add(b.schedule.Interval)
	}
	
	return &builtTask{
		id:          b.id,
		typeName:    b.typeName,
		execTime:    execTime,
		handler:     b.handler,
		onComplete:  b.onComplete,
		onCancel:    b.onCancel,
		schedule:    b.schedule,
	}
}

// 内部任务实现
type builtTask struct {
	id          string
	typeName    string
	execTime    time.Time
	handler     func(ctx context.Context) error
	onComplete  func(ctx context.Context, err error)
	onCancel    func(ctx context.Context)
	schedule    TaskSchedule
}

// 实现Task接口的方法
func (t *builtTask) ID() string {
	return t.id
}

func (t *builtTask) Type() string {
	return t.typeName
}

func (t *builtTask) Schedule() time.Time {
	return t.execTime
}

func (t *builtTask) Handler() func(ctx context.Context) error {
	return t.handler
}

func (t *builtTask) OnComplete() func(ctx context.Context, err error) {
	return t.onComplete
}

func (t *builtTask) OnCancel() func(ctx context.Context) {
	return t.onCancel
}

func (t *builtTask) IsRecurring() bool {
	return t.schedule.Type == ScheduleTypeRecurring
}

func (t *builtTask) GetInterval() time.Duration {
	return t.schedule.Interval
}
```

**设计理由**：
- 提供流式API，简化任务创建过程
- 支持灵活配置任务的各个属性
- 自动生成唯一ID，简化客户端代码
- 强制检查必要参数，确保任务有效性
- 封装任务实现细节，对外提供统一接口

**功能作用**：
- 简化任务的创建和配置
- 提供类型安全的任务构建过程
- 支持三种不同的调度类型
- 允许配置任务处理器和回调函数
- 自动生成和管理任务ID

**使用示例**：
```go
// 创建即时执行的任务
immediateTask := NewTaskBuilder().
	WithType("cleanup").
	Immediate().
	WithHandler(func(ctx context.Context) error {
		fmt.Println("执行即时任务")
		return nil
	}).
	Build()

// 创建定时任务	timedTask := NewTaskBuilder().
	WithID("notification-123").
	WithType("notification").
	ScheduleAt(time.Now().Add(1 * time.Hour)).
	WithHandler(sendNotification).
	WithOnComplete(handleNotificationComplete).
	Build()

// 创建周期性任务
recurringTask := NewTaskBuilder().
	WithType("health-check").
	ScheduleRecurring(5 * time.Minute).
	WithHandler(checkSystemHealth).
	WithOnCancel(cleanupHealthCheck).
	Build()
```

## 5. 使用示例

### 基本使用

```go
// 创建任务调度器
logger := logger.NewLogger()
scheduler := NewMemoryTaskScheduler(logger, 
	WithWorkerCount(5),
	WithMaxConcurrentTasks(10),
)

// 启动调度器
if err := scheduler.Start(); err != nil {
	panic(err)
}

// 创建并调度即时执行的任务
immediateTask := NewTaskBuilder().
	WithType("cleanup").
	Immediate().
	WithHandler(func(ctx context.Context) error {
		fmt.Println("执行即时任务")
		return nil
	}).
	WithOnComplete(func(ctx context.Context, err error) {
		if err != nil {
			fmt.Printf("任务执行失败: %v\n", err)
		} else {
			fmt.Println("任务执行成功")
		}
	}).
	Build()

if err := scheduler.Schedule(immediateTask); err != nil {
	panic(err)
}

// 创建并调度定时任务 (10秒后执行)
timedTask := NewTaskBuilder().
	WithType("notification").
	ScheduleAt(time.Now().Add(10 * time.Second)).
	WithHandler(func(ctx context.Context) error {
		fmt.Println("执行定时任务")
		return nil
	}).
	Build()

if err := scheduler.Schedule(timedTask); err != nil {
	panic(err)
}

// 创建并调度周期性任务 (每5秒执行一次)
recurringTask := NewTaskBuilder().
	WithType("monitor").
	ScheduleRecurring(5 * time.Second).
	WithHandler(func(ctx context.Context) error {
		fmt.Println("执行周期性任务")
		return nil
	}).
	Build()

if err := scheduler.Schedule(recurringTask); err != nil {
	panic(err)
}

// 取消任务
time.Sleep(2 * time.Second)
if err := scheduler.Cancel(recurringTask.ID()); err != nil {
	fmt.Printf("取消任务失败: %v\n", err)
}

// 查询任务状态
status, err := scheduler.GetTaskStatus(immediateTask.ID())
if err != nil {
	fmt.Printf("查询任务状态失败: %v\n", err)
} else {
	fmt.Printf("任务状态: %s\n", status)
}

// 列出所有任务
tasks := scheduler.ListTasks()
fmt.Printf("当前任务数: %d\n", len(tasks))

// 动态调整最大并发任务数
if err := scheduler.SetMaxConcurrentTasks(20); err != nil {
	fmt.Printf("调整并发数失败: %v\n", err)
}

// 优雅关闭	time.Sleep(15 * time.Second)
if err := scheduler.Close(); err != nil {
	panic(err)
}
```

## 6. 高级功能

### 6.1 任务重试机制

**设计思路**：
任务执行可能因为网络问题、资源限制等原因失败，实现重试机制可以提高任务的成功率。采用指数退避策略，避免在系统负载高峰期持续尝试失败的任务。

**实现方案**：

```go
// WithRetry 添加重试机制
func (b *TaskBuilder) WithRetry(maxRetries int, backoff time.Duration) *TaskBuilder {
	originalHandler := b.handler
	b.handler = func(ctx context.Context) error {
		var lastErr error
		for i := 0; i <= maxRetries; i++ {
			if i > 0 {
				// 计算退避时间（指数增长）
				sleepDuration := time.Duration(math.Pow(2, float64(i-1))) * backoff
				select {
				case <-time.After(sleepDuration):
				case <-ctx.Done():
					return ctx.Err() // 任务被取消或超时
				}
			}
			
			// 执行原始处理函数
			err := originalHandler(ctx)
			if err == nil {
				return nil // 成功执行，直接返回
			}
			lastErr = err // 记录最后一次错误
		}
		// 重试次数用尽，返回包含重试信息的错误
		return fmt.Errorf("任务执行失败，已重试%d次: %w", maxRetries, lastErr)
	}
	return b
}
```

**设计理由**：
- 拦截原始处理器，添加重试逻辑
- 采用指数退避算法，避免立即重试导致的系统压力
- 支持上下文取消，确保任务可以及时终止
- 保留原始错误信息，便于调试
- 通过包装错误提供重试次数信息

**功能作用**：
- 提供自动重试机制，提高任务成功率
- 采用指数退避策略，避免系统压力过大
- 支持上下文取消，确保任务可以及时终止
- 保留原始错误信息，便于调试和问题诊断

**使用示例**：
```go
// 创建具有重试机制的任务
networkTask := NewTaskBuilder().
	WithType("api-call").
	Immediate().
	WithHandler(func(ctx context.Context) error {
		// 可能失败的网络操作
		return http.Get("https://api.example.com/data")
	}).
	WithRetry(3, 1*time.Second) // 最多重试3次，初始退避1秒
	.Build()

// 添加重试任务到调度器
scheduler.Schedule(networkTask)
```

### 6.2 任务超时设置

**设计思路**：
长时间运行的任务可能占用系统资源，实现超时机制可以防止任务无限期执行。通过上下文控制和通道通信，确保任务能够在指定时间内完成或被强制终止。

**实现方案**：

```go
// WithTimeout 添加超时设置
func (b *TaskBuilder) WithTimeout(timeout time.Duration) *TaskBuilder {
	originalHandler := b.handler
	b.handler = func(ctx context.Context) error {
		// 创建带超时的上下文
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		
		// 使用通道接收任务执行结果
		resultChan := make(chan error, 1)
		go func() {
			resultChan <- originalHandler(timeoutCtx)
			close(resultChan)
		}()
		
		// 等待任务完成或超时
		select {
		case result := <-resultChan:
			return result // 返回任务执行结果
		case <-timeoutCtx.Done():
			return fmt.Errorf("任务执行超时: %w", timeoutCtx.Err()) // 返回超时错误
		}
	}
	return b
}
```

**设计理由**：
- 使用带超时的上下文控制任务执行时间
- 通过通道异步执行任务，避免阻塞
- 处理任务完成和超时两种情况
- 包装原始超时错误，提供更详细的错误信息
- 支持外层上下文取消，保持一致性

**功能作用**：
- 防止任务无限期执行，避免资源泄露
- 提供明确的超时错误信息
- 兼容外部上下文取消
- 确保任务在超时后能够被正确终止

**使用示例**：
```go
// 创建具有超时设置的任务
longRunningTask := NewTaskBuilder().
	WithType("data-processing").
	Immediate().
	WithHandler(func(ctx context.Context) error {
		// 耗时的数据处理操作
		return processLargeDataset(ctx)
	}).
	WithTimeout(30 * time.Second) // 30秒超时
	.Build()

// 组合使用重试和超时
reliableTask := NewTaskBuilder().
	WithType("reliable-operation").
	Immediate().
	WithHandler(performOperation).
	WithTimeout(10 * time.Second).
	WithRetry(2, 500*time.Millisecond).
	Build()
```

### 6.3 与依赖注入容器集成

**设计思路**：
现代应用程序通常使用依赖注入容器管理组件生命周期和依赖关系，提供与容器的集成方案可以简化任务调度器的配置和使用。

**实现方案**：

```go
// 提供一个工厂函数，用于从容器中创建任务调度器
func NewTaskSchedulerFromContainer(container *bootstrap.Container) TaskScheduler {
	// 从容器中获取配置和日志器
	cfg := container.Get("config").(*config.Config)
	logger := container.Get("logger").(*logger.Logger)
	
	// 尝试从容器中获取任务存储（如果有）
	var taskStore TaskStore
	storeObj, exists := container.Get("taskStore")
	if exists {
		if store, ok := storeObj.(TaskStore); ok {
			taskStore = store
		}
	}
	
	// 创建并配置任务调度器
	options := []SchedulerOption{
		WithWorkerCount(cfg.TaskScheduler.WorkerCount),
		WithMaxConcurrentTasks(cfg.TaskScheduler.MaxConcurrentTasks),
	}
	
	// 如果有任务存储，添加到配置选项中
	if taskStore != nil {
		options = append(options, WithTaskStore(taskStore))
	}
	
	scheduler := NewMemoryTaskScheduler(logger, options...)
	return scheduler
}
```

**设计理由**：
- 从依赖注入容器中获取所需组件
- 支持可选依赖的灵活处理
- 基于配置动态设置调度器参数
- 提供统一的工厂方法，简化创建过程

**功能作用**：
- 支持从依赖注入容器中获取配置和依赖
- 提供灵活的配置方式
- 简化任务调度器的创建和初始化
- 支持可选依赖（如任务存储）

**使用示例**：
```go
// 在应用启动时注册和初始化任务调度器
func setupContainer(container *bootstrap.Container) {
	// 注册任务调度器
	container.Register("taskScheduler", NewTaskSchedulerFromContainer)
	
	// 如果需要持久化，注册任务存储
	container.Register("taskStore", NewDatabaseTaskStore)
}

// 在应用中使用任务调度器
func useScheduler(container *bootstrap.Container) {
	scheduler := container.Get("taskScheduler").(TaskScheduler)
	if err := scheduler.Start(); err != nil {
		// 处理错误
	}
	
	// 调度任务...
}
```

## 7. 存储持久化

### 设计思路

为了支持应用重启后恢复任务，以及在分布式环境中共享任务，设计了任务存储接口。通过接口抽象存储操作，可以支持多种存储后端，如文件系统、数据库等。

### 实现方案

```go
// TaskStore 任务存储接口
type TaskStore interface {
	// Save 保存任务
	Save(task Task, status TaskStatus) error
	// Get 获取任务
	Get(taskID string) (Task, TaskStatus, error)
	// Delete 删除任务
	Delete(taskID string) error
	// List 获取所有任务
	List() ([]TaskInfo, error)
	// UpdateStatus 更新任务状态
	UpdateStatus(taskID string, status TaskStatus) error
	// GetPendingTasks 获取所有待执行的任务
	GetPendingTasks() ([]Task, error)
}

// 内存存储实现示例
type MemoryTaskStore struct {
	tasks map[string]*StoredTask
	mu    sync.RWMutex
}

type StoredTask struct {
	Task   Task
	Status TaskStatus
	Info   TaskInfo
}

// 数据库存储实现示例
type DatabaseTaskStore struct {
	db *sql.DB
}
```

**设计理由**：
- 抽象存储操作，支持多种存储后端
- 提供完整的CRUD操作
- 支持任务状态更新
- 提供查询待执行任务的能力
- 便于扩展到分布式环境

**功能作用**：
- 支持任务的持久化存储
- 提供任务的加载和恢复能力
- 支持任务状态的持久化更新
- 允许查询和管理存储的任务

**使用示例**：
```go
// 创建任务存储
store := NewDatabaseTaskStore(db)

// 创建带持久化的调度器	scheduler := NewMemoryTaskScheduler(logger, 
	WithTaskStore(store),
)

// 启动时加载任务	scheduler.Start() // 内部会从存储加载未完成的任务

// 任务会自动保存到存储中
```

## 8. 配置选项

### 设计思路

采用函数选项模式（Functional Options Pattern）实现灵活的配置，允许客户端只配置关心的选项，并提供合理的默认值。这种方式比构造函数参数或配置结构体更加灵活，且支持未来扩展。

### 实现方案

```go
// SchedulerOption 任务调度器配置选项
type SchedulerOption func(*MemoryTaskScheduler)

// WithWorkerCount 设置工作协程数量
func WithWorkerCount(count int) SchedulerOption {
	return func(s *MemoryTaskScheduler) {
		s.workerCount = count
	}
}

// WithTaskStore 设置任务存储
func WithTaskStore(store TaskStore) SchedulerOption {
	return func(s *MemoryTaskScheduler) {
		s.taskStore = store
	}
}

// WithRecovery 设置是否启用panic恢复
func WithRecovery(enabled bool) SchedulerOption {
	return func(s *MemoryTaskScheduler) {
		s.recoveryEnabled = enabled
	}
}

// WithMaxConcurrentTasks 设置最大并发任务数
func WithMaxConcurrentTasks(max int) SchedulerOption {
	return func(s *MemoryTaskScheduler) {
		s.maxConcurrentTasks = max
		if s.workerPool == nil || cap(s.workerPool) != max {
			s.workerPool = make(chan struct{}, max)
		}
	}
}

// NewMemoryTaskScheduler 创建新的内存任务调度器
func NewMemoryTaskScheduler(logger *logger.Logger, opts ...SchedulerOption) TaskScheduler {
	// 设置默认值
	s := &MemoryTaskScheduler{
		tasks:               make(map[string]*taskWrapper),
		taskQueue:           newPriorityQueue(),
		stopChan:            make(chan struct{}),
		workerCount:         1,
		maxConcurrentTasks:  10, // 默认最大并发任务数
		logger:              logger,
		recoveryEnabled:     true,
		taskStore:           nil,
	}
	
	// 初始化工作池，默认使用maxConcurrentTasks
	s.workerPool = make(chan struct{}, s.maxConcurrentTasks)
	
	// 应用配置选项
	for _, opt := range opts {
		opt(s)
	}
	
	return s
}
```

**设计理由**：
- 提供灵活的配置方式
- 支持部分配置，其余使用默认值
- 便于扩展新的配置选项
- 提高代码可读性
- 支持不同场景的定制需求

**功能作用**：
- 允许灵活配置任务调度器的各种参数
- 提供合理的默认值，简化常见使用场景
- 支持链式配置，提高代码可读性
- 便于未来功能扩展，不破坏兼容性

**使用示例**：
```go
// 创建带多种配置的调度器	scheduler := NewMemoryTaskScheduler(logger, 
	WithWorkerCount(5),              // 5个工作协程
	WithMaxConcurrentTasks(20),      // 最多20个并发任务
	WithTaskStore(databaseStore),    // 使用数据库存储
	WithRecovery(true),              // 启用panic恢复
)

// 创建最小配置的调度器	simpleScheduler := NewMemoryTaskScheduler(logger) // 使用所有默认值
```

## 9. 并行速度控制

### 设计思路

系统资源是有限的，控制任务的并发执行数量可以防止系统过载。设计了动态调整并发数的机制，允许根据系统负载实时调整执行策略，确保系统稳定性。

### 9.1 最大并发任务数设置

**实现方案**：

```go
// SetMaxConcurrentTasks 实现TaskScheduler接口的方法
func (s *MemoryTaskScheduler) SetMaxConcurrentTasks(max int) error {
	if max <= 0 {
		return fmt.Errorf("最大并发任务数必须大于0")
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 如果当前已经启动且正在运行任务，需要等待现有任务完成后再调整工作池大小
	if s.started && len(s.workerPool) > 0 {
		// 记录调整请求
		s.logger.Info("等待现有任务完成后调整最大并发任务数", "old_max", s.maxConcurrentTasks, "new_max", max)
		
		// 启动一个goroutine等待任务完成后调整
		go func(newMax int) {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ticker.C:
					s.mu.Lock()
					if len(s.workerPool) == 0 {
						s.maxConcurrentTasks = newMax
						s.workerPool = make(chan struct{}, newMax)
						s.logger.Info("成功调整最大并发任务数", "new_max", newMax)
						s.mu.Unlock()
						return
					}
					s.mu.Unlock()
				case <-s.stopChan:
					return
				}
			}
		}(max)
		
		return nil
	}
	
	// 直接调整工作池大小
	s.maxConcurrentTasks = max
	s.workerPool = make(chan struct{}, max)
	s.logger.Info("成功调整最大并发任务数", "new_max", max)
	
	return nil
}
```

**设计理由**：
- 实现动态调整并发数，支持运行时配置变更
- 提供参数验证，确保配置有效性
- 采用延迟调整策略，避免影响正在执行的任务
- 提供日志记录，便于问题排查和监控
- 支持优雅降级，确保系统稳定性

**功能作用**：
- 允许动态调整并发执行的任务数量
- 根据系统负载灵活配置执行策略
- 避免系统资源被过度占用
- 支持运行时调整，无需重启调度器

**使用示例**：
```go
// 初始创建时设置并发数	scheduler := NewMemoryTaskScheduler(logger, 
	WithMaxConcurrentTasks(10),
)

// 运行时动态调整	scheduler.Start()
// ...
if systemLoadHigh() {
	// 系统负载高，降低并发
	scheduler.SetMaxConcurrentTasks(5)
} else if systemLoadLow() {
	// 系统负载低，提高并发
	scheduler.SetMaxConcurrentTasks(30)
}
```

### 9.2 动态调整策略

**设计思路**：
根据系统负载动态调整并发执行的任务数量，实现资源的最优利用。

**调整策略**：
1. **即时调整**：当系统负载较低时，可以立即调整并发数
2. **延迟调整**：当有任务正在执行时，等待任务完成后再调整
3. **渐进调整**：逐步调整并发数，避免系统资源急剧变化
4. **基于负载的调整**：根据系统CPU、内存等指标动态调整并发数

**实现方案**：
```go
// 系统负载监控和动态调整示例
func MonitorAndAdjustConcurrency(scheduler TaskScheduler) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			// 获取系统负载信息
			cpuLoad := getCPULoad()
			memoryUsage := getMemoryUsage()
			
			// 基于负载确定目标并发数
			targetConcurrency := calculateTargetConcurrency(cpuLoad, memoryUsage)
			
			// 调整并发数
			if err := scheduler.SetMaxConcurrentTasks(targetConcurrency); err != nil {
				log.Printf("调整并发数失败: %v", err)
			}
		}
	}()
}

func calculateTargetConcurrency(cpuLoad, memoryUsage float64) int {
	// 基于CPU和内存负载计算目标并发数
	// 这里是简化的示例，实际应用中可能需要更复杂的算法
	
	// CPU负载权重0.7，内存负载权重0.3
	compositeLoad := cpuLoad*0.7 + memoryUsage*0.3
	
	if compositeLoad > 0.8 {
		return 5  // 高负载，降低并发
	} else if compositeLoad > 0.5 {
		return 10 // 中等负载，维持默认并发
	} else {
		return 20 // 低负载，提高并发
	}
}
```

**设计理由**：
- 提供自动化的并发度调整机制
- 综合考虑CPU和内存负载
- 使用加权算法平衡不同资源的影响
- 采用定时器定期检查和调整
- 支持异步监控，不阻塞主线程

**功能作用**：
- 根据系统负载自动调整并发执行的任务数量
- 平衡系统资源利用率和任务执行效率
- 防止系统过载，保证稳定性
- 提高系统在不同负载下的自适应能力

**使用示例**：
```go
// 启动并发监控	scheduler := NewMemoryTaskScheduler(logger)
MonitorAndAdjustConcurrency(scheduler)
scheduler.Start()

// 系统将自动根据负载调整并发数
```

### 9.3 限流保护

**设计思路**：
除了控制并发执行的任务数量，还需要防止短时间内大量任务涌入导致系统压力过大。实现令牌桶算法的限流机制，可以控制任务提交的速率。

**实现方案**：
```go
// 限流实现示例
type RateLimiter struct {
	rate       int           // 每秒允许的任务数
	burst      int           // 令牌桶容量
	tokens     float64       // 当前令牌数
	lastRefill time.Time     // 上次补充令牌的时间
	mutex      sync.Mutex    // 互斥锁，保证并发安全
}

func NewRateLimiter(rate, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}

func (rl *RateLimiter) TryAcquire() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	// 补充令牌
	now := time.Now()
	duration := now.Sub(rl.lastRefill).Seconds()
	newTokens := duration * float64(rl.rate)
	
	if newTokens > 0 {
		rl.tokens = min(float64(rl.burst), rl.tokens+newTokens)
		rl.lastRefill = now
	}
	
	// 尝试获取令牌
	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}
	
	return false
}

// 在任务调度器中集成限流功能
func (s *MemoryTaskScheduler) Schedule(task Task) error {
	// 检查是否限流
	if s.rateLimiter != nil && !s.rateLimiter.TryAcquire() {
		return fmt.Errorf("任务调度被限流")
	}
	
	// 现有调度逻辑...
	return nil
}

// 添加限流配置选项
func WithRateLimiter(rate, burst int) SchedulerOption {
	return func(s *MemoryTaskScheduler) {
		s.rateLimiter = NewRateLimiter(rate, burst)
	}
}
```

**设计理由**：
- 实现令牌桶算法，支持突发流量
- 提供并发安全的令牌获取机制
- 动态补充令牌，控制任务提交速率
- 通过配置选项集成到调度器
- 在任务提交阶段进行限流，保护系统

**功能作用**：
- 限制短时间内提交的任务数量
- 防止突发流量导致系统压力过大
- 支持一定程度的流量突发
- 与并发控制结合，全面保护系统资源

**使用示例**：
```go
// 创建带限流的任务调度器
rateLimitedScheduler := NewMemoryTaskScheduler(logger, 
	WithMaxConcurrentTasks(10),
	WithRateLimiter(100, 200), // 每秒最多100个任务，突发最多200个
)

// 尝试调度大量任务
for i := 0; i < 1000; i++ {
	task := NewTaskBuilder().
		Immediate().
		WithHandler(func(ctx context.Context) error {
			// 任务处理逻辑
			return nil
		}).
		Build()
		
	err := rateLimitedScheduler.Schedule(task)
	if err != nil {
		fmt.Printf("调度任务失败: %v\n", err)
	}
}
```

## 10. 总结与最佳实践

### 设计亮点

1. **接口驱动设计**：通过清晰的接口定义，实现高内聚低耦合的架构
2. **构建器模式**：提供流畅的API，简化任务创建过程
3. **函数选项模式**：实现灵活的配置，支持未来扩展
4. **并发安全**：通过互斥锁和通道实现线程安全
5. **错误处理**：提供完整的错误处理和恢复机制
6. **资源管理**：支持优雅关闭，避免资源泄露
7. **动态调整**：支持运行时调整并发数和限流策略

### 最佳实践

1. **合理设置并发数**：根据系统资源和任务特性，设置适当的最大并发任务数
2. **使用重试机制**：对可能失败的任务（如网络操作）添加重试逻辑
3. **设置合理超时**：为长时间运行的任务设置超时，避免资源占用
4. **结合监控告警**：监控任务执行情况，及时发现和处理问题
5. **根据负载动态调整**：实现基于系统负载的动态调整策略
6. **使用持久化存储**：在需要恢复任务的场景中使用持久化存储
7. **合理使用日志**：记录关键操作和错误信息，便于问题排查

通过以上设计，任务调度器能够满足各种复杂场景的需求，同时保持良好的性能和稳定性。