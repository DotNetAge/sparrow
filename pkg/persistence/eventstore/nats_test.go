package eventstore

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	natstestcontainers "github.com/testcontainers/testcontainers-go/modules/nats"
)

// 测试用的模拟AggregateRoot实现
type mockAggregateRoot struct {
	ID          string
	Type        string
	Version     int
	Uncommitted []entity.DomainEvent
	CreatedAt   time.Time
	UpdatedAt   time.Time
	mock.Mock
}

func (m *mockAggregateRoot) GetID() string {
	return m.ID
}

func (m *mockAggregateRoot) SetID(id string) {
	m.ID = id
}

func (m *mockAggregateRoot) SetUpdatedAt(t time.Time) {
	m.UpdatedAt = t
}

func (m *mockAggregateRoot) GetAggregateType() string {
	return m.Type
}

func (m *mockAggregateRoot) GetAggregateID() string {
	return m.ID
}

func (m *mockAggregateRoot) GetVersion() int {
	return m.Version
}

func (m *mockAggregateRoot) SetVersion(version int) {
	m.Version = version
}

func (m *mockAggregateRoot) IncrementVersion() {
	m.Version++
}

func (m *mockAggregateRoot) GetUncommittedEvents() []entity.DomainEvent {
	return m.Uncommitted
}

func (m *mockAggregateRoot) MarkEventsAsCommitted() {
	m.Uncommitted = []entity.DomainEvent{}
}

func (m *mockAggregateRoot) AddUncommittedEvents(events []entity.DomainEvent) {
	m.Uncommitted = append(m.Uncommitted, events...)
}

func (m *mockAggregateRoot) LoadFromEvents(events []entity.DomainEvent) error {
	args := m.Called(events)
	return args.Error(0)
}

func (m *mockAggregateRoot) LoadFromSnapshot(snapshot interface{}) error {
	args := m.Called(snapshot)
	return args.Error(0)
}

func (m *mockAggregateRoot) ApplyEvent(event interface{}) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *mockAggregateRoot) Validate() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockAggregateRoot) GetCreatedAt() time.Time {
	return m.CreatedAt
}

func (m *mockAggregateRoot) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}

func (m *mockAggregateRoot) HandleCommand(cmd interface{}) ([]entity.DomainEvent, error) {
	args := m.Called(cmd)
	return args.Get(0).([]entity.DomainEvent), args.Error(1)
}

// 测试用的模拟DomainEvent实现
type mockDomainEvent struct {
	ID            string
	AggregateID   string
	AggregateType string
	EventType     string
	Version       int
	Timestamp     time.Time
	Payload       map[string]interface{}
}

func (m *mockDomainEvent) GetEventID() string {
	return m.ID
}

func (m *mockDomainEvent) GetAggregateID() string {
	return m.AggregateID
}

func (m *mockDomainEvent) GetAggregateType() string {
	return m.AggregateType
}

func (m *mockDomainEvent) GetEventType() string {
	return m.EventType
}

func (m *mockDomainEvent) GetVersion() int {
	return m.Version
}

func (m *mockDomainEvent) GetCreatedAt() time.Time {
	return m.Timestamp
}

func TestNatsEventStore_SaveEvents(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 测试保存事件
	aggregateID := "test-aggregate-id"
	events := []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-1",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TestCreated",
			Version:       1,
			Timestamp:     time.Now(),
			Payload:       map[string]interface{}{"name": "test"},
		},
		&mockDomainEvent{
			ID:            "event-2",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TestUpdated",
			Version:       2,
			Timestamp:     time.Now(),
			Payload:       map[string]interface{}{"name": "updated"},
		},
	}

	// 测试保存成功
	err = eventsStore.SaveEvents(ctx, aggregateID, events, -1)
	assert.NoError(t, err)

	// 验证事件是否保存成功
	savedEvents, err := eventsStore.GetEvents(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(savedEvents))
	assert.Equal(t, "TestCreated", savedEvents[0].GetEventType())
	assert.Equal(t, "TestUpdated", savedEvents[1].GetEventType())

	// 测试并发冲突
	conflictErr := eventsStore.SaveEvents(ctx, aggregateID, events, 1)
	assert.Error(t, conflictErr)
}

func TestNatsEventStore_GetEventsFromVersion(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_version",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 保存测试事件
	aggregateID := "test-aggregate-id-version"
	events := []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-1",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TestCreated",
			Version:       1,
			Timestamp:     time.Now(),
		},
		&mockDomainEvent{
			ID:            "event-2",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TestUpdated",
			Version:       2,
			Timestamp:     time.Now(),
		},
		&mockDomainEvent{
			ID:            "event-3",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TestDeleted",
			Version:       3,
			Timestamp:     time.Now(),
		},
	}

	err = eventsStore.SaveEvents(ctx, aggregateID, events, -1)
	assert.NoError(t, err)

	// 测试从指定版本获取事件
	eventsFromVersion, err := eventsStore.GetEventsFromVersion(ctx, aggregateID, 2)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(eventsFromVersion))
	assert.Equal(t, "TestUpdated", eventsFromVersion[0].GetEventType())
	assert.Equal(t, "TestDeleted", eventsFromVersion[1].GetEventType())
}

func TestNatsEventStore_GetEventsByType(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_type",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 保存不同类型的测试事件
	aggregateID1 := "test-aggregate-id-type-1"
	aggregateID2 := "test-aggregate-id-type-2"
	events1 := []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-1",
			AggregateID:   aggregateID1,
			AggregateType: "TestAggregate",
			EventType:     "TypeA",
			Version:       1,
			Timestamp:     time.Now(),
		},
	}
	events2 := []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-2",
			AggregateID:   aggregateID2,
			AggregateType: "TestAggregate",
			EventType:     "TypeB",
			Version:       1,
			Timestamp:     time.Now(),
		},
		&mockDomainEvent{
			ID:            "event-3",
			AggregateID:   aggregateID2,
			AggregateType: "TestAggregate",
			EventType:     "TypeA",
			Version:       2,
			Timestamp:     time.Now(),
		},
	}

	err = eventsStore.SaveEvents(ctx, aggregateID1, events1, -1)
	assert.NoError(t, err)
	err = eventsStore.SaveEvents(ctx, aggregateID2, events2, -1)
	assert.NoError(t, err)

	// 测试按类型获取事件
	typeAEvents, err := eventsStore.GetEventsByType(ctx, "TypeA")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(typeAEvents))

	typeBEvents, err := eventsStore.GetEventsByType(ctx, "TypeB")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(typeBEvents))
}

func TestNatsEventStore_GetEventsByTimeRange(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_time",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 保存不同时间的测试事件
	aggregateID := "test-aggregate-id-time"
	now := time.Now()
	events := []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-1",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TypeA",
			Version:       1,
			Timestamp:     now.Add(-2 * time.Hour),
		},
		&mockDomainEvent{
			ID:            "event-2",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TypeB",
			Version:       2,
			Timestamp:     now.Add(-1 * time.Hour),
		},
		&mockDomainEvent{
			ID:            "event-3",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TypeC",
			Version:       3,
			Timestamp:     now,
		},
	}

	err = eventsStore.SaveEvents(ctx, aggregateID, events, -1)
	assert.NoError(t, err)

	// 测试按时间范围获取事件
	fromTime := now.Add(-150 * time.Minute)
	toTime := now.Add(-30 * time.Minute)
	rangeEvents, err := eventsStore.GetEventsByTimeRange(ctx, aggregateID, fromTime, toTime)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(rangeEvents))
	assert.Equal(t, "TypeB", rangeEvents[0].GetEventType())
}

func TestNatsEventStore_GetEventsWithPagination(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_pag",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 保存多个测试事件
	aggregateID := "test-aggregate-id-pag"
	events := make([]entity.DomainEvent, 0, 10)
	for i := 0; i < 10; i++ {
		events = append(events, &mockDomainEvent{
			ID:            fmt.Sprintf("event-%d", i+1),
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     fmt.Sprintf("Type%d", i+1),
			Version:       i + 1,
			Timestamp:     time.Now(),
		})
	}

	err = eventsStore.SaveEvents(ctx, aggregateID, events, -1)
	assert.NoError(t, err)

	// 测试分页获取事件
	page1, err := eventsStore.GetEventsWithPagination(ctx, aggregateID, 5, 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page1))
	assert.Equal(t, "Type1", page1[0].GetEventType())

	page2, err := eventsStore.GetEventsWithPagination(ctx, aggregateID, 5, 5)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page2))
	assert.Equal(t, "Type6", page2[0].GetEventType())

	// 测试超出范围的分页
	emptyPage, err := eventsStore.GetEventsWithPagination(ctx, aggregateID, 5, 20)
	assert.NoError(t, err)
	assert.Empty(t, emptyPage)
}

func TestNatsEventStore_GetAggregateVersion(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_version",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 保存测试事件
	aggregateID := "test-aggregate-id-version"
	events := []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-1",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TypeA",
			Version:       1,
			Timestamp:     time.Now(),
		},
		&mockDomainEvent{
			ID:            "event-2",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TypeB",
			Version:       2,
			Timestamp:     time.Now(),
		},
	}

	err = eventsStore.SaveEvents(ctx, aggregateID, events, -1)
	assert.NoError(t, err)

	// 测试获取聚合版本
	version, err := eventsStore.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 2, version)

	// 测试不存在的聚合
	nonExistentVersion, err := eventsStore.GetAggregateVersion(ctx, "non-existent-aggregate")
	assert.NoError(t, err)
	assert.Equal(t, 0, nonExistentVersion)
}

func TestNatsEventStore_Load(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_load",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 保存测试事件
	aggregateID := "test-aggregate-id-load"
	events := []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-1",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TypeA",
			Version:       1,
			Timestamp:     time.Now(),
		},
		&mockDomainEvent{
			ID:            "event-2",
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     "TypeB",
			Version:       2,
			Timestamp:     time.Now(),
		},
	}

	err = eventsStore.SaveEvents(ctx, aggregateID, events, -1)
	assert.NoError(t, err)

	// 创建模拟聚合根
	mockAggRoot := &mockAggregateRoot{
		ID:   aggregateID,
		Type: "TestAggregate",
	}

	// 设置模拟方法的返回值
	mockAggRoot.On("LoadFromSnapshot", mock.Anything).Return(nil)
	mockAggRoot.On("LoadFromEvents", mock.Anything).Return(nil) // 使用Anyting匹配任意events参数

	// 测试加载聚合
	err = eventsStore.Load(ctx, aggregateID, mockAggRoot)
	assert.NoError(t, err)

	// 验证方法调用
	mockAggRoot.AssertExpectations(t)
}

func TestNatsEventStore_SaveSnapshotAndGetLatestSnapshot(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_snapshot",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 测试保存快照
	aggregateID := "test-aggregate-id-snapshot"
	snapshot := map[string]interface{}{
		"id":   aggregateID,
		"name": "test-snapshot",
	}
	version := 5

	err = eventsStore.SaveSnapshot(ctx, aggregateID, snapshot, version)
	assert.NoError(t, err)

	// 测试获取最新快照
	savedSnapshot, savedVersion, err := eventsStore.GetLatestSnapshot(ctx, aggregateID)
	assert.NoError(t, err)
	assert.NotNil(t, savedSnapshot)
	assert.GreaterOrEqual(t, savedVersion, 0)

	// 测试获取不存在的快照
	nonExistentSnapshot, nonExistentVersion, err := eventsStore.GetLatestSnapshot(ctx, "non-existent-aggregate")
	assert.NoError(t, err)
	assert.Nil(t, nonExistentSnapshot)
	assert.Equal(t, 0, nonExistentVersion)
}

func TestNatsEventStore_SaveEventsBatch(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_batch",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 准备批量事件
	batchEvents := make(map[string][]entity.DomainEvent)
	batchEvents["aggregate-1"] = []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-1-1",
			AggregateID:   "aggregate-1",
			AggregateType: "TestAggregate",
			EventType:     "TypeA",
			Version:       1,
			Timestamp:     time.Now(),
		},
	}
	batchEvents["aggregate-2"] = []entity.DomainEvent{
		&mockDomainEvent{
			ID:            "event-2-1",
			AggregateID:   "aggregate-2",
			AggregateType: "TestAggregate",
			EventType:     "TypeB",
			Version:       1,
			Timestamp:     time.Now(),
		},
		&mockDomainEvent{
			ID:            "event-2-2",
			AggregateID:   "aggregate-2",
			AggregateType: "TestAggregate",
			EventType:     "TypeC",
			Version:       2,
			Timestamp:     time.Now(),
		},
	}

	// 测试批量保存
	err = eventsStore.SaveEventsBatch(ctx, batchEvents)
	assert.NoError(t, err)

	// 验证批量保存是否成功
	events1, err := eventsStore.GetEvents(ctx, "aggregate-1")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(events1))

	events2, err := eventsStore.GetEvents(ctx, "aggregate-2")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(events2))
}

func TestNatsEventStore_Close(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_close",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}

	// 测试关闭连接
	err = eventsStore.Close()
	assert.NoError(t, err)
}

func TestNatsEventStore_Concurrency(t *testing.T) {
	ctx := context.Background()

	// 启动NATS测试容器
	natsContainer, err := natstestcontainers.RunContainer(ctx)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// 获取NATS连接URL
	natsURL, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("获取NATS连接URL失败: %v", err)
	}

	// 创建配置
	cfg := &config.NATsConfig{
		NATSURL:     natsURL,
		StoreStream: "test_events_concurrency",
	}

	// 创建日志实例
	logConfig := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	logger, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("创建日志实例失败: %v", err)
	}

	// 创建事件存储
	eventsStore, err := NewNatsEventStore(cfg, logger)
	if err != nil {
		t.Fatalf("创建事件存储失败: %v", err)
	}
	defer eventsStore.Close()

	// 并发测试
	aggregateID := "test-aggregate-concurrency"
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	// 启动10个并发goroutine尝试保存事件
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// 获取当前版本
			version, err := eventsStore.GetAggregateVersion(ctx, aggregateID)
			if err != nil {
				errChan <- err
				return
			}

			// 尝试保存事件
			event := &mockDomainEvent{
				ID:            fmt.Sprintf("concurrent-event-%d", index),
				AggregateID:   aggregateID,
				AggregateType: "TestAggregate",
				EventType:     "ConcurrentEvent",
				Version:       version + 1,
				Timestamp:     time.Now(),
			}

			saveErr := eventsStore.SaveEvents(ctx, aggregateID, []entity.DomainEvent{event}, version)
			if saveErr != nil {
				errChan <- saveErr
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// 应该有一些冲突错误，但至少有一个保存成功
	errCount := 0
	for err := range errChan {
		if err != nil {
			errCount++
		}
	}

	// 获取最终保存的事件数量
	savedEvents, err := eventsStore.GetEvents(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Greater(t, len(savedEvents), 0)
	assert.LessOrEqual(t, errCount, 9) // 最多9个错误
}
