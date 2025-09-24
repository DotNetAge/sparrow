# 持久化仓储适配器

本目录包含了多种数据存储的仓储实现适配器，支持不同的存储引擎。

## 支持的存储引擎

### 1. PostgreSQL (`postgres.go.tmpl`)
- **特性**: 完整的关系型数据库支持
- **功能**: CRUD操作、软删除、批量操作、分页、事务、索引
- **适用场景**: 生产环境、复杂查询、数据一致性要求高

### 2. BadgerDB (`badger.go.tmpl`)
- **特性**: 嵌入式键值存储
- **功能**: CRUD操作、批量操作、分页、前缀查询
- **适用场景**: 单机应用、嵌入式系统、快速原型

### 3. Redis (`redis.go.tmpl`)
- **特性**: 内存数据结构存储
- **功能**: CRUD操作、批量操作、分页、TTL过期、高性能缓存
- **适用场景**: 缓存层、会话存储、实时数据、高并发读写

### 4. 内存存储 (`memory.go.tmpl`)
- **特性**: 基于内存的键值存储
- **功能**: 完整的CRUD操作、批量操作、分页、字段查询
- **适用场景**: 测试环境、临时数据存储、不需要持久化的场景

## 使用方法

### PostgreSQL

```go
import (
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    "{{ .ProjectName }}/internal/adapter/persistence/repo"
)

// 初始化
db, err := sqlx.Connect("postgres", "user=postgres password=postgres dbname=mydb sslmode=disable")
if err != nil {
    log.Fatal(err)
}

// 创建仓储
userRepo := repo.NewPostgresRepository[User](db, "users")

// 使用示例
user := &User{ID: "123", Name: "张三"}
err = userRepo.Save(ctx, *user)
```

### BadgerDB

```go
import (
    "github.com/dgraph-io/badger/v4"
    "{{ .ProjectName }}/internal/adapter/persistence/repo"
)

// 初始化
opts := badger.DefaultOptions("/tmp/badger")
db, err := badger.Open(opts)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// 创建仓储
userRepo := repo.NewBadgerRepository[User](db, "users")

// 使用示例
user := &User{ID: "123", Name: "张三"}
err = userRepo.Save(ctx, *user)
```

### Redis

```go
import (
    "github.com/redis/go-redis/v9"
    "{{ .ProjectName }}/internal/adapter/persistence/repo"
)

// 初始化
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "", // no password set
    DB:       0,  // use default DB
})

// 创建仓储（带1小时TTL）
userRepo := repo.NewRedisRepository[User](client, "users", time.Hour)

// 使用示例
user := &User{ID: "123", Name: "张三"}
err = userRepo.Save(ctx, *user)
```

### 内存存储

```go
import (
    "{{ .ProjectName }}/internal/adapter/persistence/repo"
)

// 创建仓储（无需外部依赖）
userRepo := repo.NewMemoryRepository[User]()

// 使用示例
user := &User{ID: "123", Name: "张三"}
err = userRepo.Save(ctx, *user)

// 测试场景：清空所有数据
_ = userRepo.Clear(ctx)
```

## 功能对比

| 功能特性 | PostgreSQL | BadgerDB | Redis | 内存存储 |
|----------|------------|----------|-------|----------|
| 事务支持 | ✅ | ❌ | ❌ | ❌ |
| 软删除 | ✅ | ❌ | ❌ | ❌ |
| 索引查询 | ✅ | ❌ | ❌ | ❌ |
| 持久化 | ✅ | ✅ | 可选 | ❌ |
| 无需外部依赖 | ❌ | ✅ | ❌ | ✅ |
| 测试友好 | ❌ | ❌ | ❌ | ✅ |
| 批量操作 | ✅ | ✅ | ✅ | ✅ |
| 分页查询 | ✅ | ✅ | ✅ | ✅ |
| 按字段查询 | ✅ | ❌ | ❌ | ✅ |
| 清空数据 | ✅ | ❌ | 部分 | ✅ |
| TTL过期 | ❌ | ❌ | ✅ | ❌ |
| 内存存储 | ❌ | ❌ | ✅ | ✅ |
| 嵌入式 | ❌ | ✅ | ❌ | ✅ |
| 网络服务 | ✅ | ❌ | ✅ | ❌ |

## 性能建议

### PostgreSQL
- 使用连接池
- 添加适当索引
- 批量操作使用事务
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

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

### 环境变量配置

```bash
# PostgreSQL
DATABASE_URL=postgres://user:password@localhost:5432/myapp?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379/0

# BadgerDB
BADGER_PATH=/tmp/badger
```

## 错误处理

所有仓储实现都使用统一的错误类型：`entity.RepositoryError`

```go
err := userRepo.Save(ctx, user)
if err != nil {
    var repoErr *entity.RepositoryError
    if errors.As(err, &repoErr) {
        fmt.Printf("操作: %s, 实体: %s, ID: %s, 错误: %s\n",
            repoErr.Operation, repoErr.EntityType, repoErr.ID, repoErr.Message)
    }
}
```

## 测试

### 单元测试示例

```go
func TestUserRepository(t *testing.T) {
    tests := []struct {
        name string
        repo entity.Repository[User]
    }{
        {"PostgreSQL", setupPostgresRepo(t)},
        {"BadgerDB", setupBadgerRepo(t)},
        {"Redis", setupRedisRepo(t)},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            testRepositoryCRUD(t, tt.repo)
        })
    }
}

func testRepositoryCRUD(t *testing.T, repo entity.Repository[User]) {
    ctx := context.Background()
    
    // 创建
    user := User{ID: "123", Name: "张三"}
    err := repo.Save(ctx, user)
    require.NoError(t, err)
    
    // 读取
    found, err := repo.FindByID(ctx, "123")
    require.NoError(t, err)
    assert.Equal(t, "张三", found.Name)
    
    // 更新
    user.Name = "李四"
    err = repo.Save(ctx, user)
    require.NoError(t, err)
    
    // 删除
    err = repo.Delete(ctx, "123")
    require.NoError(t, err)
    
    // 验证删除
    _, err = repo.FindByID(ctx, "123")
    assert.Error(t, err)
}
```

## 迁移指南

### 从PostgreSQL迁移到Redis

```go
// 1. 导出PostgreSQL数据
users, err := postgresRepo.FindAll(ctx)

// 2. 批量导入到Redis
err = redisRepo.SaveBatch(ctx, users)

// 3. 设置TTL
for _, user := range users {
    redisRepo.SetTTLForKey(ctx, user.ID, time.Hour)
}
```

### 从Redis迁移到PostgreSQL

```go
// 1. 导出Redis数据
users, err := redisRepo.FindAll(ctx)

// 2. 批量导入到PostgreSQL
err = postgresRepo.SaveBatch(ctx, users)
```