# Sparrow App 任务系统集成指南

本文档详细说明如何在 Sparrow 应用程序中初始化和使用任务系统，重点介绍直接使用任务系统的方法，而非通过 TaskBuilder。

## 目录

- [任务系统概述](#任务系统概述)
- [初始化任务系统](#初始化任务系统)
- [任务系统类型](#任务系统类型)
- [直接使用任务系统](#直接使用任务系统)
- [任务管理操作](#任务管理操作)
- [高级配置](#高级配置)
- [最佳实践](#最佳实践)
- [故障排除](#故障排除)

## 任务系统概述

Sparrow 提供了三种任务系统配置方式：

1. **基础任务系统** (`Tasks`) - 单一调度器，支持并发、顺序、流水线执行模式
2. **混合任务系统** (`HybridTasks`) - 多调度器，根据任务类型自动选择执行策略
3. **高级任务系统** (`AdvancedTasks`) - 最灵活的配置方式，支持细粒度策略控制

## 初始化任务系统

### 1. 基础任务系统

使用 `Tasks` 选项初始化基础任务系统：

```go
package main

import (
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
    "github.com/DotNetAge/sparrow/pkg/tasks"
)

func main() {
    app := bootstrap.NewApp(
        // 使用默认配置（并发模式，5个工作协程，最大并发任务数10）
        bootstrap.Tasks(),
        
        // 或者使用自定义配置
        bootstrap.Tasks(
            bootstrap.WithSequentialMode(),                  // 设置为顺序执行模式（单线程）
        ),
        
        // 并发模式示例
        // bootstrap.Tasks(
        //     bootstrap.WithConcurrentMode(),               // 设置为并发执行模式
        //     bootstrap.WithWorkerCount(8),                 // 设置工作协程数
        //     bootstrap.WithMaxConcurrentTasks(20),         // 设置最大并发任务数
        // ),
    )
    
    app.Start()
}
```

#### 执行模式说明

**重要提示**：不同的执行模式对工作协程数和并发配置有不同的处理：

- **并发模式** (`ExecutionModeConcurrent`)：
  - 使用指定的工作协程数（默认5个）
  - 支持最大并发任务数限制（默认10个）
  - 任务可以并行执行

- **顺序模式** (`ExecutionModeSequential`)：
  - **自动强制使用1个工作协程**，无论配置多少个
  - 忽略最大并发任务数配置（因为同时只能执行1个任务）
  - 任务严格按照提交顺序串行执行

- **流水线模式** (`ExecutionModePipeline`)：
  - **自动强制使用1个工作协程**，无论配置多少个
  - 忽略最大并发任务数配置
  - 任务按阶段串行执行，适用于分阶段处理

**代码示例**：
```go
// 并发模式 - 所有配置都生效
bootstrap.Tasks(
    bootstrap.WithConcurrentMode(),
    bootstrap.WithWorkerCount(10),           // 使用10个工作协程
    bootstrap.WithMaxConcurrentTasks(50),    // 最大并发50个任务
)

// 顺序模式 - 简洁配置
bootstrap.Tasks(
    bootstrap.WithSequentialMode(),
)

// 流水线模式 - 简洁配置  
bootstrap.Tasks(
    bootstrap.WithPipelineMode(),
)
```

### 2. 混合任务系统

使用 `HybridTasks` 选项初始化混合任务系统：

```go
package main

import (
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
    "github.com/DotNetAge/sparrow/pkg/tasks"
)

func main() {
    app := bootstrap.NewApp(
        bootstrap.HybridTasks(
            // 自定义工作协程数：并发、顺序、流水线
            tasks.WithHybridWorkerCount(10, 2, 2),
            tasks.WithHybridMaxConcurrentTasks(50),
            tasks.WithHybridLogger(app.Logger),
        ),
    )
    
    app.Start()
}
```

### 3. 高级任务系统

使用 `AdvancedTasks` 选项初始化高级任务系统：

```go
package main

import (
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
    "github.com/DotNetAge/sparrow/pkg/tasks"
)

func main() {
    app := bootstrap.NewApp(
        bootstrap.AdvancedTasks(
            bootstrap.WithConcurrentWorkers(10),
            bootstrap.WithAdvancedMaxConcurrentTasks(50),
            bootstrap.EnableSequentialExecution(),      // 启用顺序执行
            bootstrap.EnablePipelineExecution(),        // 启用流水线执行
            
            // 指定任务类型的执行策略
            bootstrap.WithSequentialType("report", "backup"),
            bootstrap.WithConcurrentType("notification", "email"),
            bootstrap.WithPipelineType("data-processing"),
        ),
    )
    
    app.Start()
}
```

## 任务系统类型

### 基础任务系统特性

- **单一调度器**：使用 `MemoryTaskScheduler`
- **执行模式**：支持并发、顺序、流水线三种模式
- **内存存储**：任务存储在内存中，重启后丢失
- **自动清理**：支持基于TTL和数量的任务清理

### 混合任务系统特性

- **多调度器**：包含并发、顺序、流水线三个调度器
- **策略路由**：根据任务类型自动选择执行策略
- **类型注册**：支持动态注册任务类型策略
- **统计信息**：提供详细的执行统计

### 高级任务系统特性

- **灵活配置**：最细粒度的配置选项
- **策略映射**：支持任务类型到执行策略的映射
- **模式开关**：可独立启用/禁用各种执行模式
- **企业级**：适合复杂的业务场景

## 任务重试能力

### 默认重试支持

从设计理念上，Sparrow 任务系统将重试能力作为生产系统的必要功能，所有 `RunTaskXXX` 系列方法都默认支持重试。重试能力通过 `WithRetry()` 方法以 Option 模式进行配置，默认配置已经过优化，无需额外配置即可获得最佳实践。

### 默认重试配置

默认重试配置已针对生产环境进行了优化，具有以下特性：

- 最大重试次数：3次
- 初始退避时间：1秒
- 最大退避时间：30秒
- 退避策略：指数退避（倍数2.0）
- 自动启用：无需额外配置

### 配置重试能力

#### 1. 使用默认重试配置（推荐）

最简单也是最佳实践的方式是直接使用默认配置，无需指定任何参数：

```go
package main

import (
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
    app := bootstrap.NewApp(
        bootstrap.Tasks(),                    // 初始化任务系统
        bootstrap.WithRetry(),                // 启用默认重试配置（推荐）
    )
    
    app.Start()
}
```

#### 2. 重试配置选项使用详解

Sparrow 提供了以下重试配置选项，每个选项都可以独立使用：

##### 2.1 设置最大重试次数

```go
bootstrap.WithRetry(
    bootstrap.WithMaxRetries(5),  // 设置最大重试次数为5次
)
```

##### 2.2 设置初始退避时间

```go
bootstrap.WithRetry(
    bootstrap.WithInitialBackoff(2*time.Second),  // 设置初始退避时间为2秒
)
```

##### 2.3 设置最大退避时间

```go
bootstrap.WithRetry(
    bootstrap.WithMaxBackoff(60*time.Second),  // 设置最大退避时间为60秒
)
```

##### 2.4 设置指数退避策略

```go
bootstrap.WithRetry(
    bootstrap.WithExponentialBackoff(2.5),  // 设置指数退避倍数为2.5
)
```

##### 2.5 设置线性退避策略

```go
bootstrap.WithRetry(
    bootstrap.WithLinearBackoff(),  // 启用线性退避策略
    bootstrap.WithInitialBackoff(1*time.Second),  // 线性退避的基础时间
    bootstrap.WithMaxBackoff(5*time.Second),  // 线性退避的最大时间
)
```

##### 2.6 设置固定退避策略

```go
bootstrap.WithRetry(
    bootstrap.WithFixedBackoff(),  // 启用固定退避策略
    bootstrap.WithInitialBackoff(3*time.Second),  // 固定退避时间为3秒
)
```

##### 2.7 禁用重试（不推荐）

```go
bootstrap.WithRetry(
    bootstrap.DisableRetry(),  // 禁用重试功能（不推荐用于生产环境）
)
```

#### 3. 复合配置示例

您可以根据需要组合多个配置选项：

```go
package main

import (
    "time"
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
    app := bootstrap.NewApp(
        bootstrap.Tasks(),
        bootstrap.WithRetry(
            bootstrap.WithMaxRetries(5),           // 最大重试5次
            bootstrap.WithInitialBackoff(1*time.Second),  // 初始退避1秒
            bootstrap.WithMaxBackoff(30*time.Second),      // 最大退避30秒
            bootstrap.WithExponentialBackoff(2.0),         // 指数退避策略
        ),
    )
    
    app.Start()
}
```

#### 4. 退避策略使用对比

**固定退避策略**：每次重试间隔相同时间
```go
// 简化方式（推荐）
bootstrap.WithRetry(
    bootstrap.WithMaxRetries(3),
    bootstrap.WithFixedBackoff(),
    bootstrap.WithInitialBackoff(2*time.Second),
)

// 等效的完整配置
bootstrap.WithRetry(
    bootstrap.WithMaxRetries(3),
    bootstrap.WithInitialBackoff(2*time.Second),
    bootstrap.WithMaxBackoff(2*time.Second),
    bootstrap.WithBackoffMultiplier(1.0),
)
```

**线性退避策略**：每次重试间隔线性增加
```go
// 简化方式（推荐）
bootstrap.WithRetry(
    bootstrap.WithMaxRetries(5),
    bootstrap.WithLinearBackoff(),
    bootstrap.WithInitialBackoff(1*time.Second),
    bootstrap.WithMaxBackoff(5*time.Second),
)

// 等效的完整配置
bootstrap.WithRetry(
    bootstrap.WithMaxRetries(5),
    bootstrap.WithInitialBackoff(1*time.Second),
    bootstrap.WithMaxBackoff(5*time.Second),
    bootstrap.WithBackoffMultiplier(1.0),
)
```

**指数退避策略**（默认）：每次重试间隔指数增加
```go
// 简化方式（推荐）
bootstrap.WithRetry(
    bootstrap.WithMaxRetries(5),
    bootstrap.WithExponentialBackoff(2.0),
    bootstrap.WithInitialBackoff(1*time.Second),
    bootstrap.WithMaxBackoff(60*time.Second),
)

// 等效的完整配置
bootstrap.WithRetry(
    bootstrap.WithMaxRetries(5),
    bootstrap.WithInitialBackoff(1*time.Second),
    bootstrap.WithMaxBackoff(60*time.Second),
    bootstrap.WithBackoffMultiplier(2.0),
)

### 重试能力使用示例

配置重试能力后，所有 `RunTaskXXX` 系列方法都自动支持重试：

```go
// 立即执行任务 - 自动支持重试
taskId := app.RunTask(func(ctx context.Context) error {
    // 可能失败的任务逻辑
    return sendEmail(ctx, "user@example.com", "Hello World")
})

// 定时任务 - 自动支持重试
executeTime := time.Now().Add(5 * time.Minute)
taskId = app.RunTaskAt(executeTime, func(ctx context.Context) error {
    // 可能失败的定时任务
    return processPayment(ctx, paymentId)
})

// 周期性任务 - 自动支持重试
interval := 1 * time.Hour
taskId = app.RunTaskRecurring(interval, func(ctx context.Context) error {
    // 可能失败的周期性任务
    return syncData(ctx)
})

// 类型化任务 - 自动支持重试
taskId = app.RunTypedTask("notification", func(ctx context.Context) error {
    // 可能失败的通知任务
    return sendPushNotification(ctx, userId, message)
})
```

### 重试行为说明

1. **重试触发条件**：任务处理函数返回非 nil 错误时触发重试
2. **重试间隔**：根据配置的退避策略计算下次重试时间
3. **重试限制**：达到最大重试次数后，任务标记为失败
4. **调度器支持**：
   - `MemoryTaskScheduler`：支持基础重试功能
   - `HybridTaskScheduler`：支持完整重试功能和死信队列
   - 建议在生产环境中使用 `HybridTasks` 或 `AdvancedTasks`

### 重试配置最佳实践

```go
// 生产环境推荐配置
app := bootstrap.NewApp(
    bootstrap.HybridTasks(),                    // 使用混合调度器
    bootstrap.WithRetry(
        bootstrap.WithMaxRetries(3),            // 最多重试3次
        bootstrap.WithInitialBackoff(1*time.Second),
        bootstrap.WithMaxBackoff(10*time.Second),
        bootstrap.WithBackoffMultiplier(2.0),   // 指数退避
    ),
)

// 开发环境配置
app := bootstrap.NewApp(
    bootstrap.Tasks(),                          // 使用基础调度器
    bootstrap.WithRetry(
        bootstrap.WithMaxRetries(1),            // 开发环境少重试
        bootstrap.WithInitialBackoff(500*time.Millisecond),
        bootstrap.WithMaxBackoff(2*time.Second),
        bootstrap.WithBackoffMultiplier(1.5),
    ),
)
```

## 直接使用任务系统

### 获取任务调度器

任务系统初始化后，可以通过 `App.Scheduler` 字段直接访问：

```go
// 获取任务调度器
scheduler := app.Scheduler

// 或者直接使用 app.Scheduler
```

### 提交基础任务

#### 立即执行任务

```go
// 创建并提交立即执行的任务
taskId := app.RunTask(func(ctx context.Context) error {
    // 任务逻辑
    app.Logger.Info("执行任务...")
    
    // 模拟工作
    time.Sleep(2 * time.Second)
    
    app.Logger.Info("任务完成")
    return nil
})

app.Logger.Info("任务已提交", "taskId", taskId)
```

#### 定时执行任务

```go
// 创建并提交定时执行的任务
executeTime := time.Now().Add(5 * time.Minute)
taskId := app.RunTaskAt(executeTime, func(ctx context.Context) error {
    app.Logger.Info("定时任务执行中...")
    
    // 执行定时任务逻辑
    return doScheduledWork(ctx)
})

app.Logger.Info("定时任务已提交", "taskId", taskId, "executeTime", executeTime)
```

#### 周期性执行任务

```go
// 创建并提交周期性执行的任务
interval := 10 * time.Minute
taskId := app.RunTaskRecurring(interval, func(ctx context.Context) error {
    app.Logger.Info("周期性任务执行中...")
    
    // 执行周期性任务逻辑
    return doRecurringWork(ctx)
})

app.Logger.Info("周期性任务已提交", "taskId", taskId, "interval", interval)
```

### 提交类型化任务（混合/高级任务系统）

对于混合和高级任务系统，可以指定任务类型：

```go
// 提交指定类型的任务
taskId := app.RunTypedTask("notification", func(ctx context.Context) error {
    app.Logger.Info("发送通知任务...")
    
    // 发送通知逻辑
    return sendNotification(ctx)
})

// 提交定时类型化任务
executeTime := time.Now().Add(1 * time.Hour)
taskId = app.RunTypedTaskAt("report", executeTime, func(ctx context.Context) error {
    app.Logger.Info("生成报告任务...")
    
    // 生成报告逻辑
    return generateReport(ctx)
})

// 提交周期性类型化任务
interval := 24 * time.Hour
taskId = app.RunTypedTaskRecurring("backup", interval, func(ctx context.Context) error {
    app.Logger.Info("数据备份任务...")
    
    // 数据备份逻辑
    return backupData(ctx)
})
```

### 直接操作调度器

除了使用 App 提供的便捷方法，还可以直接操作调度器：

```go
import "github.com/DotNetAge/sparrow/pkg/tasks"

// 创建任务（不使用 TaskBuilder）
task := &tasks.BuiltTask{
    ID:          "custom-task-id",
    Type:        "custom-type",
    Status:      tasks.TaskStatusPending,
    ScheduledAt: time.Now(),
    Handler: func(ctx context.Context) error {
        app.Logger.Info("自定义任务执行...")
        return doCustomWork(ctx)
    },
}

// 提交任务
err := app.Scheduler.Schedule(task)
if err != nil {
    app.Logger.Error("提交任务失败", "error", err)
}
```

## 任务管理操作

### 查询任务状态

```go
// 查询特定任务状态
status, err := app.Scheduler.GetTaskStatus("task-id")
if err != nil {
    app.Logger.Error("查询任务状态失败", "error", err)
} else {
    app.Logger.Info("任务状态", "taskId", "task-id", "status", status)
}

// 列出所有任务
tasks, err := app.Scheduler.ListTasks()
if err != nil {
    app.Logger.Error("列出任务失败", "error", err)
} else {
    for _, task := range tasks {
        app.Logger.Info("任务信息", 
            "id", task.ID,
            "type", task.Type,
            "status", task.Status,
            "scheduledAt", task.ScheduledAt,
        )
    }
}
```

### 取消任务

```go
// 取消特定任务
err := app.Scheduler.Cancel("task-id")
if err != nil {
    app.Logger.Error("取消任务失败", "error", err)
} else {
    app.Logger.Info("任务已取消", "taskId", "task-id")
}
```

### 动态配置调整

```go
// 调整最大并发任务数
err := app.Scheduler.SetMaxConcurrentTasks(100)
if err != nil {
    app.Logger.Error("调整并发数失败", "error", err)
}

// 对于基础任务系统，可以调整执行模式
if memoryScheduler, ok := app.Scheduler.(*tasks.MemoryTaskScheduler); ok {
    err := memoryScheduler.SetExecutionMode(tasks.ExecutionModeSequential)
    if err != nil {
        app.Logger.Error("设置执行模式失败", "error", err)
    }
}
```

### 获取统计信息

```go
// 获取任务统计信息
stats := app.Scheduler.GetStats()
app.Logger.Info("任务统计",
    "total", stats.Total,
    "pending", stats.Pending,
    "running", stats.Running,
    "completed", stats.Completed,
    "failed", stats.Failed,
    "cancelled", stats.Cancelled,
)

// 对于支持重试的调度器，获取重试统计
if retryScheduler, ok := app.Scheduler.(interface{ GetRetryStats() *tasks.RetryStats }); ok {
    retryStats := retryScheduler.GetRetryStats()
    app.Logger.Info("重试统计",
        "totalRetries", retryStats.TotalRetries,
        "successfulRetries", retryStats.SuccessfulRetries,
        "deadLetterTasks", retryStats.DeadLetterTasks,
    )
}
```

## 高级配置

### 注册任务策略（混合任务系统）

```go
// 获取混合调度器实例
if hybridScheduler, ok := app.Scheduler.(*tasks.HybridTaskScheduler); ok {
    // 注册任务类型策略
    err := hybridScheduler.RegisterTaskPolicy("urgent", tasks.PolicyConcurrent)
    if err != nil {
        app.Logger.Error("注册紧急任务策略失败", "error", err)
    }
    
    err = hybridScheduler.RegisterTaskPolicy("batch", tasks.PolicySequential)
    if err != nil {
        app.Logger.Error("注册批处理任务策略失败", "error", err)
    }
    
    err = hybridScheduler.RegisterTaskPolicy("pipeline", tasks.PolicyPipeline)
    if err != nil {
        app.Logger.Error("注册流水线任务策略失败", "error", err)
    }
}
```

### 配置清理策略

```go
// 对于 MemoryTaskScheduler，可以配置清理策略
if memoryScheduler, ok := app.Scheduler.(*tasks.MemoryTaskScheduler); ok {
    // 这个需要在创建调度器时通过 options 配置
    // 或者通过调度器的内部方法设置（如果有暴露的话）
}
```

## 最佳实践

### 1. 选择合适的任务系统

- **简单应用**：使用基础任务系统 (`Tasks`)
- **多类型任务**：使用混合任务系统 (`HybridTasks`)
- **企业级应用**：使用高级任务系统 (`AdvancedTasks`)

### 2. 执行模式配置注意事项

**关键原则**：根据执行模式特点进行合理配置

```go
// ✅ 正确：并发模式配置
bootstrap.Tasks(
    bootstrap.WithConcurrentMode(),
    bootstrap.WithWorkerCount(10),           // 合理：多协程提升并发性能
    bootstrap.WithMaxConcurrentTasks(50),    // 合理：控制并发数避免资源耗尽
)

// ❌ 错误：顺序模式配置（浪费配置参数，制造歧义）
bootstrap.Tasks(
    bootstrap.WithSequentialMode(),
    bootstrap.WithWorkerCount(10),           // 无意义：实际只使用1个协程
    bootstrap.WithMaxConcurrentTasks(50),    // 无意义：顺序执行无并发概念
)

// ✅ 正确：顺序模式简洁配置
bootstrap.Tasks(
    bootstrap.WithSequentialMode(),          // 足够：系统自动处理为单线程
)

// ✅ 正确：流水线模式简洁配置
bootstrap.Tasks(
    bootstrap.WithPipelineMode(),            // 足够：系统自动处理为单线程
)
```

### 3. 混合任务系统的优势

当需要同时支持不同执行模式时，优先选择混合任务系统：

```go
// ✅ 推荐：混合任务系统
bootstrap.HybridTasks(
    tasks.WithHybridWorkerCount(10, 1, 1),   // 并发10个，顺序1个，流水线1个
    tasks.WithHybridMaxConcurrentTasks(50),  // 仅对并发调度器有效
)
```

### 4. 任务类型命名

```go
// 使用有意义的任务类型名称
const (
    TaskTypeNotification = "notification"
    TaskTypeReport       = "report"
    TaskTypeBackup       = "backup"
    TaskTypeEmail        = "email"
    TaskTypeCleanup      = "cleanup"
)
```

### 5. 错误处理

```go
// 在任务处理函数中进行适当的错误处理
taskId := app.RunTask(func(ctx context.Context) error {
    // 检查上下文是否已取消
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    
    // 执行业务逻辑
    if err := doBusinessLogic(); err != nil {
        app.Logger.Error("业务逻辑执行失败", "error", err)
        return err // 返回错误以触发重试机制
    }
    
    return nil
})
```

### 6. 资源清理

```go
// 在应用关闭时，任务系统会自动清理
// 但可以手动确保所有任务完成
func (app *App) gracefulShutdown() {
    // 等待正在运行的任务完成
    if app.Scheduler != nil {
        app.Logger.Info("等待任务系统关闭...")
        // 任务系统会在 CleanUp() 中自动关闭
    }
}
```

### 7. 监控和日志

```go
// 定期输出任务统计信息
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            stats := app.Scheduler.GetStats()
            app.Logger.Info("任务系统统计",
                "total", stats.Total,
                "pending", stats.Pending,
                "running", stats.Running,
                "completed", stats.Completed,
                "failed", stats.Failed,
            )
        }
    }
}()
```

## 故障排除

### 常见问题

1. **任务不执行**
   - 检查调度器是否已启动
   - 确认任务状态为 `TaskStatusPending`
   - 检查工作协程数配置

2. **任务执行失败**
   - 检查任务处理函数的错误返回
   - 查看日志中的错误信息
   - 确认重试策略配置

3. **内存泄漏**
   - 检查任务清理策略
   - 确认已完成任务被正确清理
   - 监控任务总数

4. **性能问题**
   - 调整工作协程数
   - 优化任务处理逻辑
   - 使用合适的执行模式

### 调试技巧

```go
// 启用调试模式
app := bootstrap.NewApp(
    bootstrap.DebugMode(true),
    bootstrap.Tasks(),
)

// 查看详细任务信息
tasks, err := app.Scheduler.ListTasks()
if err == nil {
    for _, task := range tasks {
        app.Logger.Debug("任务详情",
            "id", task.ID,
            "type", task.Type,
            "status", task.Status,
            "scheduledAt", task.ScheduledAt,
            "createdAt", task.CreatedAt,
            "updatedAt", task.UpdatedAt,
        )
    }
}
```

## 总结

Sparrow 的任务系统集成提供了灵活且强大的功能：

- **多种初始化方式**：支持基础、混合、高级三种配置
- **直接API访问**：通过 `app.RunTask*` 系列方法快速提交任务
- **完整生命周期管理**：从创建、执行到清理的完整支持
- **企业级特性**：重试机制、统计信息、动态配置等

通过合理选择任务系统类型和配置选项，可以满足从简单到复杂的各种业务需求。