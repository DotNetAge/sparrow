# Sparrow 订单系统示例 - 快速入门指南

## 概述

这是一个使用 Sparrow 框架构建的完整订单管理系统，展示了如何运用 DDD、CQRS 和事件溯源等现代架构模式。

## 项目结构

```
examples/order-system/
├── cmd/
│   ├── demo.go           # 简化演示版（推荐入门）
│   └── main.go           # 完整实现版
├── pkg/
│   ├── domain/           # 领域层
│   │   └── entities/
│   │       ├── order.go        # 订单聚合根
│   │       ├── address.go      # 地址值对象
│   │       ├── events.go       # 领域事件定义
│   │       └── errors.go       # 领域错误
│   ├── usecases/         # 用例层
│   │   ├── commands/     # 命令定义
│   │   ├── queries/      # 查询定义
│   │   ├── repositories/ # 仓储接口
│   │   └── services/     # 应用服务
│   ├── adapters/         # 适配器层
│   │   ├── handlers/     # HTTP处理器
│   │   └── routers/      # 路由配置
│   └── infrastructures/  # 基础设施层
│       ├── repositories/ # 仓储实现
│       └── projections/  # CQRS 投影
└── configs/              # 配置文件
```

## 核心概念

### 1. 领域驱动设计 (DDD)

**聚合根 - Order**
```go
type Order struct {
    entity.BaseAggregateRoot
    CustomerID      string
    Items          []OrderItem
    TotalPrice      float64
    Status          OrderStatus
    ShippingAddress Address
}
```

**值对象 - Address**
```go
type Address struct {
    Street  string
    City    string
    State   string
    ZipCode string
    Country string
}
```

**领域事件**
- `OrderCreated` - 订单创建
- `OrderConfirmed` - 订单确认
- `OrderCancelled` - 订单取消
- `OrderShipped` - 订单发货
- `OrderDelivered` - 订单送达

### 2. 事件溯源

每个状态变更都被记录为不可变的事件：

```go
// 创建订单时产生事件
order := entities.NewOrder(customerID, items, address)
// 自动产生 OrderCreated 事件并添加到未提交事件列表
events := order.GetUncommittedEvents()
```

### 3. CQRS 模式

**命令端（写操作）**
```go
// 创建订单
POST /api/v1/orders

// 确认订单
POST /api/v1/orders/{id}/confirm
```

**查询端（读操作）**
```go
// 获取单个订单
GET /api/v1/orders/{id}

// 列出所有订单
GET /api/v1/orders
```

## 快速开始

### 步骤 1: 安装依赖

```bash
cd examples/order-system
go mod tidy
```

### 步骤 2: 运行演示

```bash
# 运行简化版本（推荐新手）
go run cmd/demo.go
```

### 步骤 3: 测试 API

**创建订单**
```bash
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "cust-001",
    "items": [
      {
        "product_id": "prod-001",
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

**响应示例**
```json
{
  "order_id": "uuid-here",
  "message": "Order created successfully"
}
```

**查看订单**
```bash
curl http://localhost:8080/api/v1/orders/{order-id}
```

**确认订单**
```bash
curl -X POST http://localhost:8080/api/v1/orders/{order-id}/confirm \
  -H "Content-Type: application/json" \
  -d '{"order_id": "{order-id}"}'
```

## 订单生命周期

```
[Pending] → [Confirmed] → [Shipped] → [Delivered]
    ↓            ↓
[Cancelled]  [Cancelled]
```

## 代码示例

### 创建订单的完整流程

```go
// 1. 创建地址值对象
address := entities.Address{
    Street:  "123 Main St",
    City:    "San Francisco",
    State:   "CA",
    ZipCode: "94105",
    Country: "USA",
}

// 2. 创建订单项
items := []entities.OrderItem{
    {
        ProductID: "prod-001",
        Name:      "iPhone 15",
        Price:     999.99,
        Quantity:  1,
    },
}

// 3. 创建订单聚合根（自动产生 OrderCreated 事件）
order := entities.NewOrder("customer-123", items, address)

// 4. 保存事件到事件存储
eventStore.SaveEvents(ctx, order.GetID(), order.GetUncommittedEvents(), 0)

// 5. 标记事件为已提交
order.MarkEventsAsCommitted()
```

### 确认订单

```go
// 1. 从存储加载订单
order := &entities.Order{}
eventStore.Load(ctx, orderID, order)

// 2. 执行确认操作（产生 OrderConfirmed 事件）
order.Confirm()

// 3. 保存新事件
eventStore.SaveEvents(ctx, order.GetID(), order.GetUncommittedEvents(), order.GetVersion()-1)
```

## 进阶主题

### 切换到真实数据库

当前示例使用内存存储，要切换到 PostgreSQL：

1. 实现 `OrderRepository` 接口
2. 在依赖注入容器中注册新实现
3. 修改配置文件中的数据库连接

### 添加新的业务功能

1. **添加新的领域事件**: 在 `domain/entities/events.go` 中定义
2. **添加新的命令**: 在 `usecases/commands/` 中定义
3. **添加新的查询**: 在 `usecases/queries/` 中定义
4. **更新应用服务**: 在 `usecases/services/` 中实现业务逻辑
5. **暴露 HTTP 接口**: 在 `adapters/handlers/` 中添加处理器

## 学习资源

- [Sparrow 官方文档](https://github.com/DotNetAge/sparrow)
- [领域驱动设计基础](https://martinfowler.com/bliki/DomainDrivenDesign.html)
- [事件溯源模式](https://martinfowler.com/eaaDev/EventSourcing.html)
- [CQRS 模式](https://martinfowler.com/bliki/CQRS.html)

## 常见问题

**Q: 为什么使用内存存储而不是真实数据库？**
A: 为了降低学习门槛，让开发者先理解核心概念，再自行扩展到持久化存储。

**Q: 如何将这个示例应用到实际项目？**
A: 参考本示例的架构分层，将内存仓储替换为真实的数据库实现，并根据业务需求调整领域模型。

**Q: 事件溯源相比传统 CRUD 有什么优势？**
A: 
- 完整的审计日志
- 支持时间旅行调试
- 更容易实现 CQRS
- 更好的扩展性

## 下一步

1. 尝试运行示例代码
2. 阅读源码理解架构
3. 根据业务需求修改领域模型
4. 实现持久化仓储
5. 集成到实际项目中

祝你学习愉快！🚀