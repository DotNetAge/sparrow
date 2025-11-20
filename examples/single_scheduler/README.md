# 单一调度器使用指南

单一调度器虽然功能简单，但在实际业务场景中使用频率最高。相比复杂的混合调度器，单一调度器配置简单、性能高效，适合大多数任务调度需求。

## 🎯 适用场景

### 1. **微服务架构中的任务处理**
- 每个微服务独立管理自己的任务
- 避免复杂的跨服务任务协调
- 配置简单，易于维护

### 2. **定时任务和批处理**
- 数据清理和归档
- 报表生成
- 统计计算
- 邮件发送

### 3. **实时任务处理**
- 用户操作响应
- 消息推送
- 缓存更新
- 日志处理

### 4. **监控和健康检查**
- 服务健康检查
- 性能监控
- 异常检测
- 自动恢复

## 🚀 快速开始

### 基础用法

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/DotNetAge/sparrow/pkg/tasks"
)

func main() {
    // 创建调度器
    scheduler := tasks.NewMemoryTaskScheduler()
    defer scheduler.Close(context.Background())
    
    // 创建即时任务
    task := tasks.NewTaskBuilder().
        Immediate().
        Handler(func(ctx context.Context) error {
            fmt.Println("任务执行完成")
            return nil
        }).
        Build()
    
    // 调度任务
    if err := scheduler.Schedule(task); err != nil {
        fmt.Printf("调度失败: %v\n", err)
    }
    
    // 等待任务执行
    time.Sleep(100 * time.Millisecond)
}
```

## 📋 任务类型

### 1. 即时任务 (Immediate)
适用于需要立即执行的操作：

```go
task := tasks.NewTaskBuilder().
    Immediate().
    Handler(func(ctx context.Context) error {
        // 立即执行的业务逻辑
        return nil
    }).
    Build()
```

**使用场景：**
- 用户操作响应
- 数据验证
- 消息推送
- 缓存更新

### 2. 定时任务 (Scheduled)
使用 Cron 表达式指定执行时间：

```go
task := tasks.NewTaskBuilder().
    Scheduled("0 2 * * *"). // 每天凌晨2点
    Handler(func(ctx context.Context) error {
        // 定时执行的业务逻辑
        return nil
    }).
    Build()
```

**常用 Cron 表达式：**
- `0 2 * * *` - 每天凌晨2点
- `0 */6 * * *` - 每6小时
- `0 0 * * 0` - 每周日午夜
- `0 0 1 * *` - 每月1号午夜

**使用场景：**
- 数据备份
- 日志清理
- 报表生成
- 系统维护

### 3. 重复任务 (Recurring)
使用 Cron 表达式指定重复执行：

```go
task := tasks.NewTaskBuilder().
    Recurring("*/5 * * * *"). // 每5分钟
    Handler(func(ctx context.Context) error {
        // 重复执行的业务逻辑
        return nil
    }).
    Build()
```

**使用场景：**
- 健康检查
- 监控数据收集
- 缓存刷新
- 数据同步

## 🔧 高级功能

### 错误处理

```go
task := tasks.NewTaskBuilder().
    Immediate().
    Handler(func(ctx context.Context) error {
        // 可能失败的业务逻辑
        return fmt.Errorf("任务执行失败")
    }).
    OnFailure(func(ctx context.Context, err error) {
        fmt.Printf("任务失败: %v\n", err)
        // 错误处理逻辑
    }).
    Build()
```

### 任务取消

```go
task := tasks.NewTaskBuilder().
    Immediate().
    Handler(func(ctx context.Context) error {
        select {
        case <-time.After(10 * time.Second):
            fmt.Println("长时间任务完成")
            return nil
        case <-ctx.Done():
            fmt.Println("任务被取消")
            return ctx.Err()
        }
    }).
    OnCancel(func(ctx context.Context) {
        fmt.Println("任务取消回调")
    }).
    Build()

// 调度任务
scheduler.Schedule(task)

// 取消任务
time.Sleep(1 * time.Second)
scheduler.Cancel(task.ID())
```

### 并发控制

```go
// 限制最多同时执行3个任务
scheduler := tasks.NewMemoryTaskScheduler(
    tasks.WithMaxConcurrentTasks(3),
)
```

### 状态查询

```go
// 查询任务状态
status, err := scheduler.GetTaskStatus(task.ID())
if err == nil {
    fmt.Printf("任务状态: %s\n", status)
}

// 列出所有任务
tasks := scheduler.ListTasks()
for _, task := range tasks {
    fmt.Printf("任务ID: %s, 状态: %s\n", task.ID, task.Status)
}
```

## 📊 性能优化

### 1. 合理设置并发数

```go
// 根据业务特点调整并发数
// CPU密集型任务：并发数 = CPU核心数
// IO密集型任务：并发数 = CPU核心数 * 2-4
scheduler := tasks.NewMemoryTaskScheduler(
    tasks.WithMaxConcurrentTasks(8), // 8个并发任务
)
```

### 2. 任务执行时间控制

```go
task := tasks.NewTaskBuilder().
    Immediate().
    Handler(func(ctx context.Context) error {
        // 设置超时上下文
        ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
        defer cancel()
        
        // 在超时上下文中执行任务
        return doWork(ctx)
    }).
    Build()
```

### 3. 资源池化

```go
// 对于需要数据库连接的任务
var dbPool *sql.DB

task := tasks.NewTaskBuilder().
    Immediate().
    Handler(func(ctx context.Context) error {
        conn, err := dbPool.Conn(ctx)
        if err != nil {
            return err
        }
        defer conn.Close()
        
        // 使用连接执行任务
        return executeQuery(ctx, conn)
    }).
    Build()
```

## 🛡️ 最佳实践

### 1. 任务设计原则

- **幂等性**: 任务可以安全地重复执行
- **原子性**: 任务要么完全成功，要么完全失败
- **可观测性**: 提供足够的日志和监控信息
- **可取消性**: 支持长时间运行任务的取消

### 2. 错误处理策略

```go
task := tasks.NewTaskBuilder().
    Immediate().
    Handler(func(ctx context.Context) error {
        return doBusinessLogic(ctx)
    }).
    OnFailure(func(ctx context.Context, err error) {
        // 记录错误日志
        logger.Error("任务执行失败", "error", err)
        
        // 发送告警
        alert.SendAlert("任务失败", err.Error())
        
        // 重试逻辑（可选）
        if isRetryable(err) {
            scheduleRetry(ctx)
        }
    }).
    Build()
```

### 3. 资源管理

```go
func main() {
    scheduler := tasks.NewMemoryTaskScheduler()
    defer scheduler.Close(context.Background()) // 确保资源释放
    
    // 使用调度器...
}
```

### 4. 监控和日志

```go
task := tasks.NewTaskBuilder().
    Immediate().
    Handler(func(ctx context.Context) error {
        start := time.Now()
        defer func() {
            duration := time.Since(start)
            logger.Info("任务执行完成", "duration", duration)
        }()
        
        // 执行任务逻辑
        return doWork(ctx)
    }).
    Build()
```

## 🔄 与混合调度器的对比

| 特性 | 单一调度器 | 混合调度器 |
|------|------------|------------|
| 配置复杂度 | 简单 | 复杂 |
| 性能开销 | 低 | 高 |
| 内存占用 | 少 | 多 |
| 适用场景 | 大多数业务场景 | 复杂的多类型任务场景 |
| 学习成本 | 低 | 高 |
| 维护成本 | 低 | 高 |

## 📝 总结

单一调度器是大多数应用场景的最佳选择：

1. **简单高效**: 配置简单，性能优秀
2. **易于维护**: 代码清晰，调试方便
3. **功能完整**: 满足95%的业务需求
4. **资源友好**: 内存和CPU占用少
5. **扩展性好**: 可以轻松升级到混合调度器

只有在需要同时处理多种不同类型任务的复杂场景下，才需要考虑使用混合调度器。对于大多数微服务、Web应用和后台系统，单一调度器是更合适的选择。