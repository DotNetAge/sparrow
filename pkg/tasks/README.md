# Sparrow 任务调度系统

Sparrow 任务调度系统是一个轻量级、高性能的Go语言任务调度框架，支持多种执行模式、重试机制和清理策略。

## 核心特性

- **多种调度器类型**：支持单一调度器和混合调度器
- **执行模式**：并发、顺序、流水线三种执行模式
- **重试机制**：支持多种退避策略和可配置的重试策略
- **任务生命周期管理**：完整的任务状态跟踪和管理
- **自动清理**：基于时间和数量的任务清理策略
- **统计监控**：提供详细的执行统计和重试监控
- **优雅关闭**：支持任务的优雅取消和资源清理

## 架构设计

### 核心接口

#### TaskScheduler 接口
```go
type TaskScheduler interface {
    usecase.GracefulClose
    usecase.Startable
    Schedule(task Task) error
    Stop() error
    Cancel(taskID string) error
    GetTaskStatus(taskID string) (TaskStatus, error)
    ListTasks() []TaskInfo
    SetMaxConcurrentTasks(max int) error
}
```

#### Task 接口
```go
type Task interface {
    ID() string
    Type() string
    Schedule() time.Time
    Handler() func(ctx context.Context) error
    OnComplete() func(ctx context.Context, err error)
    OnCancel() func(ctx context.Context)
    IsRecurring() bool
    GetInterval() time.Duration
}
```

### 任务状态

- `waiting`: 等待执行
- `running`: 正在执行
- `completed`: 执行成功
- `failed`: 执行失败
- `cancelled`: 已取消
- `retrying`: 重试中
- `dead_letter`: 死信（重试次数耗尽）

### 执行模式

- `ExecutionModeConcurrent`: 并发执行（默认）
- `ExecutionModeSequential`: 顺序执行
- `ExecutionModePipeline`: 流水线执行

## 调度器类型

### 1. MemoryTaskScheduler - 单一调度器

内存任务调度器，支持所有基础功能：

```go
// 创建调度器
scheduler := NewMemoryTaskScheduler(
    WithWorkerCount(5),              // 工作协程数
    WithMaxConcurrentTasks(10),      // 最大并发任务数
    WithCleanupPolicy(policy),       // 清理策略
    WithLogger(logger),              // 日志记录器
)

// 启动调度器
ctx := context.Background()
err := scheduler.Start(ctx)
if err != nil {
    log.Fatal(err)
}
defer scheduler.Close(ctx)
```

#### 执行模式控制

单一调度器支持动态切换执行模式：

```go
// 设置为顺序执行模式
scheduler.SetExecutionMode(ExecutionModeSequential)

// 设置为流水线执行模式  
scheduler.SetExecutionMode(ExecutionModePipeline)

// 设置为并发执行模式
scheduler.SetExecutionMode(ExecutionModeConcurrent)

// 获取当前执行模式
mode := scheduler.GetExecutionMode()
```

### 2. HybridTaskScheduler - 混合调度器

混合调度器根据任务类型自动选择执行策略：

```go
// 创建混合调度器
scheduler := NewHybridTaskScheduler(
    WithHybridWorkerCount(5, 1, 1),     // 并发、顺序、流水线工作协程数
    WithHybridMaxConcurrentTasks(10),    // 最大并发任务数
    WithHybridLogger(logger),            // 日志记录器
)

// 启动调度器
err := scheduler.Start(ctx)
if err != nil {
    log.Fatal(err)
}
defer scheduler.Close(ctx)

// 注册任务类型策略
scheduler.RegisterTaskPolicy("concurrent-task", PolicyConcurrent)
scheduler.RegisterTaskPolicy("sequential-task", PolicySequential)
scheduler.RegisterTaskPolicy("pipeline-task", PolicyPipeline)
```

#### 执行策略

- `PolicyConcurrent`: 并发执行策略（默认）
- `PolicySequential`: 顺序执行策略
- `PolicyPipeline`: 流水线执行策略

## 重试机制

### 重试策略配置

```go
// 使用TaskBuilder创建可重试任务
task := NewTaskBuilder().
    WithType("retryable-task").
    Immediate().
    WithHandler(func(ctx context.Context) error {
        // 任务逻辑
        return doWork()
    }).
    WithRetry(3).                                    // 最大重试3次
    WithExponentialBackoff(1 * time.Second).        // 指数退避，初始1秒
    WithMaxDelay(30 * time.Second).                 // 最大延迟30秒
    Build()
```

### 退避策略

- `BackoffStrategyFixed`: 固定间隔重试
- `BackoffStrategyLinear`: 线性增长间隔
- `BackoffStrategyExponential`: 指数增长间隔（默认）

### 重试监控

```go
// 获取重试统计
stats := scheduler.GetRetryStats()
fmt.Printf("总重试次数: %d\n", stats["total_retries"])
fmt.Printf("成功重试: %d\n", stats["successful_retries"])
fmt.Printf("失败重试: %d\n", stats["failed_retries"])
fmt.Printf("死信任务: %d\n", stats["dead_letter_count"])
fmt.Printf("平均重试时间: %s\n", stats["average_retry_time"])
```

## 任务创建

### 基础任务

```go
// 即时执行任务
task := NewTaskBuilder().
    WithType("immediate-task").
    Immediate().
    WithHandler(func(ctx context.Context) error {
        fmt.Println("任务执行")
        return nil
    }).
    Build()

// 定时执行任务
task := NewTaskBuilder().
    WithType("scheduled-task").
    ScheduleAt(time.Now().Add(1 * time.Hour)).
    WithHandler(func(ctx context.Context) error {
        fmt.Println("定时任务执行")
        return nil
    }).
    Build()

// 周期性任务
task := NewTaskBuilder().
    WithType("recurring-task").
    ScheduleRecurring(30 * time.Minute).
    WithHandler(func(ctx context.Context) error {
        fmt.Println("周期性任务执行")
        return nil
    }).
    Build()
```

### 可重试任务

```go
retryableTask := NewTaskBuilder().
    WithType("retryable-task").
    Immediate().
    WithHandler(func(ctx context.Context) error {
        // 可能失败的任务逻辑
        return doRiskyWork()
    }).
    WithRetry(5).                              // 最大重试5次
    WithLinearBackoff(2 * time.Second).        // 线性退避，每次增加2秒
    WithMaxDelay(1 * time.Minute).             // 最大延迟1分钟
    Build()
```

### 任务回调

```go
task := NewTaskBuilder().
    WithType("callback-task").
    Immediate().
    WithHandler(func(ctx context.Context) error {
        return doWork()
    }).
    WithOnComplete(func(ctx context.Context, err error) {
        if err != nil {
            fmt.Printf("任务失败: %v\n", err)
        } else {
            fmt.Println("任务成功完成")
        }
    }).
    WithOnCancel(func(ctx context.Context) {
        fmt.Println("任务被取消")
    }).
    Build()
```

## 清理策略

### 清理策略配置

```go
// 使用默认清理策略
scheduler := NewMemoryTaskScheduler(
    WithCleanupPolicy(DefaultCleanupPolicy()),
)

// 自定义清理策略
policy := &CleanupPolicy{
    CompletedTaskTTL:   1 * time.Hour,     // 已完成任务保留1小时
    FailedTaskTTL:      2 * time.Hour,     // 失败任务保留2小时
    CancelledTaskTTL:   30 * time.Minute,  // 已取消任务保留30分钟
    MaxCompletedTasks:  500,               // 最多保留500个已完成任务
    MaxFailedTasks:     200,               // 最多保留200个失败任务
    CleanupInterval:    10 * time.Minute,  // 每10分钟清理一次
    EnableAutoCleanup:  true,              // 启用自动清理
}

scheduler := NewMemoryTaskScheduler(
    WithCleanupPolicy(policy),
)
```

### 清理统计

```go
// 获取清理统计
stats := scheduler.GetCleanupStats()
fmt.Printf("总任务数: %d\n", stats["total_tasks"])
fmt.Printf("状态分布: %v\n", stats["status_counts"])
fmt.Printf("清理策略: %v\n", stats["cleanup_policy"])
```

## 运行策略调节

### 动态配置调整

```go
// 调整最大并发任务数
scheduler.SetMaxConcurrentTasks(20)

// 切换执行模式
scheduler.SetExecutionMode(ExecutionModeSequential)

// 获取当前执行模式
mode := scheduler.GetExecutionMode()
```

### 混合调度器策略管理

```go
// 动态注册任务策略
scheduler.RegisterTaskPolicy("new-task-type", PolicyConcurrent)

// 获取执行统计
stats := scheduler.GetStats()
for policy, stat := range stats {
    fmt.Printf("策略 %s: 总任务=%d, 完成=%d, 失败=%d, 进行中=%d\n",
        policy, stat.TotalTasks, stat.CompletedTasks,
        stat.FailedTasks, stat.RunningTasks)
}
```

## 高级功能

### 任务取消

```go
// 调度任务
err := scheduler.Schedule(task)
if err != nil {
    return err
}

// 取消任务
err = scheduler.Cancel(task.ID())
if err != nil {
    fmt.Printf("取消任务失败: %v\n", err)
}
```

### 状态查询

```go
// 获取任务状态
status, err := scheduler.GetTaskStatus(task.ID())
if err != nil {
    fmt.Printf("获取状态失败: %v\n", err)
} else {
    fmt.Printf("任务状态: %s\n", status)
}

// 列出所有任务
tasks := scheduler.ListTasks()
for _, task := range tasks {
    fmt.Printf("任务 %s: %s (%s)\n", task.ID, task.Type, task.Status)
}
```

### 统计信息

```go
// 单一调度器统计
retryStats := scheduler.GetRetryStats()
cleanupStats := scheduler.GetCleanupStats()

// 混合调度器统计
if hybridScheduler, ok := scheduler.(*HybridTaskScheduler); ok {
    stats := hybridScheduler.GetStats()
    for policy, stat := range stats {
        fmt.Printf("%s策略统计: %+v\n", policy, stat)
    }
}
```

## 最佳实践

### 1. 选择合适的调度器
- **单一调度器**：适用于简单的任务调度需求
- **混合调度器**：适用于需要多种执行策略的复杂场景

### 2. 合理配置工作协程
- 并发任务：根据CPU核心数和IO密集度配置
- 顺序任务：通常设置为1
- 流水线任务：根据流水线阶段数配置

### 3. 重试策略建议
- 网络请求：使用指数退避，初始延迟1-2秒
- 数据库操作：使用线性退避，避免雪崩
- 外部API调用：设置合理的最大延迟和重试次数

### 4. 清理策略配置
- 高频任务：缩短TTL，减少内存占用
- 重要任务：延长TTL，便于问题排查
- 长期运行：定期监控清理效果

## 配置选项参考

### 单一调度器配置

```go
// 配置选项
WithWorkerCount(int)                    // 工作协程数
WithMaxConcurrentTasks(int)             // 最大并发任务数
WithLogger(*logger.Logger)              // 日志记录器
WithCleanupPolicy(*CleanupPolicy)       // 清理策略
```

### 混合调度器配置

```go
// 配置选项
WithHybridWorkerCount(concurrent, sequential, pipeline int)  // 各策略工作协程数
WithHybridMaxConcurrentTasks(int)                            // 最大并发任务数
WithHybridLogger(*logger.Logger)                             // 日志记录器
```

### 清理策略配置

```go
// 默认值
CompletedTaskTTL:   30 * time.Minute
FailedTaskTTL:      60 * time.Minute
CancelledTaskTTL:   15 * time.Minute
MaxCompletedTasks:  1000
MaxFailedTasks:     500
CleanupInterval:    5 * time.Minute
EnableAutoCleanup:  true
```

## 故障排除

### 常见问题

1. **任务不执行**
   - 检查调度器是否已启动
   - 确认工作协程数配置
   - 验证任务调度时间

2. **重试不生效**
   - 确认任务实现了RetryableTask接口
   - 检查重试策略配置
   - 验证错误类型是否可重试

3. **内存占用过高**
   - 调整清理策略TTL
   - 减少最大任务数量限制
   - 缩短清理间隔

4. **执行顺序问题**
   - 顺序执行模式需要设置工作协程数为1
   - 流水线执行需要任务间依赖协调
   - 检查执行模式配置

### 调试技巧

```go
// 启用详细日志
logger := logger.New(logger.WithLevel("debug"))
scheduler := NewMemoryTaskScheduler(WithLogger(logger))

// 监控统计信息
go func() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        stats := scheduler.GetRetryStats()
        fmt.Printf("重试统计: %+v\n", stats)
    }
}()
```

## 示例项目

完整的使用示例请参考：
- `examples/basic/` - 基础任务调度示例
- `examples/hybrid/` - 混合调度器示例
- `examples/retry/` - 重试机制示例
- `examples/pipeline/` - 流水线执行示例

## API参考

详细的API文档请参考：
- [接口定义](core.go)
- [内存调度器实现](memory.go)
- [混合调度器实现](hybrid_scheduler.go)
- [重试机制实现](retry.go)
- [清理策略实现](cleanup.go)