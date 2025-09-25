# Task 机制使用指南

## 概述

Task机制是MSDL框架中用于实现长时任务和定时任务的核心组件，提供了任务的创建、执行、状态管理和监控功能。本文档详细介绍如何在代码中使用Task机制。

## 目录结构

```
task/
├── internal/
│   ├── entity/
│   │   └── task.go.tmpl       # 任务实体定义
│   └── usecase/
│       └── tasks.go.tmpl      # 任务用例实现
└── adapter/
    └── http/
        └── gin/
            └── task.go.tmpl   # HTTP接口适配器
```

## 核心概念

- **长时任务**：执行时间较长的任务，通常需要在后台异步执行
- **定时任务**：按照预定时间执行的任务
- **任务状态**：pending（待处理）、running（运行中）、completed（已完成）、failed（失败）、cancelled（已取消）

## 任务实体结构

```go
// Task 任务实体结构
type Task struct {
    ID        string                 // 任务唯一标识
    Type      string                 // 任务类型
    Status    TaskStatus             // 任务状态
    Payload   map[string]interface{} // 任务负载数据
    Result    map[string]interface{} // 任务执行结果
    Error     string                 // 错误信息
    Priority  int                    // 任务优先级
    Retries   int                    // 已重试次数
    MaxRetry  int                    // 最大重试次数
    CreatedAt time.Time              // 创建时间
    UpdatedAt time.Time              // 更新时间
    StartedAt *time.Time             // 开始时间
    EndedAt   *time.Time             // 结束时间
}
```

## 使用方法

### 1. 初始化任务用例

```go
import (
    "{{ .ModuleName }}/internal/entity"
    "{{ .ModuleName }}/internal/usecase"
    "{{ .ModuleName }}/internal/adapter/repository"
)

// 创建仓储实例
repo := repository.NewTaskRepository()

// 创建任务服务
taskService := usecase.NewTaskService(repo)
```

### 2. 注册任务处理器

任务处理器是实际执行任务逻辑的函数，需要为每种任务类型注册对应的处理器：

```go
// 定义任务处理器
processor := func(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
    // 实现任务逻辑
    // 例如处理长时计算、文件处理等
    result := make(map[string]interface{})
    result["success"] = true
    return result, nil
}

// 注册处理器
taskService.RegisterHandler("my_task_type", processor)
```

### 3. 添加长时任务

```go
import "context"

// 创建上下文
ctx := context.Background()

// 准备任务数据
payload := map[string]interface{}{
    "param1": "value1",
    "param2": 123,
}

// 创建任务
// 任务会被标记为pending状态
newTask, err := taskService.CreateTask(ctx, "my_task_type", payload)
if err != nil {
    // 处理错误
}

// 异步执行任务
// 任务会在后台goroutine中执行
if err := taskService.ExecuteTaskAsync(ctx, newTask.ID); err != nil {
    // 处理错误
}

// 或者同步执行任务（会阻塞当前goroutine直到任务完成）
// err := taskService.ExecuteTask(ctx, newTask.ID)
```

### 4. 添加定时任务

Task机制本身不包含定时调度功能，需要与外部调度系统（如cron）配合使用：

```go
import (
    "github.com/robfig/cron/v3"
    "context"
)

// 创建cron调度器
c := cron.New()

// 添加定时任务
// 每天凌晨1点执行一次
_, err := c.AddFunc("0 1 * * *", func() {
    ctx := context.Background()
    payload := map[string]interface{}{"scheduled_time": time.Now()}
    
    // 创建并立即执行任务
    newTask, err := taskService.CreateTask(ctx, "daily_backup", payload)
    if err == nil {
        taskService.ExecuteTaskAsync(ctx, newTask.ID)
    }
})

// 启动调度器
c.Start()
```

## HTTP API 接口

Task机制提供了完整的HTTP API接口，可以通过HTTP请求管理和监控任务。

### 1. 创建任务

```
POST /tasks
```

**请求体：**
```json
{
  "type": "my_task_type",
  "payload": {"key": "value"}
}
```

**响应：**
```json
{
  "id": "task_123456789",
  "type": "my_task_type",
  "status": "pending",
  "created_at": "2023-01-01T12:00:00Z",
  "updated_at": "2023-01-01T12:00:00Z"
}
```

### 2. 获取任务状态

```
GET /tasks/:id
```

**响应：**
```json
{
  "id": "task_123456789",
  "type": "my_task_type",
  "status": "completed",
  "payload": {"key": "value"},
  "result": {"success": true},
  "created_at": "2023-01-01T12:00:00Z",
  "updated_at": "2023-01-01T12:05:00Z",
  "started_at": "2023-01-01T12:00:30Z",
  "ended_at": "2023-01-01T12:05:00Z"
}
```

### 3. 列出任务

```
GET /tasks
```

**可选参数：**
- `status`: 筛选特定状态的任务（pending、running、completed、failed、cancelled）

**响应：**
```json
[
  {
    "id": "task_123456789",
    "type": "my_task_type",
    "status": "completed",
    "created_at": "2023-01-01T12:00:00Z",
    "updated_at": "2023-01-01T12:05:00Z"
  },
  // 更多任务...
]
```

### 4. 执行任务

```
POST /tasks/:id/execute
```

**响应：**
```json
{
  "success": true,
  "message": "Task execution started"
}
```

### 5. 取消任务

```
DELETE /tasks/:id/cancel
```

**响应：**
```json
{
  "success": true,
  "message": "Task cancelled successfully"
}
```

### 6. 删除任务

```
DELETE /tasks/:id
```

**响应：**
```json
{
  "success": true,
  "message": "Task deleted successfully"
}
```

### 7. 获取任务统计

```
GET /tasks/stats
```

**响应：**
```json
{
  "pending": 5,
  "running": 2,
  "completed": 42,
  "failed": 3,
  "cancelled": 1
}
```

## 任务生命周期管理

### 任务状态流转

```
pending → running → completed
           ↑         ↓
           └───── failed ←┘
               ↑       │
               └── retries
```

### 重试机制

Task机制内置了任务重试功能，当任务执行失败时会自动重试：

```go
// 创建任务时设置最大重试次数
newTask := entity.NewTask("my_task_type", payload)
newTask.MaxRetry = 5 // 最多重试5次

// 保存任务
err := taskService.repo.Save(ctx, newTask)
```

### 任务清理

系统提供了过期任务清理功能，可以定期清理超过24小时的任务：

```go
// 清理过期任务
err := taskService.CleanupExpiredTasks(ctx)
```

## 最佳实践

1. **任务类型命名**：使用清晰、有描述性的任务类型名称，避免使用模糊的名称
2. **任务负载设计**：任务负载应包含执行任务所需的全部信息，保持数据完整性
3. **错误处理**：任务处理器应返回有意义的错误信息，便于调试
4. **并发控制**：注意任务执行时的并发控制，避免资源竞争
5. **监控告警**：为重要任务设置监控告警，及时发现任务执行异常
6. **超时控制**：为长时任务设置合理的超时机制

## 示例代码

### 1. 实现文件上传任务

```go
// 注册文件上传任务处理器
fileUploadHandler := func(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
    // 获取文件信息
    fileName, ok := payload["file_name"].(string)
    if !ok {
        return nil, errors.New("missing file_name")
    }
    
    fileContent, ok := payload["file_content"].([]byte)
    if !ok {
        return nil, errors.New("missing file_content")
    }
    
    // 模拟文件上传耗时操作
    time.Sleep(5 * time.Second)
    
    // 返回结果
    result := map[string]interface{}{
        "file_name": fileName,
        "size":      len(fileContent),
        "uploaded":  true,
    }
    return result, nil
}

taskService.RegisterHandler("file_upload", fileUploadHandler)

// 创建文件上传任务
payload := map[string]interface{}{
    "file_name":    "example.txt",
    "file_content": []byte("Hello, world!")
}

newTask, err := taskService.CreateTask(ctx, "file_upload", payload)
if err == nil {
    taskService.ExecuteTaskAsync(ctx, newTask.ID)
}
```

### 2. 实现数据同步任务

```go
// 注册数据同步任务处理器
dataSyncHandler := func(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
    // 获取同步参数
    source, ok := payload["source"].(string)
    if !ok {
        return nil, errors.New("missing source")
    }
    
    destination, ok := payload["destination"].(string)
    if !ok {
        return nil, errors.New("missing destination")
    }
    
    // 模拟数据同步耗时操作
    startTime := time.Now()
    // 实际项目中这里会实现真实的数据同步逻辑
    time.Sleep(10 * time.Second)
    
    // 返回结果
    result := map[string]interface{}{
        "source":      source,
        "destination": destination,
        "records":     42, // 假设同步了42条记录
        "duration_ms": time.Since(startTime).Milliseconds(),
    }
    return result, nil
}

taskService.RegisterHandler("data_sync", dataSyncHandler)
```