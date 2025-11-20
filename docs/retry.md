# 任务重试机制

本文档介绍了 Sparrow 任务调度器中的重试机制，包括重试策略、退避算法、监控和使用示例。

## 概述

重试机制是任务调度系统的重要特性，用于处理临时性故障。当任务执行失败时，系统可以根据配置的策略自动重试任务，提高系统的可靠性和容错性。

## 核心组件

### 1. RetryPolicy（重试策略）

`RetryPolicy` 定义了任务的重试行为：

```go
type RetryPolicy struct {
    MaxRetries        int           // 最大重试次数
    BackoffStrategy   BackoffStrategy // 退避策略
    InitialBackoff    time.Duration  // 初始退避时间
    MaxBackoff        time.Duration  // 最大退避时间
    BackoffMultiplier float64       // 退避倍数
}
```

**字段说明：**
- `MaxRetries`: 任务失败后的最大重试次数，0表示不重试
- `BackoffStrategy`: 退避策略类型（固定、线性、指数）
- `InitialBackoff`: 第一次重试的等待时间
- `MaxBackoff`: 重试等待时间的上限
- `BackoffMultiplier`: 退避时间的增长倍数

### 2. 退避策略

#### 固定退避（Fixed Backoff）
每次重试使用相同的等待时间：

```go
policy := &RetryPolicy{
    MaxRetries:        3,
    BackoffStrategy:   BackoffStrategyFixed,
    InitialBackoff:    1 * time.Second,
    MaxBackoff:        1 * time.Second,
    BackoffMultiplier: 1.0,
}
```

退避时间序列：1s, 1s, 1s, ...

#### 线性退避（Linear Backoff）
每次重试等待时间线性增长：

```go
policy := &RetryPolicy{
    MaxRetries:        3,
    BackoffStrategy:   BackoffStrategyLinear,
    InitialBackoff:    1 * time.Second,
    MaxBackoff:        10 * time.Second,
    BackoffMultiplier: 1.0,
}
```

退避时间序列：1s, 2s, 3s, ...

#### 指数退避（Exponential Backoff）
每次重试等待时间指数增长：

```go
policy := &RetryPolicy{
    MaxRetries:        3,
    BackoffStrategy:   BackoffStrategyExponential,
    InitialBackoff:    1 * time.Second,
    MaxBackoff:        30 * time.Second,
    BackoffMultiplier: 2.0,
}
```

退避时间序列：1s, 2s, 4s, 8s, ...

### 3. RetryableTask 接口

可重试任务需要实现 `RetryableTask` 接口：

```go
type RetryableTask interface {
    Task
    GetRetryPolicy() *RetryPolicy
    GetRetryInfo() *TaskRetryInfo
    IncrementRetry()
    ResetRetry()
}
```

### 4. TaskRetryInfo（重试信息）

`TaskRetryInfo` 记录任务的重试状态：

```go
type TaskRetryInfo struct {
    CurrentRetry  int       // 当前重试次数
    NextRetryAt   time.Time // 下次重试时间
    LastRetryAt   time.Time // 上次重试时间
    LastError     error     // 上次失败错误
}
```

## 使用方法

### 1. 创建可重试任务

使用 `TaskBuilder` 创建带有重试策略的任务：

```go
policy := &RetryPolicy{
    MaxRetries:        3,
    BackoffStrategy:   BackoffStrategyExponential,
    InitialBackoff:    1 * time.Second,
    MaxBackoff:        30 * time.Second,
    BackoffMultiplier: 2.0,
}

task := NewTaskBuilder().
    WithType("retryable_task").
    Immediate().
    WithHandler(func(ctx context.Context) error {
        // 任务执行逻辑
        return doWork()
    }).
    WithRetry(policy).
    Build()
```

### 2. 调度可重试任务

```go
scheduler := NewMemoryTaskScheduler()
defer scheduler.Close(nil)

err := scheduler.ScheduleRetryable(task)
if err != nil {
    log.Fatalf("调度任务失败: %v", err)
}

// 启动调度器
ctx := context.Background()
err = scheduler.Start(ctx)
if err != nil {
    log.Fatalf("启动调度器失败: %v", err)
}
```

## 监控和统计

### RetryMonitor 接口

`RetryMonitor` 接口用于监控重试事件：

```go
type RetryMonitor interface {
    RecordRetry(taskID string, attempt int, err error)
    GetRetryHistory(taskID string) []RetryRecord
    GetStats() RetryStats
}
```

### RetryRecord（重试记录）

```go
type RetryRecord struct {
    TaskID      string        // 任务ID
    Attempt     int           // 尝试次数
    Error       error         // 错误信息
    Timestamp   time.Time     // 时间戳
    NextRetryAt time.Time     // 下次重试时间
}
```

### RetryStats（重试统计）

```go
type RetryStats struct {
    TotalRetries      int     // 总重试次数
    SuccessfulRetries int     // 成功重试次数
    FailedRetries     int     // 失败重试次数
    AverageRetries    float64 // 平均重试次数
}
```

## 死信队列

当任务重试次数超过最大限制时，任务会被移到死信队列：

```go
type DeadLetterQueue interface {
    Add(task RetryableTask, finalError error)
    GetByTaskID(taskID string) (*DeadLetterTask, bool)
    RemoveByTaskID(taskID string) bool
    GetAll() []DeadLetterTask
    Size() int
    Clear()
}
```

## 最佳实践

### 1. 选择合适的退避策略

- **固定退避**: 适用于快速恢复的服务故障
- **线性退避**: 适用于负载逐渐增加的场景
- **指数退避**: 适用于网络请求或外部服务调用

### 2. 设置合理的重试参数

```go
// 推荐配置示例
policy := &RetryPolicy{
    MaxRetries:        3,                    // 不超过5次
    BackoffStrategy:   BackoffStrategyExponential,
    InitialBackoff:    1 * time.Second,      // 从1秒开始
    MaxBackoff:        60 * time.Second,     // 最大1分钟
    BackoffMultiplier: 2.0,                  // 指数增长
}
```

### 3. 处理特定错误

```go
func isRetryableError(err error) bool {
    // 只对特定类型的错误进行重试
    return errors.Is(err, ErrTemporaryFailure) ||
           errors.Is(err, context.DeadlineExceeded)
}
```

### 4. 监控重试指标

```go
// 定期检查重试统计
stats := monitor.GetStats()
if stats.FailedRetries > stats.SuccessfulRetries*2 {
    log.Printf("警告: 重试失败率过高: %d/%d", 
        stats.FailedRetries, stats.TotalRetries)
}
```

## 完整示例

参考 `examples/retry_example.go` 文件，其中包含了各种重试策略的完整使用示例。

## 注意事项

1. **幂等性**: 确保任务处理逻辑是幂等的，避免重复执行造成问题
2. **资源限制**: 设置合理的重试次数，避免无限重试消耗系统资源
3. **错误分类**: 区分临时性错误和永久性错误，只对临时性错误进行重试
4. **监控告警**: 建立重试指标的监控和告警机制
5. **死信处理**: 定期处理死信队列中的任务，避免积累过多

## 性能考虑

- 重试队列使用最小堆实现，时间复杂度为 O(log n)
- 死信队列有大小限制（1000个任务），防止内存泄漏
- 重试统计信息在内存中维护，重启后会丢失

## 扩展性

系统设计支持扩展：

1. 自定义退避策略：实现 `BackoffCalculator` 接口
2. 自定义监控器：实现 `RetryMonitor` 接口
3. 自定义死信处理：实现 `DeadLetterQueue` 接口
4. 集成外部存储：将重试信息持久化到数据库