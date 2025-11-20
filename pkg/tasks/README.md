# Sparrow 任务调度系统

## 概述

Sparrow 任务调度系统是一个灵活、高效的任务调度框架，支持多种执行模式和调度器类型。系统采用模块化设计，遵循 SOLID 原则，提供了丰富的配置选项和扩展能力。

## 核心特性

- **多种执行模式**：支持并发、顺序、流水线三种执行模式
- **混合调度器**：根据任务类型自动选择最优执行策略
- **任务生命周期管理**：完整的任务状态跟踪和控制
- **优雅关闭**：支持任务的优雅关闭和资源清理
- **统计监控**：提供详细的执行统计信息
- **高度可配置**：通过 Option 模式提供灵活的配置选项

## 1. 架构设计

### 1.1 核心接口

#### TaskScheduler 接口
```go
type TaskScheduler interface {
    usecase.GracefulClose
    Schedule(task Task) error
    Start(ctx context.Context) error
    Stop() error
    Cancel(taskID string) error
    GetTaskStatus(taskID string) (TaskStatus, error)
    ListTasks() []TaskInfo
    SetMaxConcurrentTasks(max int) error
    SetExecutionMode(mode ExecutionMode) error
    GetExecutionMode() ExecutionMode
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

### 1.2 执行模式

```go
type ExecutionMode int

const (
    ExecutionModeConcurrent ExecutionMode = iota  // 并发执行
    ExecutionModeSequential                       // 顺序执行
    ExecutionModePipeline                         // 流水线执行
)
```

### 1.3 任务状态

```go
type TaskStatus string

const (
    TaskStatusWaiting   TaskStatus = "waiting"    // 等待中
    TaskStatusRunning   TaskStatus = "running"    // 执行中
    TaskStatusCompleted TaskStatus = "completed"  // 已完成
    TaskStatusCancelled TaskStatus = "cancelled"  // 已取消
    TaskStatusFailed    TaskStatus = "failed"     // 执行失败
)
```

## 2. 调度器类型

### 2.1 MemoryTaskScheduler（单一调度器）

基于内存的任务调度器，支持三种执行模式：

**特性**：
- 支持并发、顺序、流水线三种执行模式
- 优先级队列调度
- 工作池管理
- 任务取消和状态管理

**适用场景**：
- 简单的任务调度需求
- 单一执行模式的应用
- 对性能要求较高的场景

**使用方式**：

```go
// 基础用法
app, err := bootstrap.NewApp(
    bootstrap.Tasks(), // 默认并发调度器
)

// 高级配置
app, err := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithWorkerCount(5),           // 工作协程数
        bootstrap.WithMaxConcurrentTasks(20),   // 最大并发任务数
    ),
)
```

### 2.2 HybridTaskScheduler（混合调度器）

智能任务调度器，根据任务类型自动选择执行策略：

**特性**：
- 根据任务类型自动选择执行模式
- 支持动态策略注册
- 提供执行统计信息
- 多调度器协调管理

**执行策略**：
```go
type TaskExecutionPolicy string

const (
    PolicyConcurrent TaskExecutionPolicy = "concurrent"  // 并发执行
    PolicySequential TaskExecutionPolicy = "sequential"  // 顺序执行
    PolicyPipeline   TaskExecutionPolicy = "pipeline"    // 流水线执行
)
```

**适用场景**：
- 复杂的任务调度需求
- 多种类型任务并存
- 需要精细控制执行策略
- 需要监控和统计的场景

## 3. 使用指南

### 3.1 使用方式对比

**推荐使用简洁方式**，符合Go语言哲学：

| 方式 | 推荐度 | 代码行数 | 适用场景 |
|------|--------|----------|----------|
| `app.RunTask()` | ⭐⭐⭐⭐⭐ | 1行 | 90%的日常使用场景 |
| `app.RunTypedTask()` | ⭐⭐⭐⭐⭐ | 1行 | 混合调度器中的类型化任务 |
| `tasks.NewTaskBuilder()` | ⭐⭐ | 5-10行 | 延迟执行、周期性任务等特殊需求 |

**为什么推荐简洁方式？**
- ✅ 符合Go的简洁哲学
- ✅ 减少样板代码
- ✅ 自动处理任务ID生成
- ✅ 代码更易读和维护

### 3.2 快速开始 - 最简洁的方式

**推荐使用 `app.RunTask` 方式**，这是最符合Go语言简洁哲学的用法：

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
    // 创建应用实例 - 默认并发调度器
    app, err := bootstrap.NewApp(bootstrap.Tasks())
    if err != nil {
        log.Fatalf("创建应用失败: %v", err)
    }
    
    // 启动应用
    if err := app.Start(); err != nil {
        log.Fatalf("启动应用失败: %v", err)
    }
    defer app.CleanUp()
    
    // 提交任务 - 最简洁的方式
    taskID := app.RunTask(func(ctx context.Context) error {
        log.Println("执行任务...")
        time.Sleep(2 * time.Second)
        log.Println("任务完成")
        return nil
    })
    
    log.Printf("任务已提交，ID: %s", taskID)
    
    // 等待任务完成
    time.Sleep(5 * time.Second)
}
```

### 3.3 单一调度器配置

```go
package main

import (
    "log"
    
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
    // 创建应用实例 - 带配置
    app, err := bootstrap.NewApp(
        bootstrap.Tasks(
            bootstrap.WithWorkerCount(10),          // 10个工作协程
            bootstrap.WithMaxConcurrentTasks(50),   // 最大50个并发任务
            bootstrap.WithSequentialMode(),         // 顺序执行模式
        ),
    )
    if err != nil {
        log.Fatalf("创建应用失败: %v", err)
    }
    
    // 启动应用
    if err := app.Start(); err != nil {
        log.Fatalf("启动应用失败: %v", err)
    }
    defer app.CleanUp()
    
    // 提交多个任务
    for i := 0; i < 5; i++ {
        taskID := app.RunTask(func(ctx context.Context) error {
            log.Printf("执行任务 %d", i+1)
            return nil
        })
        log.Printf("任务 %d 已提交，ID: %s", i+1, taskID)
    }
}
```

### 3.4 混合调度器配置

```go
package main

import (
    "log"
    
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
    // 创建混合调度器应用
    app, err := bootstrap.NewApp(
        bootstrap.AdvancedTasks(
            bootstrap.WithConcurrentWorkers(5),           // 并发调度器工作协程数
            bootstrap.EnableSequentialExecution(),        // 启用顺序执行模式
            bootstrap.EnablePipelineExecution(),          // 启用流水线执行模式
            bootstrap.WithMaxConcurrentTasks(20),          // 最大并发任务数
            // 配置任务类型策略
            bootstrap.WithSequentialType("email", "report"),
            bootstrap.WithConcurrentType("image", "notification"),
            bootstrap.WithPipelineType("data-processing"),
        ),
    )
    if err != nil {
        log.Fatalf("创建应用失败: %v", err)
    }
    
    // 启动应用
    if err := app.Start(); err != nil {
        log.Fatalf("启动应用失败: %v", err)
    }
    defer app.CleanUp()
    
    // 提交不同类型的任务 - 简洁方式
    emailTaskID := app.RunTypedTask("email", func(ctx context.Context) error {
        log.Println("发送邮件...")
        return nil
    })
    
    imageTaskID := app.RunTypedTask("image", func(ctx context.Context) error {
        log.Println("处理图片...")
        return nil
    })
    
    dataTaskID := app.RunTypedTask("data-processing", func(ctx context.Context) error {
        log.Println("处理数据...")
        return nil
    })
    
    log.Printf("邮件任务: %s", emailTaskID)
    log.Printf("图片任务: %s", imageTaskID)
    log.Printf("数据处理任务: %s", dataTaskID)
}
```

### 3.5 高级任务创建 - TaskBuilder（可选）

对于需要精细控制任务属性的场景，可以使用TaskBuilder：

```go
import "github.com/DotNetAge/sparrow/pkg/tasks"

// 即时执行任务
task := tasks.NewTaskBuilder().
    WithID("task-1").
    WithType("email").
    WithHandler(func(ctx context.Context) error {
        return sendEmail()
    }).
    WithImmediateExecution().
    Build()

// 延迟执行任务
task := tasks.NewTaskBuilder().
    WithID("task-2").
    WithType("report").
    WithHandler(func(ctx context.Context) error {
        return generateReport()
    }).
    WithDelayedExecution(time.Now().Add(5 * time.Minute)).
    Build()

// 周期性任务
task := tasks.NewTaskBuilder().
    WithID("task-3").
    WithType("cleanup").
    WithHandler(func(ctx context.Context) error {
        return cleanup()
    }).
    WithRecurringExecution(time.Hour).
    Build()

// 提交到调度器
if err := app.Scheduler.Schedule(task); err != nil {
    log.Printf("提交任务失败: %v", err)
}
```

**注意**：在大多数情况下，`app.RunTask` 和 `app.RunTypedTask` 已经足够使用，只有在需要延迟执行、周期性执行等特殊需求时才需要使用TaskBuilder。

## 4. 高级功能

### 4.1 任务取消

```go
// 提交任务并获取ID
taskID := app.RunTask(func(ctx context.Context) error {
    // 长时间运行的任务
    for i := 0; i < 10; i++ {
        select {
        case <-ctx.Done():
            log.Println("任务被取消")
            return ctx.Err()
        default:
            log.Printf("任务进度: %d/10", i+1)
            time.Sleep(1 * time.Second)
        }
    }
    return nil
})

// 取消任务
err := app.CancelTask(taskID)
if err != nil {
    log.Printf("取消任务失败: %v", err)
}
```

### 4.2 任务状态查询

```go
// 获取任务状态
status, err := app.GetTaskStatus(taskID)
if err != nil {
    log.Printf("获取任务状态失败: %v", err)
    return
}

switch status {
case tasks.TaskStatusWaiting:
    log.Println("任务等待中")
case tasks.TaskStatusRunning:
    log.Println("任务执行中")
case tasks.TaskStatusCompleted:
    log.Println("任务已完成")
case tasks.TaskStatusFailed:
    log.Println("任务执行失败")
case tasks.TaskStatusCancelled:
    log.Println("任务已取消")
}
```

### 4.3 任务列表

```go
// 列出所有任务
allTasks := app.ListTasks()
for _, taskInfo := range allTasks {
    log.Printf("任务ID: %s, 类型: %s, 状态: %s", 
        taskInfo.ID, taskInfo.Type, taskInfo.Status)
}
```

### 4.4 动态配置调整

```go
// 调整最大并发任务数
err := app.SetMaxConcurrentTasks(100)
if err != nil {
    log.Printf("调整并发数失败: %v", err)
}

// 对于单一调度器，可以调整执行模式
err := app.SetExecutionMode(tasks.ExecutionModeSequential)
if err != nil {
    log.Printf("设置执行模式失败: %v", err)
}
```

### 4.5 混合调度器统计信息

```go
// 获取混合调度器统计信息
if stats, ok := app.GetTaskStats(); ok {
    for policy, stat := range stats {
        log.Printf("策略 %s: 总任务=%d, 已完成=%d, 失败=%d", 
            policy, stat.TotalTasks, stat.CompletedTasks, stat.FailedTasks)
    }
}
```

## 5. 最佳实践

### 5.1 优先使用简洁API

**强烈推荐使用 `app.RunTask` 和 `app.RunTypedTask`**，这是最符合Go语言简洁哲学的方式：

```go
// ✅ 推荐 - 简洁明了
taskID := app.RunTask(func(ctx context.Context) error {
    return doSomething()
})

// ✅ 推荐 - 类型化任务
taskID := app.RunTypedTask("email", func(ctx context.Context) error {
    return sendEmail()
})

// ❌ 避免 - 除非有特殊需求
task := tasks.NewTaskBuilder().
    WithID("task-1").
    WithHandler(func(ctx context.Context) error {
        return doSomething()
    }).
    WithImmediateExecution().
    Build()

if err := app.Scheduler.Schedule(task); err != nil {
    log.Printf("提交任务失败: %v", err)
}
```

### 5.2 选择合适的调度器

- **单一调度器**：大多数应用的首选，配置简单
- **混合调度器**：需要不同执行模式的复杂应用

### 5.3 任务类型设计

- **合理分类**：根据任务特性选择合适的执行策略
- **命名规范**：使用清晰的任务类型名称
- **策略一致性**：相同类型的任务应使用相同的执行策略

### 5.4 错误处理

- **任务内部错误处理**：在任务处理函数中处理预期的错误
- **回调处理**：使用 OnComplete 回调处理任务完成后的逻辑
- **日志记录**：记录任务执行过程中的关键信息

### 5.5 资源管理

- **优雅关闭**：确保应用在关闭时正确清理资源
- **并发控制**：合理设置最大并发任务数，避免资源耗尽
- **内存管理**：及时清理已完成的任务信息

### 5.6 监控和调试

- **统计信息**：定期检查任务执行统计
- **日志记录**：记录任务调度和执行的关键事件
- **状态监控**：监控任务状态变化，及时发现问题

## 6. 配置选项参考

### 6.1 单一调度器选项

```go
type Option func(*Options)

// WithWorkerCount 设置工作协程数量
func WithWorkerCount(count int) Option

// WithMaxConcurrentTasks 设置最大并发任务数
func WithMaxConcurrentTasks(max int) Option

// WithLogger 设置日志记录器
func WithLogger(logger *logger.Logger) Option
```

### 6.2 混合调度器选项

```go
type AdvancedTaskOption func(*AdvancedTaskConfig)

// 工作协程配置
func WithConcurrentWorkers(count int) AdvancedTaskOption

// 执行模式启用配置
func EnableSequentialExecution() AdvancedTaskOption
func EnablePipelineExecution() AdvancedTaskOption

// 并发控制
func WithMaxConcurrentTasks(max int) AdvancedTaskOption

// 任务策略配置（推荐方式）
func WithSequentialType(taskTypes ...string) AdvancedTaskOption
func WithConcurrentType(taskTypes ...string) AdvancedTaskOption
func WithPipelineType(taskTypes ...string) AdvancedTaskOption
```

#### API 设计说明

**重要变更**：为了解决语义混淆问题，我们进行了以下改进：

1. **移除误导性配置**：
   - 删除了 `WithSequentialWorkers(count)` 和 `WithPipelineWorkers(count)`
   - 这些配置暗示可以设置多个工作协程，但顺序/流水线执行逻辑上应该是独占的

2. **新增语义清晰的配置**：
   - `EnableSequentialExecution()` - 启用顺序执行模式
   - `EnablePipelineExecution()` - 启用流水线执行模式
   - 这些配置明确表示启用某种执行模式，内部固定使用1个工作协程

3. **保持向后兼容性**：
   - 保留了 `WithConcurrentWorkers(count)` 用于并发调度器
   - 所有现有的任务策略配置API保持不变

**设计原则**：
- **语义清晰**：配置名称直接反映其功能
- **逻辑一致**：顺序/流水线执行强制单线程，避免并发冲突
- **简洁易用**：通过布尔开关控制模式启用，减少配置复杂度

## 7. 故障排除

### 7.1 常见问题

**Q: 任务提交失败**
A: 检查调度器是否已启动，任务参数是否正确

**Q: 任务不执行**
A: 检查任务调度时间，工作协程是否正常

**Q: 并发任务过多**
A: 调整 MaxConcurrentTasks 参数

**Q: 内存占用过高**
A: 及时清理已完成任务，调整任务队列大小

### 7.2 调试技巧

- 启用详细日志记录
- 使用统计信息监控系统状态
- 检查任务状态和错误信息
- 验证配置参数的正确性

## 8. 示例项目

完整示例代码位于：
- `examples/advanced_tasks/` - 高级任务调度示例
- `examples/simple_api/` - 简洁API使用示例
- `examples/hybrid_scheduler_demo.go` - 混合调度器演示

这些示例展示了不同场景下的最佳实践和常见用法。