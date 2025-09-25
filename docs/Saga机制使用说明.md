# Saga 机制使用说明

## 概述

Saga 机制提供了一个分布式事务协调框架，用于处理跨服务边界的复杂事务操作。它实现了所谓的 "Saga 模式"，通过将长事务分解为一系列短事务（称为步骤）并为每个步骤定义补偿操作，确保系统在出现故障时能够回滚到一致状态。

## 目录结构

Saga 机制的代码组织遵循整洁架构原则：

```
saga/
├── adapter/              # 接口适配器层
│   └── saga/             # Saga协调器适配器
└── internal/             # 内部实现
    ├── entity/           # 实体层 - 核心业务数据结构
    └── usecase/          # 用例层 - 业务逻辑
```

## 核心概念

### 事务实体（Transaction）

Transaction 实体表示一个完整的 Saga 事务，包含以下主要字段：

- `ID`: 事务唯一标识符
- `Name`: 事务名称
- `Steps`: 事务步骤列表
- `Current`: 当前执行到的步骤索引
- `Status`: 事务状态
- `CreatedAt`: 创建时间
- `UpdatedAt`: 更新时间

```go
// Transaction 事务实体结构
type Transaction struct {
    ID      string    `json:"id"`
    Name    string    `json:"name"`
    Steps   []Step    `json:"steps"`
    Current int       `json:"current"`
    Status  string    `json:"status"`
    Created time.Time `json:"created_at"`
    Updated time.Time `json:"updated_at"`
}
```

### 步骤实体（Step）

Step 实体表示事务中的一个操作步骤，包含以下字段：

- `Name`: 步骤名称
- `Handler`: 处理器名称，用于标识执行该步骤的函数
- `Payload`: 步骤执行所需的参数数据
- `Compensate`: 补偿处理器名称，用于标识回滚该步骤的函数

```go
// Step 事务步骤结构
type Step struct {
    Name       string                 `json:"name"`
    Handler    string                 `json:"handler"`
    Payload    map[string]interface{} `json:"payload"`
    Compensate string                 `json:"compensate,omitempty"`
}
```

### 事务状态

Saga 事务支持以下状态：

- `StatusPending`: 事务已创建但尚未开始执行
- `StatusRunning`: 事务正在执行中
- `StatusCompleted`: 事务所有步骤执行成功
- `StatusFailed`: 事务执行失败
- `StatusCompensated`: 事务失败并已执行补偿操作

### SagaService 接口

SagaService 是 Saga 机制的核心服务，提供了事务管理的主要功能：

- 注册步骤处理器
- 注册补偿处理器
- 执行事务
- 获取事务状态

## 两种Saga实现模式

Saga模式主要有两种实现方式：Orchestration（编排式）和Choreography（编排式/协作式）。这两种模式各有优缺点，适用于不同的业务场景。

### Orchestration Saga（编排式Saga）

Orchestration Saga通过一个中心协调器（Coordinator）来控制整个事务流程。协调器了解所有事务步骤和它们之间的依赖关系，负责向各个服务发送命令并处理响应。

#### 特点

- 有一个中心协调器（如当前框架中的`SagaService`和`Coordinator`）
- 协调器控制整个事务的执行流程
- 服务之间不直接通信，只与协调器通信
- 事务逻辑集中在协调器中

#### 在当前框架中的实现

本框架实现的正是Orchestration Saga模式，主要体现在：

1. `SagaService`作为中心协调器，负责管理事务的整个生命周期
2. 所有步骤由协调器按顺序调用和控制
3. 当步骤失败时，协调器负责触发补偿流程
4. 事务状态由协调器集中维护

```go
// 本框架的实现本质上是Orchestration Saga
coordinator := saga.NewCoordinator() // 中心协调器
coordinator.Register(...)           // 注册处理器和补偿器
coordinator.Execute(...)            // 执行事务，由协调器控制整个流程
```

### Choreography Saga（编排式/协作式Saga）

Choreography Saga没有中心协调器，而是通过事件（Event）在服务之间进行通信。每个服务负责执行自己的事务步骤，并在完成后发布事件，其他服务监听这些事件并执行相应的操作。

#### 特点

- 没有中心协调器
- 服务之间通过事件进行松耦合通信
- 每个服务知道自己需要监听哪些事件以及如何响应
- 事务逻辑分布在各个服务中

#### 与服务自治原则的契合度

Choreography Saga更符合服务自治原则，因为：

1. 每个服务可以独立开发、部署和维护
2. 服务之间不需要了解整个事务流程
3. 服务只关注自己负责的业务领域
4. 系统更具弹性，单个服务的变更不会影响整个事务流程

#### 如何使用当前框架实现类似Choreography的效果

虽然本框架主要实现的是Orchestration模式，但您可以通过以下方式模拟Choreography的效果：

1. 将协调器逻辑简化，主要负责事件的路由和转发
2. 在处理器函数中发布领域事件
3. 使用消息队列作为事件总线，实现服务间的松耦合通信
4. 每个服务监听相关事件并执行自己的业务逻辑

```go
// 模拟Choreography效果的示例
coordinator.RegisterHandler("orderCreated", func(ctx context.Context, payload map[string]interface{}) error {
    // 1. 执行本地业务逻辑
    // 2. 发布领域事件
    eventBus.Publish("OrderCreated", payload)
    return nil
})
```

### 两种模式的比较

| 特性 | Orchestration Saga | Choreography Saga |
|------|-------------------|-------------------|
| 控制方式 | 集中式控制 | 分布式控制 |
| 复杂度 | 初期实现简单，复杂业务可能导致协调器臃肿 | 初期设计复杂，但更灵活 |
| 服务耦合度 | 服务与协调器强耦合 | 服务间松耦合 |
| 流程可见性 | 整个流程清晰可见 | 流程分散，难以追踪 |
| 变更影响 | 集中式变更，可能影响所有服务 | 变更影响范围小，只影响相关服务 |
| 适合场景 | 流程固定、相对简单的事务 | 流程复杂、需要高服务自治性的场景 |

### 为什么本框架采用Orchestration模式

本框架选择实现Orchestration Saga模式主要基于以下考虑：

1. **实现简单**：对于大多数业务场景，中心协调器模式更容易实现和理解
2. **流程可控**：集中式控制使得事务流程更加可控和可预测
3. **错误处理简单**：当事务失败时，协调器可以统一管理补偿流程
4. **调试和监控方便**：所有事务状态集中存储，便于调试和监控

然而，我们也认识到Choreography模式在服务自治方面的优势。如果您希望实现更符合服务自治原则的Saga，可以考虑结合消息队列和事件驱动架构，在当前框架基础上进行扩展。

## 使用方法

### 1. 初始化 SagaService

首先，需要创建 SagaService 实例，注入必要的依赖：

```go
import (
    "context"
    "{{ ModuleName }}/internal/entity"
    "{{ ModuleName }}/internal/usecase"
    "{{ ModuleName }}/internal/adapter/repository"
)

// 创建仓储实例（根据实际存储实现）
repo := repository.NewMemoryRepository[*entity.Transaction]()

// 创建SagaService
sagaService := usecase.NewSagaService(repo)
```

### 2. 注册处理器和补偿器

在执行事务前，需要先注册所有步骤的处理器和补偿器：

```go
// 注册账户扣款处理器
sagaService.RegisterHandler("deductMoney", func(ctx context.Context, payload map[string]interface{}) error {
    // 实现账户扣款逻辑
    accountID := payload["accountID"].(string)
    amount := payload["amount"].(float64)
    // 执行扣款操作
    return nil
})

// 注册账户扣款补偿器
sagaService.RegisterCompensator("refundMoney", func(ctx context.Context, payload map[string]interface{}) error {
    // 实现账户退款逻辑（与扣款相反的操作）
    accountID := payload["accountID"].(string)
    amount := payload["amount"].(float64)
    // 执行退款操作
    return nil
})

// 注册订单创建处理器
sagaService.RegisterHandler("createOrder", func(ctx context.Context, payload map[string]interface{}) error {
    // 实现订单创建逻辑
    return nil
})
```

### 3. 构建并执行事务

定义事务步骤并执行事务：

```go
// 构建事务步骤
steps := []entity.Step{
    {
        Name:    "扣款",
        Handler: "deductMoney",
        Payload: map[string]interface{}{
            "accountID": "user-123",
            "amount":    100.0,
        },
        Compensate: "refundMoney", // 扣款失败时执行退款补偿
    },
    {
        Name:    "创建订单",
        Handler: "createOrder",
        Payload: map[string]interface{}{
            "productID": "prod-456",
            "quantity":  2,
        },
        // 该步骤没有补偿器
    },
}

// 执行事务
ctx := context.Background()
transaction, err := sagaService.ExecuteTransaction(ctx, "购买商品", steps)
if err != nil {
    // 事务执行失败（可能已自动补偿）
    fmt.Printf("事务执行失败: %v, 状态: %s\n", err, transaction.Status)
} else {
    // 事务执行成功
    fmt.Printf("事务执行成功: %s\n", transaction.ID)
}
```

### 4. 查询事务状态

可以随时查询事务的执行状态：

```go
// 查询事务状态
tx, err := sagaService.GetTransaction(ctx, transaction.ID)
if err != nil {
    // 处理错误
} else {
    fmt.Printf("事务状态: %s, 当前步骤: %d/%d\n", 
        tx.Status, tx.Current, len(tx.Steps))
}
```

## 使用协调器简化操作

为了方便使用，Saga 机制提供了一个简化的协调器（Coordinator）包装：

```go
import (
    "context"
    "{{ ModuleName }}/internal/adapter/saga"
)

// 创建协调器（使用默认内存存储）
coordinator := saga.NewCoordinator()

// 注册处理器和补偿器
coordinator.Register("deductMoney", 
    // 处理器函数
    func(ctx context.Context, payload map[string]interface{}) error {
        // 实现扣款逻辑
        return nil
    },
    // 补偿器函数
    func(ctx context.Context, payload map[string]interface{}) error {
        // 实现退款逻辑
        return nil
    },
)

// 构建步骤
steps := []saga.Step{
    {
        Name:    "扣款",
        Handler: "deductMoney",
        Payload: map[string]interface{}{
            "accountID": "user-123",
            "amount":    100.0,
        },
        Compensate: "deductMoney", // 补偿器与处理器同名
    },
}

// 执行事务
ctx := context.Background()
tx, err := coordinator.Execute(ctx, "购买商品", steps)
```

## Saga 事务的执行流程

1. **创建事务**：构建事务对象，包含所有需要执行的步骤
2. **保存初始状态**：将会话状态持久化存储
3. **执行步骤**：按顺序执行每个步骤的处理器函数
4. **步骤成功**：更新事务状态，继续执行下一个步骤
5. **步骤失败**：触发补偿流程，按相反顺序执行已完成步骤的补偿器
6. **事务完成**：所有步骤执行成功，事务标记为完成状态
7. **事务补偿**：所有已完成步骤的补偿器执行完成，事务标记为已补偿状态

## 最佳实践

1. **设计幂等操作**：确保处理器和补偿器是幂等的，即重复执行不会产生副作用
2. **提供足够上下文**：在payload中包含足够的信息，以便补偿器能够正确回滚操作
3. **日志记录**：为每个步骤的执行和补偿操作添加详细日志，便于问题排查
4. **超时处理**：实现合理的超时机制，避免长时间阻塞
5. **错误处理**：在处理器中明确区分可重试错误和致命错误
6. **持久化存储**：在生产环境中，使用可靠的持久化存储替代内存存储
7. **监控和告警**：为Saga事务添加监控指标和告警机制，及时发现异常
8. **补偿策略**：根据业务需求设计合适的补偿策略，不是所有操作都需要补偿

## 示例：完整的订单支付流程

以下是一个完整的订单支付流程示例，展示了Saga机制在实际业务场景中的应用：

```go
import (
    "context"
    "fmt"
    "errors"
    "{{ ModuleName }}/internal/adapter/saga"
)

func main() {
    // 创建协调器
    coordinator := saga.NewCoordinator()
    
    // 注册账户扣款处理器和补偿器
    coordinator.Register("deductMoney", 
        func(ctx context.Context, payload map[string]interface{}) error {
            accountID := payload["accountID"].(string)
            amount := payload["amount"].(float64)
            fmt.Printf("从账户 %s 扣款 %.2f\n", accountID, amount)
            // 模拟扣款操作
            // 这里可以调用账户服务的API
            return nil
        },
        func(ctx context.Context, payload map[string]interface{}) error {
            accountID := payload["accountID"].(string)
            amount := payload["amount"].(float64)
            fmt.Printf("向账户 %s 退款 %.2f\n", accountID, amount)
            // 模拟退款操作
            return nil
        },
    )
    
    // 注册订单创建处理器
    coordinator.Register("createOrder", 
        func(ctx context.Context, payload map[string]interface{}) error {
            productID := payload["productID"].(string)
            quantity := payload["quantity"].(int)
            fmt.Printf("创建订单: 商品 %s, 数量 %d\n", productID, quantity)
            // 模拟订单创建
            // 这里可以调用订单服务的API
            // 模拟随机失败，展示补偿机制
            if quantity > 10 { // 假设库存不足
                return errors.New("库存不足")
            }
            return nil
        },
        nil, // 没有补偿器
    )
    
    // 注册库存扣减处理器和补偿器
    coordinator.Register("reserveInventory", 
        func(ctx context.Context, payload map[string]interface{}) error {
            productID := payload["productID"].(string)
            quantity := payload["quantity"].(int)
            fmt.Printf("扣减库存: 商品 %s, 数量 %d\n", productID, quantity)
            // 模拟库存扣减
            return nil
        },
        func(ctx context.Context, payload map[string]interface{}) error {
            productID := payload["productID"].(string)
            quantity := payload["quantity"].(int)
            fmt.Printf("恢复库存: 商品 %s, 数量 %d\n", productID, quantity)
            // 模拟库存恢复
            return nil
        },
    )
    
    // 构建购买流程步骤
    steps := []saga.Step{
        {
            Name:    "账户扣款",
            Handler: "deductMoney",
            Payload: map[string]interface{}{
                "accountID": "user-123",
                "amount":    299.99,
            },
            Compensate: "deductMoney", // 使用同名补偿器
        },
        {
            Name:    "扣减库存",
            Handler: "reserveInventory",
            Payload: map[string]interface{}{
                "productID": "prod-456",
                "quantity":  2,
            },
            Compensate: "reserveInventory",
        },
        {
            Name:    "创建订单",
            Handler: "createOrder",
            Payload: map[string]interface{}{
                "productID": "prod-456",
                "quantity":  2, // 尝试改为11来测试失败场景
            },
            // 无补偿器，订单创建失败依赖前面步骤的补偿
        },
    }
    
    // 执行事务
    ctx := context.Background()
tx, err := coordinator.Execute(ctx, "用户购买商品", steps)
    
    if err != nil {
        fmt.Printf("事务执行失败: %v\n", err)
        fmt.Printf("事务状态: %s\n", tx.Status)
    } else {
        fmt.Printf("事务执行成功! 交易ID: %s\n", tx.ID)
    }
}
```

## 集成说明

Saga 机制设计为可与各种服务架构集成：

1. **与微服务集成**：可作为微服务间事务协调的基础组件
2. **与消息队列集成**：可结合消息队列实现异步事务处理
3. **与持久化存储集成**：通过实现自定义Repository接口，支持各种存储后端
4. **与监控系统集成**：可添加指标收集，监控事务执行情况

## 扩展建议

1. **持久化改进**：实现基于数据库或消息队列的可靠存储
2. **超时重试**：添加自动重试机制，处理临时性网络故障
3. **并行执行**：支持部分无依赖步骤的并行执行，提高效率
4. **可视化监控**：开发事务执行状态的可视化监控界面
5. **事务编排**：支持更复杂的事务编排模式，如分支、条件判断等

通过以上说明，您应该能够快速了解和使用Saga机制进行分布式事务管理。如有任何问题或建议，请联系开发团队。