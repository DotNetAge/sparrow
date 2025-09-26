# 持久化仓储适配器

本目录包含了多种数据存储的仓储实现适配器，支持不同的存储引擎。这些适配器提供统一的接口，使应用程序可以轻松切换底层存储而无需改变业务逻辑。

## 支持的存储引擎

### 1. PostgreSQL (`postgres.go`)
- **特性**: 完整的关系型数据库支持
- **功能**: CRUD操作、软删除、批量操作、分页、事务、索引
- **适用场景**: 生产环境、复杂查询、数据一致性要求高

### 2. MongoDB (`mongodb.go`)
- **特性**: 文档型NoSQL数据库
- **功能**: CRUD操作、批量操作、分页、复杂嵌套查询、事务
- **适用场景**: 半结构化数据、快速开发、灵活模式、水平扩展

### 3. GORM (`gorm.go`)
- **特性**: Go语言的ORM库
- **功能**: 支持多种关系型数据库(MySQL, PostgreSQL, SQLite等)，完整ORM功能
- **适用场景**: 需要ORM功能、跨数据库兼容性、复杂关系模型

### 4. BadgerDB (`badger.go`)
- **特性**: 嵌入式键值存储
- **功能**: CRUD操作、批量操作、分页、前缀查询
- **适用场景**: 单机应用、嵌入式系统、快速原型

### 5. Redis (`redis.go`)
- **特性**: 内存数据结构存储
- **功能**: CRUD操作、批量操作、分页、TTL过期、高性能缓存
- **适用场景**: 缓存层、会话存储、实时数据、高并发读写

### 6. 内存存储 (`memory.go`)
- **特性**: 基于内存的键值存储
- **功能**: 完整的CRUD操作、批量操作、分页、字段查询
- **适用场景**: 测试环境、临时数据存储、不需要持久化的场景

## 仓库接口定义

所有存储实现都遵循统一的`usecase.Repository[T]`接口，定义如下：

```go
// Repository 泛型仓储接口
type Repository[T any] interface {
    // Save 保存实体
    Save(ctx context.Context, entity T) error
    // FindByID 根据实体的ID查询
    FindByID(ctx context.Context, id string) (T, error)
    // FindAll 返回所有实体
    FindAll(ctx context.Context) ([]T, error)
    // Update 更新实体数据
    Update(ctx context.Context, entity T) error
    // Delete 删除指定ID的实体
    Delete(ctx context.Context, id string) error
    // SaveBatch 批量操作
    SaveBatch(ctx context.Context, entities []T) error
    // FindByIDs 根据ID列表批量查询
    FindByIDs(ctx context.Context, ids []string) ([]T, error)
    // DeleteBatch 批量删除
    DeleteBatch(ctx context.Context, ids []string) error
    // FindWithPagination 分页查询
    FindWithPagination(ctx context.Context, limit, offset int) ([]T, error)
    // Count 统计实体数量
    Count(ctx context.Context) (int64, error)
    // FindByField 根据指定字段的值查询
    FindByField(ctx context.Context, field string, value interface{}) ([]T, error)
    // Exists 检查指定ID的实体是否存在
    Exists(ctx context.Context, id string) (bool, error)
    // FindByFieldWithPagination 按字段查找并支持分页
    FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error)
    // CountByField 按字段统计数量
    CountByField(ctx context.Context, field string, value interface{}) (int64, error)
    // FindWithConditions 根据条件查询
    FindWithConditions(ctx context.Context, options QueryOptions) ([]T, error)
    // CountWithConditions 根据条件统计
    CountWithConditions(ctx context.Context, conditions []QueryCondition) (int64, error)
}
```

## 实体定义

实体需要实现`entity.Entity`接口，该接口要求实现以下方法：

```go
// Entity 定义实体接口
// 所有需要持久化的实体都必须实现此接口
type Entity interface {
    GetID() string
    SetID(id string)
}
```

系统提供了基础实体实现`entity.BaseEntity`，可以直接继承使用：

```go
// BaseEntity 提供基本实体实现
// 包含ID、创建时间、更新时间等公共字段
// 使用示例: type User struct { entity.BaseEntity; Name string }
type BaseEntity struct {
    Id        string    `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

## 使用方法

### PostgreSQL

```go
import (
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    "github.com/DotNetAge/sparrow/pkg/persistence/repo"
)

// 初始化
opts := "user=postgres password=postgres dbname=mydb sslmode=disable"
db, err := sqlx.Connect("postgres", opts)
if err != nil {
    log.Fatal(err)
}

tableName := "users"
// 创建仓储
userRepo := repo.NewPostgresRepository[*User](db, tableName)

// 使用示例
user := &User{Name: "张三", Email: "zhangsan@example.com"}
user.SetID("123")
err = userRepo.Save(ctx, user)
```

### MongoDB

```go
import (
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "github.com/DotNetAge/sparrow/pkg/persistence/repo"
)

// 初始化
clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
client, err := mongo.Connect(ctx, clientOptions)
if err != nil {
    log.Fatal(err)
}

dbName := "myapp"
// 创建仓储
userRepo := repo.NewMongoDBRepository[*User](client, dbName)

// 使用示例
user := &User{Name: "张三", Email: "zhangsan@example.com"}
// MongoDB支持自动生成ObjectID
user.SetID(primitive.NewObjectID().Hex())
err = userRepo.Save(ctx, user)
```

### GORM

```go
import (
    "gorm.io/gorm"
    "gorm.io/driver/postgres"
    "github.com/DotNetAge/sparrow/pkg/persistence/repo"
)

// 初始化
opts := "host=localhost user=postgres password=postgres dbname=mydb port=5432 sslmode=disable"
db, err := gorm.Open(postgres.Open(opts), &gorm.Config{})
if err != nil {
    log.Fatal(err)
}

// 创建仓储 - GORM会自动映射表名
userRepo := repo.NewGormRepository[*User](db)

// 使用示例
user := &User{Name: "张三", Email: "zhangsan@example.com"}
err = userRepo.Save(ctx, user)
```

### BadgerDB

```go
import (
    "github.com/dgraph-io/badger/v4"
    "github.com/DotNetAge/sparrow/pkg/persistence/repo"
)

// 初始化
opts := badger.DefaultOptions("/tmp/badger")
db, err := badger.Open(opts)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

prefix := "users"
// 创建仓储
userRepo := repo.NewBadgerRepository[*User](db, prefix)

// 使用示例
user := &User{Name: "张三", Email: "zhangsan@example.com"}
user.SetID("123")
err = userRepo.Save(ctx, user)
```

### Redis

```go
import (
    "github.com/redis/go-redis/v9"
    "time"
    "github.com/DotNetAge/sparrow/pkg/persistence/repo"
)

// 初始化
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "", // no password set
    DB:       0,  // use default DB
})

prefix := "users"
// 创建仓储（带1小时TTL）
userRepo := repo.NewRedisRepository[*User](client, prefix, time.Hour)

// 使用示例
user := &User{Name: "张三", Email: "zhangsan@example.com"}
user.SetID("123")
err = userRepo.Save(ctx, user)
```

### 内存存储

```go
import (
    "github.com/DotNetAge/sparrow/pkg/persistence/repo"
)

// 创建仓储（无需外部依赖）
userRepo := repo.NewMemoryRepository[*User]()

// 使用示例
user := &User{Name: "张三", Email: "zhangsan@example.com"}
user.SetID("123")
err = userRepo.Save(ctx, user)

// 测试场景：清空所有数据
_ = userRepo.Clear(ctx)
```

## 功能对比

| 功能特性 | PostgreSQL | MongoDB | GORM | BadgerDB | Redis | 内存存储 |
|----------|------------|---------|------|----------|-------|----------|
| 事务支持 | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| 软删除 | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| 索引查询 | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| 持久化 | ✅ | ✅ | ✅ | ✅ | 可选 | ❌ |
| 无需外部依赖 | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ |
| 测试友好 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 批量操作 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 分页查询 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 按字段查询 | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
| 清空数据 | ✅ | ❌ | ✅ | ❌ | 部分 | ✅ |
| TTL过期 | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
| 内存存储 | ❌ | 部分 | ❌ | ❌ | ✅ | ✅ |
| 嵌入式 | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ |
| 网络服务 | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ |
| 文档存储 | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| 跨数据库支持 | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |

## 性能建议

### PostgreSQL
- 使用连接池
- 添加适当索引
- 批量操作使用事务
- 监控慢查询

### MongoDB
- 使用合适的索引
- 避免大型文档
- 使用投影限制返回字段
- 考虑分片策略应对大数据量

### GORM
- 预加载关联数据以减少N+1查询
- 使用事务批量处理
- 适当使用原生SQL提高性能
- 监控慢查询

### BadgerDB
- 避免大量小写入
- 定期压缩数据库
- 合理设置缓存大小
- 使用批量操作

### Redis
- 设置合理的TTL
- 使用批量操作减少网络往返
- 监控内存使用
- 考虑持久化配置

## 配置示例

### Docker Compose 配置

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: myapp
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  mongodb:
    image: mongo:6.0
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  mongo_data:
  redis_data:
```

### 环境变量配置

```bash
# PostgreSQL
DATABASE_URL=postgres://user:password@localhost:5432/myapp?sslmode=disable

# MongoDB
MONGO_URL=mongodb://localhost:27017/myapp

# Redis
REDIS_URL=redis://localhost:6379/0

# BadgerDB
BADGER_PATH=/tmp/badger
```

## 错误处理

所有仓储实现都使用统一的错误类型：`errs.RepositoryError`

```go
import (
    "errors"
    "github.com/DotNetAge/sparrow/pkg/errs"
)

err := userRepo.Save(ctx, user)
if err != nil {
    var repoErr *errs.RepositoryError
    if errors.As(err, &repoErr) {
        fmt.Printf("操作: %s, 实体: %s, ID: %s, 错误: %s\n",
            repoErr.Operation, repoErr.EntityType, repoErr.ID, repoErr.Message)
    }
}
```

## 测试

### 单元测试示例

```go
import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/DotNetAge/sparrow/pkg/entity"
    "github.com/DotNetAge/sparrow/pkg/persistence/repo"
)

// User 示例实体
type User struct {
    entity.BaseEntity
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func TestUserRepository(t *testing.T) {
    tests := []struct {
        name string
        repo usecase.Repository[*User]
    }{
        {"PostgreSQL", setupPostgresRepo(t)},
        {"MongoDB", setupMongoDBRepo(t)},
        {"GORM", setupGORMRepo(t)},
        {"BadgerDB", setupBadgerRepo(t)},
        {"Redis", setupRedisRepo(t)},
        {"Memory", repo.NewMemoryRepository[*User]()},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            testRepositoryCRUD(t, tt.repo)
        })
    }
}

func testRepositoryCRUD(t *testing.T, repo usecase.Repository[*User]) {
    ctx := context.Background()
    
    // 创建
    user := &User{Name: "张三", Email: "zhangsan@example.com", Age: 30}
    user.SetID("123")
    err := repo.Save(ctx, user)
    require.NoError(t, err)
    
    // 读取
    found, err := repo.FindByID(ctx, "123")
    require.NoError(t, err)
    assert.Equal(t, "张三", found.Name)
    assert.Equal(t, "zhangsan@example.com", found.Email)
    
    // 更新
    found.Name = "李四"
    err = repo.Save(ctx, found)
    require.NoError(t, err)
    
    // 验证更新
    updated, err := repo.FindByID(ctx, "123")
    require.NoError(t, err)
    assert.Equal(t, "李四", updated.Name)
    
    // 按字段查询
    users, err := repo.FindByField(ctx, "age", 30)
    require.NoError(t, err)
    assert.GreaterOrEqual(t, len(users), 1)
    
    // 删除
    err = repo.Delete(ctx, "123")
    require.NoError(t, err)
    
    // 验证删除
    _, err = repo.FindByID(ctx, "123")
    assert.Error(t, err)
}
```

### 复杂实体测试示例

```go
// OrderItem 订单项
type OrderItem struct {
    ID          string  `json:"id"`
    ProductID   string  `json:"product_id"`
    ProductName string  `json:"product_name"`
    Quantity    int     `json:"quantity"`
    UnitPrice   float64 `json:"unit_price"`
}

// Order 订单实体
type Order struct {
    entity.BaseEntity
    OrderNumber     string `json:"order_number"`
    CustomerID      string `json:"customer_id"`
    Status          string `json:"status"`
    TotalAmount     float64 `json:"total_amount"`
    Items           []OrderItem `json:"items"`
    ShippingAddress struct {
        Name    string `json:"name"`
        Phone   string `json:"phone"`
        Address string `json:"address"`
        City    string `json:"city"`
        ZIPCode string `json:"zip_code"`
        Country string `json:"country"`
    } `json:"shipping_address"`
    PaymentInfo struct {
        Method       string `json:"method"`
        TransactionID string `json:"transaction_id"`
        Status       string `json:"status"`
    } `json:"payment_info"`
}

func TestComplexEntityRepository(t *testing.T) {
    // 初始化仓储（根据需要选择不同存储）
    repo := repo.NewMongoDBRepository[*Order](client, dbName)
    ctx := context.Background()
    
    // 创建包含嵌套结构的订单
    order := &Order{
        OrderNumber: "ORD-2024-001",
        CustomerID:  "cust-456",
        Status:      "pending",
        TotalAmount: 349.97,
        Items: []OrderItem{
            {
                ID:          "item-1",
                ProductID:   "prod-001",
                ProductName: "高级智能手表",
                Quantity:    1,
                UnitPrice:   299.99,
            },
            {
                ID:          "item-2",
                ProductID:   "prod-002",
                ProductName: "手表充电器",
                Quantity:    2,
                UnitPrice:   24.99,
            },
        },
    }
    order.SetID("order-123")
    
    // 保存复杂实体
    err := repo.Save(ctx, order)
    require.NoError(t, err)
    
    // 查询并验证复杂实体
    found, err := repo.FindByID(ctx, "order-123")
    require.NoError(t, err)
    assert.Equal(t, 2, len(found.Items))
    assert.Equal(t, "高级智能手表", found.Items[0].ProductName)
}
```

## 迁移指南

### 从一种存储迁移到另一种存储

```go
// 1. 从源存储导出数据
ctx := context.Background()
sourceRepo := setupSourceRepository()
entities, err := sourceRepo.FindAll(ctx)

// 2. 批量导入到目标存储
targetRepo := setupTargetRepository()
err = targetRepo.SaveBatch(ctx, entities)
```

### 从PostgreSQL迁移到MongoDB

```go
// 1. 导出PostgreSQL数据
users, err := postgresRepo.FindAll(ctx)

// 2. 批量导入到MongoDB
// MongoDB会自动处理嵌套结构和ID格式
err = mongoRepo.SaveBatch(ctx, users)
```

### 从Redis迁移到PostgreSQL

```go
// 1. 导出Redis数据
users, err := redisRepo.FindAll(ctx)

// 2. 批量导入到PostgreSQL
err = postgresRepo.SaveBatch(ctx, users)
```

## 扩展性

如需添加新的存储适配器，只需实现`usecase.Repository[T]`接口，并确保遵循以下模式：

```go
// NewCustomRepository 创建自定义存储仓储实例
func NewCustomRepository[T entity.Entity](/* 必要的连接参数 */) *CustomRepository[T] {
    // 初始化逻辑
    return &CustomRepository[T]{
        /* 字段初始化 */
    }
}

// CustomRepository 自定义存储实现
type CustomRepository[T entity.Entity] struct {
    // 实现usecase.BaseRepository[T]或直接实现接口方法
    /* 存储相关字段 */
}

// 实现所有必要的接口方法
func (r *CustomRepository[T]) Save(ctx context.Context, entity T) error {
    // 保存逻辑
}

// 实现其他接口方法...
```

## 最佳实践

1. **依赖注入**：使用依赖注入模式注入仓储接口，而不是具体实现
2. **事务处理**：在业务层使用事务处理涉及多个实体变更的操作
3. **实体设计**：遵循DDD原则，将业务逻辑封装在实体中
4. **错误处理**：统一处理和转换仓储层错误
5. **测试隔离**：使用内存存储进行单元测试，集成测试使用实际存储
6. **性能优化**：根据具体存储类型应用相应的性能优化策略
7. **版本控制**：对于数据库模式变更，实施适当的版本控制策略

## 兼容性说明

- 所有仓储实现都支持泛型实体
- 对于嵌套结构，MongoDB和GORM提供更好的支持
- 事务支持因存储类型而异，使用前请查阅功能对比表
- 实体ID格式应与存储类型兼容（如MongoDB建议使用ObjectID）

## 相关文档

- [实体定义指南](../entity/README.md)
- [错误处理文档](../errs/README.md)
- [用例层接口定义](../usecase/README.md)