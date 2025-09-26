# 事件存储实现指南

本目录提供了三种事件存储的实现：PostgreSQL、Redis和Nats JetStream。每种实现都支持完整的EventStore接口。

## 实现对比

| 特性             | PostgreSQL   | Redis        | Nats JetStream | BadgerDB     |
| ---------------- | ------------ | ------------ | -------------- | ------------ |
| **存储类型**     | 关系型数据库 | 内存键值存储 | 消息流+KV存储  | 嵌入式KV存储 |
| **持久化**       | ✅ 永久存储   | ⚠️ 可配置     | ✅ 永久存储     | ✅ 永久存储   |
| **事务支持**     | ✅ 完整事务   | ❌            | ✅ 原子操作     | ✅ ACID事务   |
| **并发控制**     | ✅ 乐观锁     | ✅ 版本检查   | ✅ 版本检查     | ✅ 事务锁     |
| **批量操作**     | ✅            | ✅            | ✅              | ✅            |
| **分页查询**     | ✅            | ✅            | ✅              | ✅            |
| **时间范围查询** | ✅            | ✅            | ✅              | ✅            |
| **快照支持**     | ✅            | ✅            | ✅              | ✅            |
| **性能**         | 中等         | 高           | 高             | 非常高       |
| **扩展性**       | 垂直扩展     | 水平扩展     | 水平扩展       | 垂直扩展     |
| **网络传输**     | 有           | 有           | 有             | 无           |
| **嵌入式**       | ❌            | ❌            | ❌              | ✅            |

## 使用方法

### PostgreSQL事件存储

```go
import (
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    "{{ .ProjectName }}/internal/entity/event"
)

// 初始化
db, err := sqlx.Connect("postgres", "user=postgres password=postgres dbname=events sslmode=disable")
if err != nil {
    log.Fatal(err)
}

eventStore, err := event.NewPostgreSQLEventStore(db)
if err != nil {
    log.Fatal(err)
}

// 使用示例
userCreated := UserCreatedEvent{
    UserID:   "123",
    Username: "张三",
    Email:    "zhangsan@example.com",
}

err = eventStore.SaveEvents(ctx, "user-123", []event.Event{userCreated}, 0)
```

### Redis事件存储

```go
import (
    "github.com/redis/go-redis/v9"
    "{{ .ProjectName }}/internal/entity/event"
)

// 初始化
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})

eventStore := event.NewRedisEventStore(client, "events")

// 使用示例
err := eventStore.SaveEvents(ctx, "user-123", []event.Event{userCreated}, 0)
```

### Nats JetStream事件存储

```go
import (
    "github.com/nats-io/nats.go"
    "{{ .ProjectName }}/internal/entity/event"
)

// 初始化
conn, err := nats.Connect("nats://localhost:4222")
if err != nil {
    log.Fatal(err)
}

eventStore, err := event.NewNATSEventStore(conn, "eventstore", "eventbucket")
if err != nil {
    log.Fatal(err)
}

defer eventStore.Close()

// 使用示例
err := eventStore.SaveEvents(ctx, "user-123", []event.Event{userCreated}, 0)
```

### BadgerDB事件存储

```go
import (
    "{{ .ProjectName }}/internal/entity/event"
)

// 初始化
eventStore, err := event.NewBadgerEventStore("./data/events")
if err != nil {
    log.Fatal(err)
}

defer eventStore.Close()

// 使用示例
userCreated := UserCreatedEvent{
    UserID:   "123",
    Username: "张三",
    Email:    "zhangsan@example.com",
}

err = eventStore.SaveEvents(ctx, "user-123", []event.Event{userCreated}, 0)
```

## 配置示例

### PostgreSQL

```sql
-- 创建数据库
create database events;

-- 连接字符串
DATABASE_URL=postgres://user:password@localhost:5432/events?sslmode=disable
```

### Redis

```bash
# Docker启动
redis-server --appendonly yes --save 900 1 --save 300 10 --save 60 10000

# 连接字符串
REDIS_URL=redis://localhost:6379/0
```

### Nats JetStream

```bash
# Docker启动
docker run -d --name nats -p 4222:4222 -p 8222:8222 nats:latest -js

# 连接字符串
NATS_URL=nats://localhost:4222
```

### BadgerDB

```go
// 无需额外配置，直接使用文件系统存储
// 确保数据目录有写入权限
eventStore, err := event.NewBadgerEventStore("./data/events")
```

## Docker Compose配置

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: events
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

  nats:
    image: nats:latest
    ports:
      - "4222:4222"
      - "8222:8222"
    command: -js
    volumes:
      - nats_data:/data

volumes:
  postgres_data:
  redis_data:
  nats_data:
```

## 性能建议

### PostgreSQL
- 为aggregate_id和event_type创建索引
- 使用连接池
- 定期清理旧事件（如果需要）
- 监控查询性能

### Redis
- 设置合理的内存限制
- 使用Redis集群进行水平扩展
- 配置持久化策略
- 监控内存使用

### Nats JetStream
- 配置合适的流保留策略
- 使用集群模式提高可用性
- 监控磁盘使用
- 配置消费者重试策略

### BadgerDB
- 使用SSD存储提高性能
- 配置合适的内存表大小
- 定期压缩数据库
- 监控磁盘空间使用
- 避免频繁打开/关闭数据库

## 迁移指南

### 从PostgreSQL迁移到Redis

```go
// 1. 导出PostgreSQL事件
events, err := postgresStore.GetEvents(ctx, aggregateID)

// 2. 导入到Redis
err = redisStore.SaveEvents(ctx, aggregateID, events, -1)

// 3. 迁移快照
snapshot, version, err := postgresStore.GetLatestSnapshot(ctx, aggregateID)
if snapshot != nil {
    err = redisStore.SaveSnapshot(ctx, aggregateID, snapshot, version)
}
```

### 从Redis迁移到Nats

```go
// 1. 导出Redis事件
events, err := redisStore.GetEvents(ctx, aggregateID)

// 2. 导入到Nats
err = natsStore.SaveEvents(ctx, aggregateID, events, -1)

// 3. 迁移快照
snapshot, version, err := redisStore.GetLatestSnapshot(ctx, aggregateID)
if snapshot != nil {
    err = natsStore.SaveSnapshot(ctx, aggregateID, snapshot, version)
}
```

## 错误处理

所有实现都使用统一的错误处理模式：

```go
err := eventStore.SaveEvents(ctx, aggregateID, events, expectedVersion)
if err != nil {
    var storeErr *EventStoreError
    if errors.As(err, &storeErr) {
        switch storeErr.Type {
        case "concurrency_conflict":
            // 处理并发冲突
        case "aggregate_not_found":
            // 处理聚合不存在
        default:
            // 处理其他错误
        }
    }
}
```

## 测试

### 集成测试

```go
func TestEventStore(t *testing.T) {
	stores := []struct {
		name string
		store event.EventStore
	}{
		{"PostgreSQL", setupPostgresStore(t)},
		{"Redis", setupRedisStore(t)},
		{"Nats", setupNatsStore(t)},
	}

	for _, store := range stores {
		t.Run(store.name, func(t *testing.T) {
			testEventStoreCRUD(t, store.store)
		})
	}
}

func testEventStoreCRUD(t *testing.T, store event.EventStore) {
	ctx := context.Background()
	aggregateID := "test-123"

	// 保存事件
	events := []event.Event{
		&UserCreatedEvent{UserID: "123", Username: "张三"},
		&UserUpdatedEvent{UserID: "123", Username: "李四"},
	}

	err := store.SaveEvents(ctx, aggregateID, events, 0)
	require.NoError(t, err)

	// 获取事件
	retrievedEvents, err := store.GetEvents(ctx, aggregateID)
	require.NoError(t, err)
	assert.Len(t, retrievedEvents, 2)

	// 获取版本
	version, err := store.GetAggregateVersion(ctx, aggregateID)
	require.NoError(t, err)
	assert.Equal(t, 2, version)

	// 保存快照
	snapshot := &UserSnapshot{UserID: "123", Username: "李四"}
	err = store.SaveSnapshot(ctx, aggregateID, snapshot, 2)
	require.NoError(t, err)

	// 获取快照
	retrievedSnapshot, version, err := store.GetLatestSnapshot(ctx, aggregateID)
	require.NoError(t, err)
	assert.Equal(t, 2, version)
	assert.NotNil(t, retrievedSnapshot)
}
```

## 最佳实践

### 1. 选择合适的存储
- **PostgreSQL**: 需要复杂查询、事务、数据一致性
- **Redis**: 高性能要求、缓存层、实时处理
- **Nats**: 事件驱动架构、微服务通信、水平扩展
- **BadgerDB**: 嵌入式应用、单机高性能、无需网络通信

### 2. 事件设计
- 保持事件小而专注
- 包含足够的信息进行重放
- 使用时间戳进行排序
- 添加版本号进行并发控制

### 3. 快照策略
- 基于事件数量创建快照
- 基于时间间隔创建快照
- 基于内存使用创建快照

### 4. 监控和运维
- 监控存储性能
- 设置合理的TTL（如使用Redis）
- 定期备份数据
- 监控磁盘空间使用
- BadgerDB需要定期压缩和清理

## 高级配置

### PostgreSQL分区表

```sql
-- 按时间分区
CREATE TABLE events_2024_01 PARTITION OF events
FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
```

### Redis集群

```go
// 使用Redis集群
client := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{"localhost:7000", "localhost:7001", "localhost:7002"},
})
```

### Nats集群

```bash
# 启动Nats集群
docker run -d --name nats-1 -p 4222:4222 nats:latest -js --cluster_name=eventstore --cluster=nats://0.0.0.0:6222 --routes=nats://nats-2:6222,nats://nats-3:6222
```