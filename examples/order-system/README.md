# Sparrow 订单系统示例

这是一个基于 Sparrow构建的完整订单系统示例，展示了如何使用 DDD（领域驱动设计）、CQRS（命令查询职责分离）和事件溯源模式来构建现代化的微服务应用。

##架概览

本示例遵循整洁架构原则，分为以下层次：

```
├── cmd/                    #应用入口点
│   ├── demo.go            #简化演示版本
│   └── main.go            #完整实现版本
├── configs/               #配置文件
├── pkg/
│   ├── domain/            #层
│   │  └── entities/      #实体、聚合根、值对象
│   ├── usecases/          # 用例层
│   │   ├── commands/      #命定义
│   │   ├── queries/       # 查询定义
│   │   ├── repositories/   # 仓储接口
│   │  └── services/       #应用服务
│   ├── adapters/          # 适配器层
│   │   ├── handlers/      # HTTP处理器
│   │   └── routers/      #路配置
│   └── infrastructures/   #基础设施层
│       ├── repositories/  # 仓储实现
│       └── projections/   # CQRS投实现
```

## 核心特性

### 1.驱动设计 (DDD)
- **聚合根**: `Order`根管理订单的完整生命周期
- **值对象**: `Address`地值对象确保数据一致性
- **领域事件**:订单状态变更时自动产生领域事件
- **业务规则**:内置订单状态转换的业务规则验证

### 2. 事件溯源
- 使用 Sparrow 的事件存储机制记录所有状态变更
-支持从事件流重建聚合根状态
- 事件版本控制确保数据一致性

### 3. CQRS 模式
- **命令端**:处理订单创建、确认、取消、发货、收货等写操作
- **查询端**: 通过投影机制维护读模型，支持高效查询

### 4.微自治
- 使用内存数据库实现完全自治的服务架构
- 事件驱动的松耦合通信机制
-支持水平扩展和独立部署

##快开始

### 环境要求
- Go 1.25+
- Sparrow

###安依赖
```bash
cd examples/order-system
go mod tidy
```

###启动服务
```bash
#运行简化演示版
go run cmd/demo.go

#或者运行完整版（需要完整依赖）
go run cmd/main.go
```

服务将在 `http://localhost:8080`启。

### API测试

#### 1. 创建订单
```bash
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "customer-123",
    "items": [
      {
        "product_id": "product-001",
        "name": "iPhone 15",
        "price": 999.99,
        "quantity": 1
      }
    ],
    "shipping_address": {
      "street": "123 Main St",
      "city": "San Francisco",
      "state": "CA",
      "zip_code": "94105",
      "country": "USA"
    }
  }'
```

#### 2. 获取订单
```bash
curl http://localhost:8080/api/v1/orders/{order-id}
```

#### 3.列出订单
```bash
curl "http://localhost:8080/api/v1/orders?customer_id=customer-123"
```

#### 4. 确认订单
```bash
curl -X POST http://localhost:8080/api/v1/orders/{order-id}/confirm \
  -H "Content-Type: application/json" \
  -d '{"order_id": "{order-id}"}'
```

#### 5. 发货订单
```bash
curl -X POST http://localhost:8080/api/v1/orders/{order-id}/ship \
  -H "Content-Type: application/json" \
  -d '{"order_id": "{order-id}"}'
```

#### 6. 确认收货
```bash
curl -X POST http://localhost:8080/api/v1/orders/{order-id}/deliver \
  -H "Content-Type: application/json" \
  -d '{"order_id": "{order-id}"}'
```

#### 7.取订单
```bash
curl -X POST http://localhost:8080/api/v1/orders/{order-id}/cancel \
  -H "Content-Type: application/json" \
  -d '{"order_id": "{order-id}"}'
```

##模型

###订单状态流转
```
Pending → Confirmed → Shipped → Delivered
    ↓         ↓
 Cancelled  Cancelled
```

###事件
- `OrderCreated` - 订单创建
- `OrderConfirmed` - 订单确认
- `OrderCancelled` - 订单取消
- `OrderShipped` - 订单发货
- `OrderDelivered` - 订单送达

##技术栈

- **框架**: Sparrow DDD/CQRS
- **Web框架**: Gin
- **事件存储**:内存事件存储
- **事件总线**: 内存事件总线
- **仓储**:内存仓储
- **依赖注入**: Sparrow DI容

##扩指南

###切到持久化存储
要切换到 PostgreSQL化存储：

1. 修改 `cmd/main.go` 中的依赖注册：
```go
// 注册 PostgreSQL 仓储
app.Container.RegisterSingleton(func() repositories.OrderRepository {
    // 返回 PostgreSQL 实现
})
```

2. 实现 `OrderRepository` 接口的持久化版本

### 添加新的领域事件
1. 在 `domain/entities/events.go` 中定义新事件
2. 在聚合根中添加相应的业务方法
3. 在投影中处理新事件

### 添加新的查询
1. 在 `usecases/queries/` 中定义查询对象
2. 在应用服务中实现查询逻辑
3. 在 HTTP处理器中暴露查询接口

## 最佳实践

### 1.模型设计
- 保持聚合根的边界清晰
-保证数据一致性
-事件反映业务事实

### 2.错误处理
- 使用领域特定的错误类型
- 在应用服务层统一处理错误
- 提供有意义的错误信息

### 3.测试策略
-层单元测试
-应用服务层集成测试
- HTTP层端到端测试

##学资源

- [Sparrow文档](https://github.com/DotNetAge/sparrow)
- [领域驱动设计](https://domainlanguage.com/ddd/)
- [CQRS 和事件溯源](https://martinfowler.com/bliki/CQRS.html)

##许证

MIT License