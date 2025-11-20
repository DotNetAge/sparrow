# 单一调度器执行模式配置

Sparrow现在支持为单一调度器配置不同的执行模式，满足不同业务场景的需求。

## 配置选项

### Tasks

使用单一任务调度器，支持配置执行模式：

```go
app := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithConcurrentMode(),      // 并发模式（默认）
        bootstrap.WithWorkerCount(5),         // 工作协程数
        bootstrap.WithMaxConcurrentTasks(10), // 最大并发任务数
    ),
)
```

**向后兼容性：**
- `bootstrap.Tasks()` - 无参数调用，使用默认并发模式配置
- `bootstrap.Tasks(opts...)` - 带参数调用，使用指定配置

### 执行模式选项

#### 1. 并发执行模式 - WithConcurrentMode()

```go
app := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithConcurrentMode(),
    ),
)
```

**特点：**
- 多个任务同时执行
- 充分利用系统资源
- 适用于CPU密集型或IO密集型任务
- 总耗时约等于最长任务的耗时

**适用场景：**
- 独立的任务处理
- 并行数据处理
- 微服务调用

#### 2. 顺序执行模式 - WithSequentialMode()

```go
app := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithSequentialMode(),
    ),
)
```

**特点：**
- 任务严格按提交顺序执行
- 适用于有依赖关系的任务
- 总耗时等于所有任务耗时之和

**适用场景：**
- 需要严格顺序的业务流程
- 有依赖关系的任务链
- 数据库事务操作

#### 3. 流水线执行模式 - WithPipelineMode()

```go
app := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithPipelineMode(),
    ),
)
```

**特点：**
- 任务按阶段串行执行
- 适用于需要分阶段处理的任务
- 总耗时等于所有任务耗时之和

**适用场景：**
- 分阶段的数据处理
- 工作流处理
- 批处理任务

### 性能配置选项

#### 工作协程数 - WithWorkerCount()

```go
app := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithConcurrentMode(),      // 只有并发模式下，WorkerCount才生效
        bootstrap.WithWorkerCount(10),       // 设置10个工作协程
    ),
)
```

**重要说明：**
- **并发模式**：`WithWorkerCount()` 设置并发工作协程数
- **顺序模式**：固定使用1个工作协程，`WithWorkerCount()` 参数会被忽略
- **流水线模式**：固定使用1个工作协程，`WithWorkerCount()` 参数会被忽略

#### 最大并发任务数 - WithMaxConcurrentTasks()

```go
app := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithMaxConcurrentTasks(20), // 最大20个并发任务
    ),
)
```

## 使用示例

### 基础用法

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
    // 创建并发模式的单一调度器
    app := bootstrap.NewApp(
        bootstrap.Tasks(
            bootstrap.WithConcurrentMode(),
            bootstrap.WithWorkerCount(5),
        ),
    )
    
    // 提交任务
    taskId := app.RunTask(func(ctx context.Context) error {
        fmt.Println("任务执行中...")
        time.Sleep(1 * time.Second)
        fmt.Println("任务完成")
        return nil
    })
    
    fmt.Printf("任务ID: %s\n", taskId)
}
```

### 定时任务与执行模式

```go
app := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithSequentialMode(), // 顺序模式下的定时任务
    ),
)

// 创建定时任务
scheduledTime := time.Now().Add(2 * time.Second)
taskId := app.RunTaskAt(scheduledTime, func(ctx context.Context) error {
    fmt.Println("定时任务执行")
    return nil
})
```

### 重复任务与执行模式

```go
app := bootstrap.NewApp(
    bootstrap.Tasks(
        bootstrap.WithConcurrentMode(), // 并发模式下的重复任务
    ),
)

// 创建重复任务
taskId := app.RunTaskRecurring(1*time.Second, func(ctx context.Context) error {
    fmt.Println("重复任务执行")
    return nil
})
```

## 执行模式对比

| 模式 | 执行方式 | 工作协程数 | 总耗时 | 适用场景 |
|------|----------|------------|--------|----------|
| 并发 | 同时执行 | 可配置（默认5） | ≈最长任务耗时 | 独立任务、并行处理 |
| 顺序 | 逐个执行 | 固定1个 | 所有任务耗时之和 | 有依赖关系的任务 |
| 流水线 | 分阶段执行 | 固定1个 | 所有任务耗时之和 | 分阶段处理、工作流 |

## 最佳实践

1. **选择合适的执行模式**
   - 独立任务使用并发模式
   - 有依赖关系的任务使用顺序模式
   - 分阶段处理使用流水线模式

2. **合理配置工作协程数**
   - **并发模式**：CPU密集型任务建议协程数 = CPU核心数，IO密集型任务建议协程数 = CPU核心数 × 2
   - **顺序/流水线模式**：无需配置，系统自动使用1个工作协程

3. **设置合理的最大并发数**
   - 避免过多并发导致资源竞争
   - 根据系统资源情况调整

4. **避免配置歧义**
   - 顺序模式和流水线模式下不要设置 `WithWorkerCount() > 1`
   - 系统会自动调整并发出警告，但建议在代码中直接使用正确的配置

4. **监控任务执行**
   - 使用日志记录任务执行情况
   - 监控系统资源使用情况

## 完整示例

参考 `examples/single_scheduler_modes/main.go` 文件，该示例展示了：

1. 三种执行模式的对比
2. 定时任务与执行模式的结合
3. 重复任务与执行模式的结合
4. 不同配置选项的使用方法

运行示例：
```bash
go run ./examples/single_scheduler_modes
```