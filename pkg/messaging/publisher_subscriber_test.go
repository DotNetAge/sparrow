package messaging

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// TestEvent - 测试用的领域事件
type TestEvent struct {
	ID            string
	AggregateID   string
	EventType     string
	AggregateType string
	Version       int
	Timestamp     time.Time
	Message       string
}

// GetEventID 返回事件ID
func (e *TestEvent) GetEventID() string {
	return e.ID
}

// GetEventType 返回事件类型
func (e *TestEvent) GetEventType() string {
	return e.EventType
}

// GetCreatedAt 返回事件创建时间
func (e *TestEvent) GetCreatedAt() time.Time {
	return e.Timestamp
}

// GetAggregateID 返回聚合根ID
func (e *TestEvent) GetAggregateID() string {
	return e.AggregateID
}

// GetAggregateType 返回聚合根类型
func (e *TestEvent) GetAggregateType() string {
	return e.AggregateType
}

// GetVersion 返回事件版本
func (e *TestEvent) GetVersion() int {
	return e.Version
}

// TestPublisherSubscriberCommunication - 测试 Publisher 和 Subscriber 之间的通信
func TestPublisherSubscriberCommunication(t *testing.T) {
	// 创建测试上下文
	ctx := context.Background()

	// 创建内存事件总线
	bus := eventbus.NewMemoryEventBus()

	// 创建日志记录器配置
	logConfig := &config.LogConfig{
		Level:  "info",
		Format: "console",
	}
	log, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 服务名称
	serviceName := "test-service"

	// 创建测试事件
	testEvent := &TestEvent{
		ID:            "test-event-id",
		AggregateID:   "test-aggregate-id",
		EventType:     "TestEvent",
		AggregateType: "TestAggregate",
		Version:       1,
		Timestamp:     time.Now(),
		Message:       "Hello, this is a test message",
	}

	// 创建 Publisher
	publisher := NewEventPublisher(nil, bus, serviceName)

	// 创建两个 Subscriber 实例
	subscriber1 := &EventSubscriber[*TestEvent]{}
	subscriber1.Init(serviceName, "TestAggregate", bus, log)

	subscriber2 := &EventSubscriber[*TestEvent]{}
	subscriber2.Init(serviceName, "TestAggregate", bus, log)

	// 用于同步的等待组
	var wg sync.WaitGroup

	// 用于验证事件接收的通道
	eventReceived1 := make(chan *TestEvent, 1)
	eventReceived2 := make(chan *TestEvent, 1)

	// 设置超时
	timeout := time.After(5 * time.Second)

	// 添加处理器到 subscriber1
	wg.Add(1)
	handlerErr1 := subscriber1.AddHandler(func(ctx context.Context, event *TestEvent) error {
		defer wg.Done()
		eventReceived1 <- event
		return nil
	})
	if handlerErr1 != nil {
		t.Fatalf("Failed to add handler to subscriber1: %v", handlerErr1)
	}

	// 添加处理器到 subscriber2
	wg.Add(1)
	handlerErr2 := subscriber2.AddHandler(func(ctx context.Context, event *TestEvent) error {
		defer wg.Done()
		eventReceived2 <- event
		return nil
	})
	if handlerErr2 != nil {
		t.Fatalf("Failed to add handler to subscriber2: %v", handlerErr2)
	}

	// 模拟聚合根
	mockAggregate := &mockAggregateRoot{
		id:                "test-aggregate-id",
		version:           0,
		uncommittedEvents: []entity.DomainEvent{testEvent},
	}

	// 发布事件
	publishErr := publisher.PublishUncommittedEvents(ctx, mockAggregate)
	if publishErr != nil {
		t.Fatalf("Failed to publish events: %v", publishErr)
	}

	// 等待事件处理完成或超时
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// 验证两个订阅者都接收到了事件
		select {
		case receivedEvent1 := <-eventReceived1:
			if receivedEvent1.Message != testEvent.Message {
				t.Errorf("Subscriber1 received incorrect event message: got %v, want %v", 
					receivedEvent1.Message, testEvent.Message)
			}
		case <-timeout:
			t.Fatalf("Subscriber1 did not receive event within timeout")
		}

		select {
		case receivedEvent2 := <-eventReceived2:
			if receivedEvent2.Message != testEvent.Message {
				t.Errorf("Subscriber2 received incorrect event message: got %v, want %v", 
					receivedEvent2.Message, testEvent.Message)
			}
		case <-timeout:
			t.Fatalf("Subscriber2 did not receive event within timeout")
		}

	case <-timeout:
		t.Fatalf("Event processing timed out")
	}

	// 清理资源
	bus.Close()
}

// mockAggregateRoot - 用于测试的模拟聚合根
type mockAggregateRoot struct {
	id                string
	version           int
	uncommittedEvents []entity.DomainEvent
}

// GetID - 获取聚合根ID
func (m *mockAggregateRoot) GetID() string {
	return m.id
}

// SetID - 设置聚合根ID
func (m *mockAggregateRoot) SetID(id string) {
	m.id = id
}

// GetAggregateType - 获取聚合类型
func (m *mockAggregateRoot) GetAggregateType() string {
	return "TestAggregate"
}

// GetAggregateID - 获取聚合ID
func (m *mockAggregateRoot) GetAggregateID() string {
	return m.id
}

// GetVersion - 获取聚合根版本
func (m *mockAggregateRoot) GetVersion() int {
	return m.version
}

// SetVersion - 设置聚合根版本
func (m *mockAggregateRoot) SetVersion(version int) {
	m.version = version
}

// IncrementVersion - 递增版本号
func (m *mockAggregateRoot) IncrementVersion() {
	m.version++
}

// GetCreatedAt - 获取创建时间
func (m *mockAggregateRoot) GetCreatedAt() time.Time {
	return time.Time{}
}

// GetUpdatedAt - 获取更新时间
func (m *mockAggregateRoot) GetUpdatedAt() time.Time {
	return time.Time{}
}

// SetUpdatedAt - 设置更新时间
func (m *mockAggregateRoot) SetUpdatedAt(t time.Time) {
	// 仅用于测试，无需实现具体逻辑
}

// GetUncommittedEvents - 获取未提交的事件
func (m *mockAggregateRoot) GetUncommittedEvents() []entity.DomainEvent {
	return m.uncommittedEvents
}

// MarkEventsAsCommitted - 标记事件为已提交
func (m *mockAggregateRoot) MarkEventsAsCommitted() {
	m.uncommittedEvents = nil
}

// AddUncommittedEvents - 添加未提交事件
func (m *mockAggregateRoot) AddUncommittedEvents(events []entity.DomainEvent) {
	m.uncommittedEvents = append(m.uncommittedEvents, events...)
}

// LoadFromEvents - 从事件加载状态
func (m *mockAggregateRoot) LoadFromEvents(events []entity.DomainEvent) error {
	return nil
}

// LoadFromSnapshot - 从快照加载状态
func (m *mockAggregateRoot) LoadFromSnapshot(snapshot interface{}) error {
	return nil
}

// ApplyEvent - 应用单个事件
func (m *mockAggregateRoot) ApplyEvent(event interface{}) error {
	return nil
}

// Validate - 验证聚合状态
func (m *mockAggregateRoot) Validate() error {
	return nil
}

// HandleCommand - 处理命令
func (m *mockAggregateRoot) HandleCommand(cmd interface{}) ([]entity.DomainEvent, error) {
	return nil, nil
}