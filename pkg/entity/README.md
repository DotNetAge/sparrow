# 聚合根模板使用指南

## 概述

这个模板系统提供了一个完整的聚合根实现，遵循领域驱动设计(DDD)原则和事件溯源模式。模板使用Go模板语法，支持从YAML配置文件中动态生成聚合根代码。

## 模板特性

### 1. 接口实现
- **完整实现 `entity.AggregateRoot` 接口**
- **支持事件溯源(Event Sourcing)**
- **支持快照(Snapshot)功能**
- **版本控制(乐观锁)**

### 2. 核心功能
- 聚合根生命周期管理
- 事件存储和回放
- 业务验证规则
- 快照支持
- 乐观并发控制

### 3. 配置驱动
- 从 `msdl.yml` 配置文件读取聚合定义
- 支持字段级验证规则
- 可配置JSON标签
- 可选字段支持

## 文件结构

```
templates/entities/internal/entity/
├── {{.AggregateName}}/
│   └── aggregate.go.tmpl          # 聚合根模板
├── event/
│   ├── eventstore.go.tmpl         # 事件存储接口
│   └── snapshot.go.tmpl           # 快照接口
├── interfaces.go.tmpl             # 接口定义(包含AggregateRoot)
├── msdl.yml.example               # 配置示例
└── generate_aggregate.sh           # 生成脚本
```

## 使用方法

### 1. 配置聚合定义

在 `msdl.yml` 中定义聚合根：

```yaml
project:
  name: "my-project"
  package: "myapp"

aggregates:
  - name: "Order"
    description: "订单聚合根"
    aggregate_type: "Order"
    fields:
      - name: "CustomerID"
        type: "string"
        json: "customer_id"
        required: true
        validation:
          type: "regex"
          pattern: "^[a-zA-Z0-9-]+$"
      - name: "Amount"
        type: "float64"
        json: "amount"
        required: true
        validation:
          type: "min"
          value: 0.01
```

### 2. 生成聚合根

使用生成脚本：

```bash
# 生成特定聚合根
./generate_aggregate.sh Order msdl.yml

# 或手动使用模板
export ProjectName=my-project
export PackageName=myapp
export AggregateName=Order
go generate
```

### 3. 实现事件处理

生成的聚合根需要补充事件处理逻辑：

```go
// 在 ApplyEvent 方法中添加事件处理
func (a *Order) ApplyEvent(event interface{}) error {
    switch e := event.(type) {
    case *OrderCreatedEvent:
        a.CustomerID = e.CustomerID
        a.Amount = e.Amount
        a.Status = "pending"
    case *OrderConfirmedEvent:
        a.Status = "confirmed"
    default:
        return errors.New("unknown event type")
    }
    return nil
}
```

## 模板变量

### 模板变量说明

| 变量名           | 说明       | 示例         |
| ---------------- | ---------- | ------------ |
| `.ProjectName`   | 项目名称   | `my-project` |
| `.PackageName`   | 包名       | `myapp`      |
| `.AggregateName` | 聚合根名称 | `Order`      |
| `.AggregateType` | 聚合类型   | `order`      |
| `.Description`   | 描述信息   | `订单聚合根` |
| `.Fields`        | 字段列表   | 见下表       |

### 字段配置

每个字段支持以下配置：

```yaml
fields:
  - name: "FieldName"        # 字段名(首字母大写)
    type: "string"           # Go类型
    json: "field_name"       # JSON标签(可选)
    required: true            # 是否必填(可选, 默认为true)
    validation:               # 验证规则(可选)
      type: "min"            # 验证类型: required, min, max, regex
      value: 18              # 对于min/max类型
      pattern: "^[0-9]+$"    # 对于regex类型
```

## 验证规则

支持以下验证规则：

1. **required**: 必填字段
2. **min**: 最小值(用于数字)
3. **max**: 最大值(用于数字)
4. **regex**: 正则表达式匹配(用于字符串)

## 事件使用示例

```go
// 创建聚合根
order, err := NewOrder("order-123", "customer-456", 99.99)

// 添加事件
order.AddEvent(&OrderCreatedEvent{
    OrderID:    order.ID,
    CustomerID: order.CustomerID,
    Amount:     order.Amount,
})

// 获取未提交事件
events := order.GetUncommittedEvents()

// 保存到事件存储
err := eventStore.SaveEvents(order.GetAggregateType(), order.ID, events, order.Version)

// 标记为已提交
order.MarkEventsAsCommitted()
```

## 快照使用

```go
// 获取快照数据
snapshotData := order.GetSnapshotData()

// 保存快照
err := snapshotStore.SaveSnapshot(snapshotData)

// 从快照恢复
snapshot, err := snapshotStore.GetLatestSnapshot("Order", orderID)
if err == nil {
    order := &Order{}
    order.SetVersion(snapshot.Version)
    // 加载状态...
}
```

## 注意事项

1. **事件处理**: 生成的 `ApplyEvent` 方法需要手动实现具体的事件处理逻辑
2. **业务方法**: 根据业务需求添加自定义方法
3. **事件定义**: 需要单独定义事件类型结构体
4. **验证规则**: 复杂的业务验证可以在 `Validate` 方法中扩展
5. **并发控制**: 使用版本号进行乐观并发控制

## 扩展建议

1. **添加业务方法**: 在生成的聚合根中添加业务行为方法
2. **事件定义**: 为每个业务操作定义对应的事件类型
3. **工厂方法**: 根据业务需求扩展工厂方法
4. **验证规则**: 实现更复杂的业务验证逻辑
5. **查询优化**: 结合CQRS模式实现查询优化


```go
// 配套使用示例：AggregateRoot + EventStore + Snapshot
// 使用示例：AggregateRoot与EventStore和Snapshot的完整配套
package example

import (
	"context"
	"time"
	"errors"
	"fmt"
)

// Order 聚合根示例
type Order struct {
	ID         string
	CustomerID string
	Status     string
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	
	uncommittedEvents []event.DomainEvent
}

// 实现AggregateRoot接口
func (o *Order) GetID() string            { return o.ID }
func (o *Order) SetID(id string)         { o.ID = id }
func (o *Order) GetCreatedAt() time.Time { return o.CreatedAt }
func (o *Order) GetUpdatedAt() time.Time { return o.UpdatedAt }
func (o *Order) SetUpdatedAt(t time.Time) { o.UpdatedAt = t }

func (o *Order) GetAggregateType() string   { return "Order" }
func (o *Order) GetAggregateID() string     { return o.ID }
func (o *Order) GetVersion() int            { return o.Version }
func (o *Order) SetVersion(version int)    { o.Version = version }
func (o *Order) IncrementVersion()         { o.Version++ }

func (o *Order) GetUncommittedEvents() []event.DomainEvent {
	return o.uncommittedEvents
}

func (o *Order) MarkEventsAsCommitted() {
	o.uncommittedEvents = []event.DomainEvent{}
}

func (o *Order) LoadFromEvents(events []event.DomainEvent) error {
	o.Version = 0
	for _, event := range events {
		if err := o.ApplyEvent(event); err != nil {
			return err
		}
	}
	return nil
}

func (o *Order) ApplyEvent(event interface{}) error {
	switch e := event.(type) {
	case *OrderCreatedEvent:
		o.ID = e.OrderID
		o.CustomerID = e.CustomerID
		o.Status = "created"
		o.CreatedAt = e.CreatedAt
		o.UpdatedAt = e.CreatedAt
		o.Version = e.Version
	default:
		return errors.New("unknown event type")
	}
	return nil
}

func (o *Order) Validate() error {
	if o.ID == "" {
		return errors.New("order ID is required")
	}
	return nil
}

// 与EventStore和Snapshot的配套使用
func ExampleUsage() {
	ctx := context.Background()
	
	// 1. 创建事件存储
	var eventStore event.EventStore
	
	// 2. 创建聚合根
	order := &Order{ID: "order-123"}
	
	// 3. 创建事件
	event := &OrderCreatedEvent{
		OrderID:    "order-123",
		CustomerID: "customer-456",
		CreatedAt:  time.Now(),
		Version:    1,
	}
	
	// 4. 应用事件
	err := order.ApplyEvent(event)
	if err != nil {
		panic(err)
	}
	
	// 5. 保存事件到事件存储
	err = eventStore.SaveEvents(ctx, order.GetAggregateID(), 
		[]event.DomainEvent{event}, order.GetVersion())
	if err != nil {
		panic(err)
	}
	
	// 6. 创建快照
	snapshot := event.NewSnapshotData("Order", order.GetAggregateID(), 
		order.GetVersion(), *order)
	
	// 7. 保存快照
	err = eventStore.SaveSnapshot(ctx, order.GetAggregateID(), 
		snapshot, order.GetVersion())
	if err != nil {
		panic(err)
	}
	
	// 8. 从快照恢复
	snapshotData, version, err := eventStore.GetLatestSnapshot(ctx, order.GetAggregateID())
	if err != nil {
		panic(err)
	}
	
	// 9. 从快照恢复聚合状态
	if snapshotData != nil {
		snapshot := snapshotData.(*event.SnapshotData[Order])
		restoredOrder := snapshot.State
		
		// 10. 获取后续事件并应用
		events, err := eventStore.GetEventsFromVersion(ctx, 
			restoredOrder.GetAggregateID(), version)
		if err != nil {
			panic(err)
		}
		
		// 11. 应用事件恢复完整状态
		err = restoredOrder.LoadFromEvents(events)
		if err != nil {
			panic(err)
		}
		
		fmt.Printf("订单恢复成功: %+v\n", restoredOrder)
	}
}

```


```go
// 使用示例
// 定义实体
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u *User) GetID() string          { return u.ID }
func (u *User) SetID(id string)       { u.ID = id }
func (u *User) GetCreatedAt() time.Time { return u.CreatedAt }
func (u *User) GetUpdatedAt() time.Time { return u.UpdatedAt }
func (u *User) SetUpdatedAt(t time.Time) { u.UpdatedAt = t }

// 使用示例
func main() {
	// 创建基础仓储
	repo := NewBaseRepository[User]()
	
	// 使用示例
	ctx := context.Background()
	
	// 1. 简单字段查询（保持向后兼容）
	users, err := repo.FindByField(ctx, "department_id", "dept-123")
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
	}
	
	// 2. 带分页的字段查询（新增功能）
	page1Users, err := repo.FindByFieldWithPagination(ctx, "department_id", "dept-123", 10, 0)
	if err != nil {
		fmt.Printf("分页查询失败: %v\n", err)
	}
	
	// 3. 统计符合字段条件的总数
	totalUsers, err := repo.CountByField(ctx, "department_id", "dept-123")
	if err != nil {
		fmt.Printf("统计失败: %v\n", err)
	}
	
	fmt.Printf("部门ID为dept-123的用户共有 %d 名，第1页显示 %d 名\n", totalUsers, len(page1Users))
	
	// 4. 多条件复杂查询（结合之前设计）
	conditions := []QueryCondition{
		NewQueryCondition("department_id", "EQ", "dept-123"),
		NewQueryCondition("age", "GT", 25),
	}
	
	sortFields := []SortField{
		NewSortField("created_at", false),
	}
	
	options := QueryOptions{
		Conditions: conditions,
		SortFields: sortFields,
		Limit:      10,
		Offset:     0,
	}
	
	complexQueryUsers, err := repo.FindWithConditions(ctx, options)
	if err != nil {
		fmt.Printf("复杂查询失败: %v\n", err)
	}
	
	complexQueryTotal, err := repo.CountWithConditions(ctx, conditions)
	if err != nil {
		fmt.Printf("复杂查询统计失败: %v\n", err)
	}
}
```