package eventstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// MemoryTestMockAggregateRoot 用于测试的mock聚合根实现
type MemoryTestMockAggregateRoot struct {
	ID            string
	Version       int
	EventsApplied int
	SnapshotData  interface{}
	AggregateType string

	// 用于测试错误情况
	ShouldFailApplyEvent       bool
	ShouldFailLoadFromEvents   bool
	ShouldFailLoadFromSnapshot bool
}

func NewMemoryTestMockAggregateRoot(id string) *MemoryTestMockAggregateRoot {
	return &MemoryTestMockAggregateRoot{
		ID:            id,
		Version:       0,
		EventsApplied: 0,
		AggregateType: "MockAggregate",
	}
}

// 实现Entity接口
func (m *MemoryTestMockAggregateRoot) GetID() string {
	return m.ID
}

func (m *MemoryTestMockAggregateRoot) SetID(id string) {
	m.ID = id
}

func (m *MemoryTestMockAggregateRoot) GetCreatedAt() time.Time {
	return time.Now()
}

func (m *MemoryTestMockAggregateRoot) GetUpdatedAt() time.Time {
	return time.Now()
}

func (m *MemoryTestMockAggregateRoot) SetUpdatedAt(t time.Time) {
	// 不需要实现
}

// 实现AggregateRoot接口
func (m *MemoryTestMockAggregateRoot) GetAggregateType() string {
	return m.AggregateType
}

func (m *MemoryTestMockAggregateRoot) GetAggregateID() string {
	return m.ID
}

func (m *MemoryTestMockAggregateRoot) GetVersion() int {
	return m.Version
}

func (m *MemoryTestMockAggregateRoot) SetVersion(version int) {
	m.Version = version
}

func (m *MemoryTestMockAggregateRoot) IncrementVersion() {
	m.Version++
}

func (m *MemoryTestMockAggregateRoot) GetUncommittedEvents() []entity.DomainEvent {
	return nil
}

func (m *MemoryTestMockAggregateRoot) MarkEventsAsCommitted() {
	// 不需要实现
}

func (m *MemoryTestMockAggregateRoot) AddUncommittedEvents(events []entity.DomainEvent) {
	// 不需要实现
}

func (m *MemoryTestMockAggregateRoot) LoadFromEvents(events []entity.DomainEvent) error {
	if m.ShouldFailLoadFromEvents {
		return errors.New("failed to load from events")
	}

	for range events {
		m.EventsApplied++
		m.IncrementVersion()
	}
	return nil
}

func (m *MemoryTestMockAggregateRoot) LoadFromSnapshot(snapshot interface{}) error {
	if m.ShouldFailLoadFromSnapshot {
		return errors.New("failed to load from snapshot")
	}

	m.SnapshotData = snapshot
	// 假设快照包含版本信息
	if snapWithVersion, ok := snapshot.(map[string]interface{}); ok {
		if version, ok := snapWithVersion["version"].(float64); ok {
			m.Version = int(version)
		}
	}
	return nil
}

func (m *MemoryTestMockAggregateRoot) ApplyEvent(event interface{}) error {
	if m.ShouldFailApplyEvent {
		return errors.New("failed to apply event")
	}
	m.EventsApplied++
	m.IncrementVersion()
	return nil
}

func (m *MemoryTestMockAggregateRoot) Validate() error {
	return nil
}

func (m *MemoryTestMockAggregateRoot) HandleCommand(cmd interface{}) ([]entity.DomainEvent, error) {
	return nil, nil
}

// NewMemoryTestMockEvent 创建测试用的内存事件，默认使用当前时间
func NewMemoryTestMockEvent(aggregateID, eventType, aggregateType string, version int) *MemoryTestMockEvent {
	return &MemoryTestMockEvent{
		ID:            uuid.New().String(),
		AggregateID:   aggregateID,
		EventType:     eventType,
		AggregateType: aggregateType,
		Version:       version,
		Timestamp:     time.Now(),
	}
}

// NewMemoryTestMockEventWithTime 创建指定时间戳的测试事件
func NewMemoryTestMockEventWithTime(aggregateID, eventType, aggregateType string, version int, timestamp time.Time) *MemoryTestMockEvent {
	return &MemoryTestMockEvent{
		ID:            uuid.New().String(),
		AggregateID:   aggregateID,
		EventType:     eventType,
		AggregateType: aggregateType,
		Version:       version,
		Timestamp:     timestamp,
	}
}

// setupMemoryTestEventStore 创建用于测试的内存事件存储
func setupMemoryTestEventStore(t *testing.T) *MemoryEventStore {
	logCfg := &config.LogConfig{
		Level:  "error", // 只记录错误日志，减少测试输出
		Format: "console",
	}
	log, err := logger.NewLogger(logCfg)
	assert.NoError(t, err)
	es := NewMemoryEventStore(log).(*MemoryEventStore)
	t.Cleanup(func() {
		es.Close()
	})
	return es
}

// TestMemoryEventStore_New 测试创建新的内存事件存储
func TestMemoryEventStore_New(t *testing.T) {
	logCfg := &config.LogConfig{
		Level:  "error",
		Format: "console",
	}
	log, err := logger.NewLogger(logCfg)
	assert.NoError(t, err)
	es := NewMemoryEventStore(log).(*MemoryEventStore)
	assert.NotNil(t, es)
	assert.NotNil(t, es.events)
	assert.NotNil(t, es.snapshots)
	assert.NotNil(t, es.versions)
}

// TestMemoryEventStore_Close 测试关闭事件存储
func TestMemoryEventStore_Close(t *testing.T) {
	es := setupMemoryTestEventStore(t)
	err := es.Close()
	assert.NoError(t, err)
}

// TestMemoryEventStore_SaveEvents 测试保存事件功能
func TestMemoryEventStore_SaveEvents(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID := "test-aggregate-1"

	// 创建测试事件
	events := []entity.DomainEvent{
		NewMemoryTestMockEvent(aggregateID, "TestEvent1", "TestAggregate", 1),
		NewMemoryTestMockEvent(aggregateID, "TestEvent2", "TestAggregate", 2),
	}

	// 测试保存事件
	err := es.SaveEvents(ctx, aggregateID, events, 0)
	assert.NoError(t, err)

	// 验证版本
	version, err := es.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 2, version)

	// 测试并发冲突
	err = es.SaveEvents(ctx, aggregateID, events, 0)
	assert.Error(t, err)
	assert.IsType(t, &errs.EventStoreError{}, err)
}

// TestMemoryEventStore_GetEvents 测试获取事件功能
func TestMemoryEventStore_GetEvents(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID := "test-aggregate-2"

	// 保存测试事件
	events := []entity.DomainEvent{
		NewMemoryTestMockEvent(aggregateID, "TestEvent1", "TestAggregate", 1),
		NewMemoryTestMockEvent(aggregateID, "TestEvent2", "TestAggregate", 2),
	}
	es.SaveEvents(ctx, aggregateID, events, 0)

	// 测试获取事件
	retrievedEvents, err := es.GetEvents(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Len(t, retrievedEvents, 2)

	// 测试获取不存在的聚合事件
	retrievedEvents, err = es.GetEvents(ctx, "non-existent-aggregate")
	assert.NoError(t, err)
	assert.Empty(t, retrievedEvents)
}

// TestMemoryEventStore_GetEventsFromVersion 测试从指定版本获取事件
func TestMemoryEventStore_GetEventsFromVersion(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID := "test-aggregate-3"

	// 保存测试事件
	events := []entity.DomainEvent{
		NewMemoryTestMockEvent(aggregateID, "TestEvent1", "TestAggregate", 1),
		NewMemoryTestMockEvent(aggregateID, "TestEvent2", "TestAggregate", 2),
		NewMemoryTestMockEvent(aggregateID, "TestEvent3", "TestAggregate", 3),
	}
	es.SaveEvents(ctx, aggregateID, events, 0)

	// 测试从指定版本获取事件
	retrievedEvents, err := es.GetEventsFromVersion(ctx, aggregateID, 2)
	assert.NoError(t, err)
	assert.Len(t, retrievedEvents, 2)

	// 测试从不存在的版本获取事件
	retrievedEvents, err = es.GetEventsFromVersion(ctx, "non-existent-aggregate", 1)
	assert.NoError(t, err)
	assert.Empty(t, retrievedEvents)
}

// TestMemoryEventStore_GetEventsByType 测试按类型获取事件
func TestMemoryEventStore_GetEventsByType(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID1 := "test-aggregate-4"
	aggregateID2 := "test-aggregate-5"

	// 保存测试事件
	events1 := []entity.DomainEvent{
		NewMemoryTestMockEvent(aggregateID1, "TypeA", "TestAggregate", 1),
		NewMemoryTestMockEvent(aggregateID1, "TypeB", "TestAggregate", 2),
	}
	events2 := []entity.DomainEvent{
		NewMemoryTestMockEvent(aggregateID2, "TypeA", "TestAggregate", 1),
		NewMemoryTestMockEvent(aggregateID2, "TypeC", "TestAggregate", 2),
	}
	es.SaveEvents(ctx, aggregateID1, events1, 0)
	es.SaveEvents(ctx, aggregateID2, events2, 0)

	// 测试按类型获取事件
	typeAEvents, err := es.GetEventsByType(ctx, "TypeA")
	assert.NoError(t, err)
	assert.Len(t, typeAEvents, 2)

	// 测试获取不存在的类型事件
	typeDEvents, err := es.GetEventsByType(ctx, "TypeD")
	assert.NoError(t, err)
	assert.Empty(t, typeDEvents)
}

// TestMemoryEventStore_Load 测试加载聚合根
func TestMemoryEventStore_Load(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID := "test-aggregate-6"

	// 保存测试事件
	events := []entity.DomainEvent{
		NewMemoryTestMockEvent(aggregateID, "TestEvent1", "TestAggregate", 1),
		NewMemoryTestMockEvent(aggregateID, "TestEvent2", "TestAggregate", 2),
	}
	es.SaveEvents(ctx, aggregateID, events, 0)

	// 创建聚合根
	aggregate := NewMemoryTestMockAggregateRoot(aggregateID)

	// 测试加载聚合根
	err := es.Load(ctx, aggregateID, aggregate)
	assert.NoError(t, err)
	assert.Equal(t, 2, aggregate.EventsApplied)
	assert.Equal(t, 2, aggregate.Version)

	// 测试加载不存在的聚合根
	aggregate2 := NewMemoryTestMockAggregateRoot("non-existent-aggregate")
	err = es.Load(ctx, "non-existent-aggregate", aggregate2)
	assert.NoError(t, err)
	assert.Equal(t, 0, aggregate2.EventsApplied)
}

// TestMemoryEventStore_Load_Failure 测试加载失败情况
func TestMemoryEventStore_Load_Failure(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID := "test-aggregate-failure"

	// 保存测试事件
	events := []entity.DomainEvent{
		NewMemoryTestMockEvent(aggregateID, "TestEvent1", "TestAggregate", 1),
	}
	es.SaveEvents(ctx, aggregateID, events, 0)

	// 测试应用事件失败
	aggregate := NewMemoryTestMockAggregateRoot(aggregateID)
	aggregate.ShouldFailLoadFromEvents = true
	err := es.Load(ctx, aggregateID, aggregate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load from events")

	// 测试快照加载失败
	es.SaveSnapshot(ctx, aggregateID, map[string]interface{}{"version": 1}, 1)
	aggregate2 := NewMemoryTestMockAggregateRoot(aggregateID)
	aggregate2.ShouldFailLoadFromSnapshot = true
	err = es.Load(ctx, aggregateID, aggregate2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load from snapshot")
}

// TestMemoryEventStore_GetAggregateVersion 测试获取聚合根版本
func TestMemoryEventStore_GetAggregateVersion(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID := "test-aggregate-7"

	// 保存测试事件
	events := []entity.DomainEvent{
		NewMemoryTestMockEvent(aggregateID, "TestEvent1", "TestAggregate", 1),
	}
	es.SaveEvents(ctx, aggregateID, events, 0)

	// 测试获取版本
	version, err := es.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 1, version)

	// 测试获取不存在的聚合根版本
	version, err = es.GetAggregateVersion(ctx, "non-existent-aggregate")
	assert.NoError(t, err)
	assert.Equal(t, 0, version)
}

// TestMemoryEventStore_SaveSnapshot 测试保存快照
func TestMemoryEventStore_SaveSnapshot(t *testing.T) {
	ctx := context.Background()
	es := setupMemoryTestEventStore(t)
	aggregateID := "test-aggregate-snapshot"

	// 初始状态下应该没有快照
	snapshot, _, err := es.GetLatestSnapshot(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Nil(t, snapshot)

	// 先创建一些事件来增加聚合根版本
	events := []entity.DomainEvent{}
	for i := 0; i < 5; i++ {
		event := NewMemoryTestMockEvent(aggregateID, "TestAggregate", fmt.Sprintf("event-%d", i), i+1)
		events = append(events, event)
	}
	err = es.SaveEvents(ctx, aggregateID, events, 0)
	assert.NoError(t, err)

	// 保存快照后应该有快照
	snapshotData := map[string]interface{}{"name": "test", "version": 5}
	err = es.SaveSnapshot(ctx, aggregateID, snapshotData, 5)
	assert.NoError(t, err)

	// 检查是否存在快照
	snapshot, version, err := es.GetLatestSnapshot(ctx, aggregateID)
	assert.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.Equal(t, 5, version)

	// 验证快照数据
	data, ok := snapshot.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test", data["name"])
}

// TestMemoryEventStore_GetLatestSnapshot 测试获取快照
func TestMemoryEventStore_GetLatestSnapshot(t *testing.T) {
	ctx := context.Background()
	es := setupMemoryTestEventStore(t)
	aggregateID := "test-aggregate-get-snapshot"

	// 初始状态下应该没有快照
	snapshot, _, err := es.GetLatestSnapshot(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Nil(t, snapshot)

	// 先创建一些事件来增加聚合根版本
	events := []entity.DomainEvent{}
	for i := 0; i < 5; i++ {
		event := NewMemoryTestMockEvent(aggregateID, "TestAggregate", fmt.Sprintf("event-%d", i), i+1)
		events = append(events, event)
	}
	err = es.SaveEvents(ctx, aggregateID, events, 0)
	assert.NoError(t, err)

	// 保存快照
	snapshotData := map[string]interface{}{"field": "value"}
	err = es.SaveSnapshot(ctx, aggregateID, snapshotData, 5)
	assert.NoError(t, err)

	// 获取快照
	snapshot, version, err := es.GetLatestSnapshot(ctx, aggregateID)
	assert.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.Equal(t, 5, version)
}

// TestMemoryEventStore_DeleteSnapshot 测试删除快照
func TestMemoryEventStore_SaveSnapshot_NoDelete(t *testing.T) {
	ctx := context.Background()
	es := setupMemoryTestEventStore(t)
	aggregateID := "test-aggregate-delete-snapshot"

	// 先创建一些事件来增加聚合根版本
	events := []entity.DomainEvent{}
	for i := 0; i < 6; i++ {
		event := NewMemoryTestMockEvent(aggregateID, "TestAggregate", fmt.Sprintf("event-%d", i), i+1)
		events = append(events, event)
	}
	err := es.SaveEvents(ctx, aggregateID, events, 0)
	assert.NoError(t, err)

	// 保存快照（版本5）
	snapshotData := map[string]interface{}{"field": "value"}
	err = es.SaveSnapshot(ctx, aggregateID, snapshotData, 5)
	assert.NoError(t, err)

	// 验证快照存在
	snapshot, _, err := es.GetLatestSnapshot(ctx, aggregateID)
	assert.NoError(t, err)
	assert.NotNil(t, snapshot)

	// 注意：MemoryEventStore 不支持删除快照功能
	// 为了测试，可以通过保存一个空快照或低版本快照来覆盖
	emptySnapshot := map[string]interface{}{}
	err = es.SaveSnapshot(ctx, aggregateID, emptySnapshot, 6)
	assert.NoError(t, err)

	// 验证快照被更新
	updatedSnapshot, _, err := es.GetLatestSnapshot(ctx, aggregateID)
	assert.NoError(t, err)
	assert.NotNil(t, updatedSnapshot)
	assert.NotEqual(t, snapshotData, updatedSnapshot)
}

// TestMemoryEventStore_SaveEventsBatch 测试批量保存事件
func TestMemoryEventStore_SaveEventsBatch(t *testing.T) {
	ctx := context.Background()
	es := setupMemoryTestEventStore(t)

	// 创建批量事件 - 使用 map[string][]entity.DomainEvent 类型
	batch := make(map[string][]entity.DomainEvent)
	batch["batch-aggregate-1"] = []entity.DomainEvent{
		NewMemoryTestMockEvent("batch-aggregate-1", "BatchEvent1", "TestAggregate", 1),
		NewMemoryTestMockEvent("batch-aggregate-1", "BatchEvent2", "TestAggregate", 2),
	}
	batch["batch-aggregate-2"] = []entity.DomainEvent{
		NewMemoryTestMockEvent("batch-aggregate-2", "BatchEvent3", "TestAggregate", 1),
	}

	// 批量保存事件
	err := es.SaveEventsBatch(ctx, batch)
	assert.NoError(t, err)

	// 验证事件是否保存成功
	events1, err := es.GetEvents(ctx, "batch-aggregate-1")
	assert.NoError(t, err)
	assert.Len(t, events1, 2)

	events2, err := es.GetEvents(ctx, "batch-aggregate-2")
	assert.NoError(t, err)
	assert.Len(t, events2, 1)

	// 验证版本号
	version1, err := es.GetAggregateVersion(ctx, "batch-aggregate-1")
	assert.NoError(t, err)
	assert.Equal(t, 2, version1)

	version2, err := es.GetAggregateVersion(ctx, "batch-aggregate-2")
	assert.NoError(t, err)
	assert.Equal(t, 1, version2)
}

// TestMemoryEventStore_GetEventsByTimeRange 测试按时间范围获取事件
func TestMemoryEventStore_GetEventsByTimeRange(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID := "test-aggregate-time-range"

	// 创建时间点
	now := time.Now()
	before := now.Add(-5 * time.Minute)
	after := now.Add(5 * time.Minute)

	// 保存测试事件 - 使用特定的时间戳
	events := []entity.DomainEvent{
		// 第一个事件在before时间点
		NewMemoryTestMockEventWithTime(aggregateID, "TestAggregate", "TestEvent1", 1, before),
		// 第二个事件在now时间点
		NewMemoryTestMockEventWithTime(aggregateID, "TestAggregate", "TestEvent2", 2, now),
		// 第三个事件在after.Add(2*time.Hour)时间点，超出查询范围
		NewMemoryTestMockEventWithTime(aggregateID, "TestAggregate", "TestEvent3", 3, after.Add(2*time.Hour)),
	}
	err := es.SaveEvents(ctx, aggregateID, events, 0)
	assert.NoError(t, err)

	// 测试获取时间范围内的事件
	rangeEvents, err := es.GetEventsByTimeRange(ctx, aggregateID, before, now)
	assert.NoError(t, err)
	assert.Len(t, rangeEvents, 2) // 应该包含before和now的事件

	// 测试获取空时间范围的事件
	rangeEvents, err = es.GetEventsByTimeRange(ctx, aggregateID, after, after.Add(time.Hour))
	assert.NoError(t, err)
	assert.Empty(t, rangeEvents)
}

// TestMemoryEventStore_GetEventsWithPagination 测试分页获取事件
func TestMemoryEventStore_GetEventsWithPagination(t *testing.T) {
	es := setupMemoryTestEventStore(t)

	ctx := context.Background()
	aggregateID := "test-aggregate-pagination"

	// 保存多个测试事件
	events := make([]entity.DomainEvent, 10)
	for i := 0; i < 10; i++ {
		events[i] = NewMemoryTestMockEvent(aggregateID, fmt.Sprintf("TestEvent%d", i+1), "TestAggregate", i+1)
	}
	es.SaveEvents(ctx, aggregateID, events, 0)

	// 测试第一页，每页3条
	page1Events, err := es.GetEventsWithPagination(ctx, aggregateID, 3, 0)
	assert.NoError(t, err)
	assert.Len(t, page1Events, 3)

	// 测试第二页，每页3条
	page2Events, err := es.GetEventsWithPagination(ctx, aggregateID, 3, 3)
	assert.NoError(t, err)
	assert.Len(t, page2Events, 3)

	// 测试超出范围的分页
	emptyPageEvents, err := es.GetEventsWithPagination(ctx, aggregateID, 3, 100)
	assert.NoError(t, err)
	assert.Empty(t, emptyPageEvents)
}

// TestMemoryEventStore_ConcurrentAccess 测试并发访问
func TestMemoryEventStore_ConcurrentAccess(t *testing.T) {
	es := setupMemoryTestEventStore(t)
	concurrency := 5
	iterations := 5

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		i := i // 捕获循环变量
		go func(workerID int) {
			defer wg.Done()

			ctx := context.Background()
			aggregateID := fmt.Sprintf("concurrent-aggregate-%d", workerID)
			var events []entity.DomainEvent

			// 创建事件
			for j := 0; j < iterations; j++ {
				event := NewMemoryTestMockEvent(
					aggregateID, 
					fmt.Sprintf("ConcurrentEvent-%d-%d", workerID, j), 
					"TestAggregate", 
					j+1,
				)
				events = append(events, event)
			}

			// 一次性保存所有事件，让事件存储自动管理版本
			err := es.SaveEvents(ctx, aggregateID, events, -1) // -1 表示不检查版本
			if err != nil {
				t.Errorf("Worker %d: failed to save events: %v", workerID, err)
				return
			}

			// 读取并验证事件
			storedEvents, err := es.GetEvents(ctx, aggregateID)
			if err != nil {
				t.Errorf("Worker %d: failed to get events: %v", workerID, err)
				return
			}

			if len(storedEvents) != iterations {
				t.Errorf("Worker %d: expected %d events, got %d", workerID, iterations, len(storedEvents))
				return
			}
		}(i)
	}

	wg.Wait()

	// 验证最终状态
	for i := 0; i < concurrency; i++ {
		aggregateID := fmt.Sprintf("concurrent-aggregate-%d", i)
		version, err := es.GetAggregateVersion(context.Background(), aggregateID)
		assert.NoError(t, err)
		assert.Equal(t, iterations, version)
	}
}