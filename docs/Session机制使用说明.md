# Session 机制使用说明

## 概述

Session 机制提供了一个轻量级的会话管理功能，用于在分布式系统中跟踪用户会话状态、存储临时数据，并支持会话过期控制。该机制采用整洁架构设计，具有良好的可测试性和可扩展性。

## 目录结构

Session 机制的代码组织遵循整洁架构原则：

```
session/
├── adapter/               # 接口适配器层
│   └── http/gin/          # HTTP处理器实现
└── internal/              # 内部实现
    ├── entity/            # 实体层 - 核心业务规则
    └── usecase/           # 用例层 - 业务逻辑
        └── repo/          # 仓储接口定义
```

## 核心概念

### Session 实体

Session 实体表示一个用户会话，包含以下主要字段：

- `ID`: 会话唯一标识符
- `Data`: 会话存储的数据（键值对形式）
- `Expiry`: 会话过期时间
- 继承自 BaseEntity 的标准字段（CreatedAt、UpdatedAt、Version）

```go
// Session 会话实体结构
type Session struct {
    entity.BaseEntity            // 嵌入基础实体，继承标准字段和方法
    Data   map[string]interface{} `json:"data"`
    Expiry time.Time              `json:"expiry"`
}
```

### SessionService 接口

SessionService 提供了会话管理的核心业务逻辑：

- 创建会话
- 获取会话
- 保存会话数据
- 更新会话特定数据
- 获取会话特定数据
- 删除会话
- 清理过期会话

## 使用方法

### 1. 初始化 SessionService

首先，需要创建 SessionService 实例，注入必要的依赖：

```go
import (
    "context"
    "time"
    "{{ project_name }}/internal/entity"
    "{{ project_name }}/internal/usecase"
)

// 创建仓储实例（根据实际存储实现）
repo := NewSessionRepository() // 假设有一个具体的仓储实现

// 设置会话过期时间（例如：24小时）
// 这里使用1小时作为示例
sessionTTL := time.Hour * 1

// 创建SessionService
sessionService := usecase.NewSessionService(repo, sessionTTL)
```

### 2. 基本会话操作

#### 创建会话

```go
// 创建一个新会话，指定会话ID
ctx := context.Background()
id := "user-123"

session, err := sessionService.CreateSession(ctx, id)
if err != nil {
    // 处理错误
}
```

#### 获取会话

```go
// 获取会话（自动检查过期状态）
session, err := sessionService.GetSession(ctx, "user-123")
if err != nil {
    // 会话不存在或已过期
}
```

#### 存储数据到会话

```go
// 方式1：直接设置键值对
err := sessionService.UpdateSessionData(ctx, "user-123", "username", "john_doe")
err := sessionService.UpdateSessionData(ctx, "user-123", "role", "admin")

// 方式2：获取会话后批量设置
session, err := sessionService.GetSession(ctx, "user-123")
session.SetData("username", "john_doe")
session.SetData("role", "admin")
err = sessionService.SaveSession(ctx, session)
```

#### 从会话获取数据

```go
// 获取单个键的值
value, exists, err := sessionService.GetSessionData(ctx, "user-123", "username")
if exists {
    // 使用value
    username := value.(string)
}

// 或者获取整个会话数据
session, err := sessionService.GetSession(ctx, "user-123")
for key, val := range session.Data {
    // 遍历所有数据
}
```

#### 删除会话

```go
// 永久删除会话
err := sessionService.DeleteSession(ctx, "user-123")
```

### 3. 会话管理

#### 清理过期会话

定期运行以下方法清理过期会话，可在后台任务中执行：

```go
// 清理所有过期会话
err := sessionService.ClearExpiredSessions(ctx)
```

## HTTP API 接口

Session 机制提供了 RESTful API 接口，可通过 HTTP 请求管理会话：

| 接口 | 方法 | 路径 | 描述 |
|------|------|------|------|
| 创建会话 | POST | /sessions | 创建新的会话 |
| 获取会话 | GET | /sessions/:id | 获取指定ID的会话信息 |
| 更新会话数据 | PUT | /sessions/:id/data | 更新会话中的特定数据 |
| 获取会话数据 | GET | /sessions/:id/data/:key | 获取会话中指定键的数据 |
| 删除会话 | DELETE | /sessions/:id | 永久删除指定ID的会话 |

### HTTP API 使用示例

#### 创建会话

```bash
curl -X POST http://localhost:8080/sessions \
  -H "Content-Type: application/json" \
  -d '{"id": "user-123"}'
```

#### 获取会话

```bash
curl -X GET http://localhost:8080/sessions/user-123
```

#### 更新会话数据

```bash
curl -X PUT http://localhost:8080/sessions/user-123/data \
  -H "Content-Type: application/json" \
  -d '{"key": "username", "value": "john_doe"}'
```

## 会话生命周期

1. **创建**：调用 `CreateSession` 或通过 POST /sessions API 创建新会话
2. **使用**：通过各种方法读写会话数据
3. **自动过期**：当会话过期后，`GetSession` 操作会自动检测并删除过期会话
4. **手动删除**：可通过 `DeleteSession` 方法或 DELETE /sessions/:id API 手动删除会话
5. **批量清理**：可通过 `ClearExpiredSessions` 方法批量清理过期会话

## 最佳实践

1. **会话ID生成**：使用唯一、不可猜测的ID作为会话标识符，推荐使用UUID
2. **会话过期设置**：根据业务需求合理设置会话过期时间（TTL）
3. **数据序列化**：存储在会话中的复杂对象应确保可正确序列化和反序列化
4. **定期清理**：设置定时任务定期调用 `ClearExpiredSessions` 清理过期会话
5. **错误处理**：正确处理会话操作过程中的错误，特别是会话不存在或已过期的情况
6. **数据加密**：如果会话中存储敏感信息，建议对数据进行加密处理

## 示例：完整的会话使用流程

```go
import (
    "context"
    "time"
    "fmt"
    "{{ project_name }}/internal/entity"
    "{{ project_name }}/internal/usecase"
)

func main() {
    // 初始化上下文
    ctx := context.Background()
    
    // 创建仓储实例（假设实现）
    repo := NewSessionRepository()
    
    // 创建SessionService，设置1小时过期
    sessionService := usecase.NewSessionService(repo, time.Hour)
    
    // 创建会话
    sessionID := "user-456"
    session, err := sessionService.CreateSession(ctx, sessionID)
    if err != nil {
        fmt.Printf("创建会话失败: %v\n", err)
        return
    }
    fmt.Printf("创建会话成功: %s\n", session.ID)
    
    // 存储数据
    err = sessionService.UpdateSessionData(ctx, sessionID, "username", "jane_doe")
    err = sessionService.UpdateSessionData(ctx, sessionID, "login_count", 1)
    
    // 获取数据
    username, exists, _ := sessionService.GetSessionData(ctx, sessionID, "username")
    if exists {
        fmt.Printf("用户名: %s\n", username.(string))
    }
    
    // 增加登录计数
    loginCount, exists, _ := sessionService.GetSessionData(ctx, sessionID, "login_count")
    if exists {
        newCount := loginCount.(int) + 1
        sessionService.UpdateSessionData(ctx, sessionID, "login_count", newCount)
        fmt.Printf("更新后的登录计数: %d\n", newCount)
    }
    
    // 清理过期会话（通常在后台任务中执行）
    if err := sessionService.ClearExpiredSessions(ctx); err != nil {
        fmt.Printf("清理过期会话失败: %v\n", err)
    }
}
```

## 集成说明

Session 机制可与各种 Web 框架集成，默认提供了 Gin 框架的适配器。要在其他框架中使用，只需实现相应的 HTTP 处理器，调用 SessionService 提供的方法即可。

## 扩展建议

1. **分布式会话**：对于分布式系统，可实现基于 Redis 等分布式存储的 Repository
2. **会话集群**：支持在多实例部署环境中共享会话数据
3. **会话持久化**：实现将会话数据持久化到数据库的功能
4. **会话监控**：添加会话使用情况的监控和统计功能

通过以上说明，您应该能够快速了解和使用 Session 机制进行会话管理。如有任何问题或建议，请联系开发团队。