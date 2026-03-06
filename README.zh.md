<div align="center">

# 🐦 Sparrow

![Sparrow](logo.png)
**一个为现代微服务而生的 Go 领域驱动框架**

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/badge/Build-Passing-brightgreen?style=flat-square)]()

*事件驱动 · 领域建模 · 多存储 · 服务完全自治 · 生产就绪*

</div>

<div align="center">


[English](./README.md) | [中文](./README.zh.md)

</div>

## 特点

### 使用侧

- **零样板代码** — 聚合根、仓储、事件发布三位一体，继承即用，无需重复实现基础设施
- **接口即契约** — 仓储、事件总线、事件存储全部面向接口编程，Mock 测试毫不费力
- **渐进式接入** — 从内存实现起步，生产时只需替换依赖注入，业务代码零改动
- **统一错误语义** — 全局错误包装规范，跨层传递不丢失上下文，可观测性开箱即用
- **配置驱动启动** — Viper 多来源配置（文件 / 环境变量 / 远程），一套 `App` 生命周期托管所有服务

### 技术侧

- **整洁架构分层** — 严格依赖倒置，领域层对基础设施零感知，架构腐化从根源阻断
- **泛型仓储 DSL** — Go 泛型加持，类型安全的 CRUD / 分页 / 软删除，多数据库一套接口
- **事件总线多后端** — Memory / NATS JetStream / RabbitMQ / Redis，接口统一，运行时按需切换
- **NATS JetStream 原生集成** — 持久化事件流、消费者组、重放能力，天然支持 Event Sourcing
- **Saga 分布式事务** — 编排式 Saga 协调器，补偿事务链路清晰，跨服务一致性有保障
- **Testcontainers 集成测试** — PostgreSQL、Redis 真实容器测试，告别 Mock 数据库的假阳性

### 微服务自治

> **一服务一数据库**，不再是奢望

微服务架构的终极目标是**完全自治**——每个服务拥有独立的数据存储，彼此无共享、无耦合。Sparrow 内置两张王牌让这一目标触手可及：

|                         | NATS                     | BadgerDB                  |
| ----------------------- | ------------------------ | ------------------------- |
| **定位**                | 轻量高性能事件流         | 嵌入式亿级 KV 数据库      |
| **二进制体积**          | < 20 MB                  | 无独立进程，随服务启动    |
| **吞吐量**              | 数百万消息/秒            | 数百万 ops/秒（SSD）      |
| **部署依赖**            | 单一可执行文件，无需运维 | 零外部依赖，数据随服务走  |
| **在 Sparrow 中的角色** | 服务间事件通信与事件溯源 | 服务私有读模型 / 状态存储 |

```
┌──────────────┐     NATS JetStream     ┌──────────────┐
│  订单服务     │ ──── OrderPlaced ────► │  库存服务     │
│  BadgerDB    │                        │  BadgerDB    │
│  (私有存储)   │ ◄─── StockReserved ─── │  (私有存储)   │
└──────────────┘                        └──────────────┘
     ↑ 无共享数据库，服务之间只通过事件通信，完全解耦自治
```

这意味着：每个微服务可以**独立部署、独立扩缩容、独立升级**，其他服务无感知。

---

## 为什么选择 Sparrow？

传统的微服务开发往往陷入三种困境：

- **业务与数据强耦合** — REST 直连数据库，审计与回放无从谈起
- **跨服务事务噩梦** — 同步调用链路，一致性问题无解
- **基础设施重复造轮子** — 仓储层、错误处理、事件承载各自为政

Sparrow 用 DDD + CQRS + Event Sourcing 的完整工具链，将这些问题一次性解决。

---

## 核心能力

### 事件驱动架构

统一的事件总线抽象，支持多种后端，按需切换，无需改动业务代码：

```go
// 订阅领域事件
bus.Sub("OrderPlaced", func(ctx context.Context, evt eventbus.Event) error {
    // 处理订单创建事件
    return nil
})

// 发布事件
bus.Pub(ctx, eventbus.Event{EventType: "OrderPlaced", Payload: order})
```

| 后端           | 适用场景                 |
| -------------- | ------------------------ |
| Memory         | 单元测试、本地开发       |
| NATS JetStream | 高吞吐、事件持久化与重放 |
| RabbitMQ       | 企业消息路由、复杂拓扑   |
| Redis Pub/Sub  | 轻量实时通知             |

### 领域驱动设计 (DDD)

内置聚合根模板，事件溯源开箱即用：

```go
type Order struct {
    entity.BaseAggregateRoot
    Status string
}

func (o *Order) PlaceOrder(items []Item) {
    // 产生领域事件，自动追踪版本
    o.AddDomainEvent(&OrderPlaced{Items: items})
    o.Status = "placed"
}
```

- 聚合根版本控制 & 乐观锁
- 快照机制加速大版本聚合重建
- 命令模式与用例执行器

### 泛型仓储 — 换存储不换代码

```go
// 声明接口，实现自动注入
type OrderRepo interface {
    usecase.Repository[Order]
}

// 从内存切换到 PostgreSQL，只需换一行依赖注入
```

| 实现       | 特性                         |
| ---------- | ---------------------------- |
| Memory     | 零依赖，测试利器             |
| PostgreSQL | 完整关系型支持，软删除，分页 |
| MongoDB    | 文档型，半结构化数据         |
| Redis      | 高性能缓存层，TTL 过期       |
| BadgerDB   | 嵌入式 KV，无需额外部署      |
| GORM (SQL) | 多数据库兼容，复杂关系模型   |

### 任务调度系统

三种调度策略灵活组合：

```go
// 并发调度 — 多任务同时跑
scheduler := tasks.NewConcurrentScheduler()

// 顺序调度 — 严格保序
scheduler := tasks.NewSequentialScheduler()

// 混合调度 — 批次内并发，批次间有序
scheduler := tasks.NewHybridScheduler()
```

### CQRS 投影系统

基于 NATS JetStream 的全量 & 增量投影，自动将事件流重建为读模型：

```
事件流 → 聚合根重建 → 投影计算 → 视图存储
```

### Saga 协调器

跨服务分布式事务的编排中枢，补偿事务一键回滚。

---

## 架构总览

```
┌─────────────────────────────────────────────────────┐
│                   HTTP / WebSocket                   │
│          Gin Router · JWT · Casbin RBAC              │
├─────────────────────────────────────────────────────┤
│                    应用用例层                         │
│        Command Handler · Query · Saga · Executor     │
├──────────────────────┬──────────────────────────────┤
│       领域模型        │         事件驱动              │
│  Entity · AggRoot    │  EventBus · EventStore        │
│  DomainEvent · Cmd   │  Publisher · Subscriber       │
├──────────────────────┴──────────────────────────────┤
│                    基础设施层                         │
│   PostgreSQL · MongoDB · Redis · BadgerDB · GORM     │
│   NATS JetStream · RabbitMQ · Zap · Viper            │
└─────────────────────────────────────────────────────┘
```

---

## 快速开始

**环境要求**: Go 1.25+

```bash
# 克隆项目
git clone https://github.com/DotNetAge/sparrow.git
cd sparrow/src

# 安装依赖
make deps

# 运行测试
make test

# 启动服务
make run
```

**使用 Docker**:

```bash
# 构建镜像
make docker-build

# 启动全套服务（含依赖）
make docker-run
```

---

## 技术栈

| 领域       | 技术                                    |
| ---------- | --------------------------------------- |
| 语言       | Go 1.25                                 |
| Web 框架   | Gin v1.11                               |
| 权限控制   | Casbin v2 (RBAC)                        |
| 认证       | JWT (golang-jwt/jwt v5) + RSA           |
| ORM        | GORM v1.31 · sqlx v1.4                  |
| 数据库     | PostgreSQL · MongoDB · SQLite           |
| 缓存       | Redis (go-redis v8/v9)                  |
| 嵌入式存储 | BadgerDB v4                             |
| 消息队列   | NATS JetStream v1.46 · RabbitMQ amqp091 |
| 日志       | Zap v1.27 + Lumberjack 滚动日志         |
| 配置管理   | Viper v1.21                             |
| 实时通信   | Gorilla WebSocket v1.5                  |
| 测试       | Testify · Testcontainers                |

---

## 项目结构

```
pkg/
├── entity/          # 领域模型：聚合根、实体、领域事件、命令
├── usecase/         # 用例层：仓储接口、事件存储接口、执行器
├── eventbus/        # 事件总线：Memory / NATS / RabbitMQ / Redis
├── messaging/       # 消息子系统：JetStream 发布订阅、事件序列化
├── persistence/     # 仓储实现：多数据源适配
│   └── repo/        # PostgreSQL · MongoDB · Redis · BadgerDB · GORM · Memory
├── projection/      # 投影系统：全量投影、JetStream 索引器
├── tasks/           # 任务调度：并发 / 顺序 / 混合调度器
├── auth/            # 认证授权：JWT 签发、RSA 验证
├── bootstrap/       # 应用启动：DI 容器、生命周期管理
├── config/          # 配置管理：多格式、多来源
├── adapter/         # 接口适配器
│   ├── http/        # Gin 路由、Handler、中间件
│   ├── projection/  # 投影适配器
│   └── saga/        # Saga 协调器
├── logger/          # 日志：Zap + 滚动文件
└── ws/              # WebSocket 适配器
```

---

## 适用场景

- 需要 **可审计、可回放** 的金融 / 电商 / 物流业务系统
- 微服务架构下的 **跨服务分布式事务** 协调
- 对 **可测试性、可维护性** 有高要求的中大型团队
- 希望用 DDD 落地但不想从零搭建基础设施的项目

---

## 开发指南

```bash
make fmt      # 格式化代码
make lint     # 运行 golangci-lint
make test     # 运行全部测试
make build    # 编译二进制
```

---

<div align="center">

**Sparrow** — 小身躯，大能量。让复杂分布式系统回归简单。

</div>

