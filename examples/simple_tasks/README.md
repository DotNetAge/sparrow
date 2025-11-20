# Sparrow 简化API使用指南

## 概述

Sparrow 提供了两种API风格：
1. **简化API** - 适用于80%的常见场景，代码简洁
2. **TaskBuilder API** - 适用于复杂场景，功能完整

## 简化API

### 核心方法

App对象提供了三个简化的任务创建方法：

```go
// 即时任务
app.RunTask(handler func(ctx context.Context) error) string

// 定时任务 - 在指定时间执行
app.RunTaskAt(at time.Time, handler func(ctx context.Context) error) string

// 重复任务 - 按指定间隔重复执行
app.RunTaskRecurring(interval time.Duration, handler func(ctx context.Context) error) string
```

### 基本用法

```go
package main

import (
    "context"
    "time"
    "github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
    // 创建App并配置任务系统
    app := bootstrap.NewApp(
        bootstrap.AdvancedTasks(
            // 配置任务执行策略
            bootstrap.WithSequentialType("email", "report"),     // 顺序执行
            bootstrap.WithConcurrentType("image", "notification"), // 并发执行
            bootstrap.WithPipelineType("data_import"),           // 流水线执行
            
            // 性能配置
            bootstrap.WithConcurrentWorkers(10),
            bootstrap.WithMaxConcurrentTasks(50),
        ),
    )

    // 1. 即时任务
    taskId := app.RunTask(func(ctx context.Context) error {
        println("执行即时任务")
        return nil
    })

    // 2. 定时任务
    backupTime := time.Now().Add(24 * time.Hour) // 明天此时
    app.RunTaskAt(backupTime, func(ctx context.Context) error {
        println("执行定时备份")
        return nil
    })

    // 3. 重复任务
    app.RunTaskRecurring(10*time.Minute, func(ctx context.Context) error {
        println("执行健康检查")
        return nil
    })
}
```

## 实际业务场景

### 用户注册流程

```go
// 用户注册后的系列任务
welcomeEmailId := app.RunTask(func(ctx context.Context) error {
    fmt.Println("发送欢迎邮件给新用户")
    return nil
})

processAvatarId := app.RunTask(func(ctx context.Context) error {
    fmt.Println("处理用户上传的头像")
    return nil
})

notifyAdminId := app.RunTask(func(ctx context.Context) error {
    fmt.Println("通知管理员有新用户注册")
    return nil
})
```

### 定时任务

```go
// 每天凌晨3点数据备份
backupTime := time.Date(time.Now().Year(), time.Now().Month(), 
    time.Now().Day()+1, 3, 0, 0, 0, time.Local)
app.RunTaskAt(backupTime, func(ctx context.Context) error {
    // 执行数据备份逻辑
    return nil
})

// 每小时清理临时文件
app.RunTaskRecurring(time.Hour, func(ctx context.Context) error {
    // 清理临时文件
    return nil
})
```

## TaskBuilder API（复杂场景）

当需要更复杂的任务配置时，可以使用TaskBuilder：

```go
import "github.com/DotNetAge/sparrow/pkg/tasks"

taskId := tasks.NewTaskBuilder().
    WithID("custom-task-id").
    WithType("custom-type").
    WithHandler(func(ctx context.Context) error {
        // 任务逻辑
        return nil
    }).
    WithTimeout(30*time.Second).
    WithRetry(3).
    Immediate().
    Build()

app.Scheduler.Schedule(taskId)
```

### 适用场景

- 需要自定义任务ID
- 需要设置超时时间
- 需要配置重试策略
- 需要错误处理
- 需要任务取消
- 需要任务状态查询

## API对比

| 特性 | 简化API | TaskBuilder API |
|------|---------|-----------------|
| 代码量 | 少（1行） | 多（5-10行） |
| 自定义ID | ❌ 自动生成 | ✅ 可指定 |
| 超时设置 | ❌ 默认值 | ✅ 可配置 |
| 重试策略 | ❌ 默认值 | ✅ 可配置 |
| 错误处理 | ❌ 基础 | ✅ 完整 |
| 任务取消 | ❌ 不支持 | ✅ 支持 |
| 状态查询 | ❌ 不支持 | ✅ 支持 |
| 适用场景 | 80%常见任务 | 20%复杂任务 |

## 最佳实践

1. **优先使用简化API** - 对于大多数业务场景，简化API足够使用
2. **策略统一配置** - 通过App配置统一管理任务执行策略
3. **混合使用** - 简化API处理常规任务，TaskBuilder处理特殊需求
4. **类型安全** - 利用编译时类型检查避免运行时错误

## 示例文件

- `examples/simple_tasks/` - 简化API实际使用示例
- `examples/single_scheduler/` - 简化后的单一调度器示例
- `examples/simple_api/` - 配置选项示例
- `examples/advanced_tasks/` - 高级配置示例

## 总结

简化API遵循KISS原则，提供了：
- **极简语法** - 一行代码创建任务
- **类型安全** - 编译时检查
- **自动管理** - ID生成、生命周期管理
- **策略配置** - 统一的执行策略管理
- **代码简洁** - 相比TaskBuilder减少60%代码量

对于80%的业务场景，简化API是最佳选择。对于需要精细控制的复杂场景，TaskBuilder API提供了完整的功能支持。