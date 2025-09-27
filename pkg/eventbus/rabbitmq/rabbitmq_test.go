package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupRabbitMQContainer 设置RabbitMQ测试容器
func setupRabbitMQContainer(t *testing.T) (testcontainers.Container, *config.RabbitMQConfig) {
	t.Helper()

	// 创建RabbitMQ容器
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3-management",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		WaitingFor:   wait.ForLog("Server startup complete"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("启动RabbitMQ容器失败: %v", err)
	}

	// 获取容器IP和端口
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("获取容器主机失败: %v", err)
	}

	port, err := container.MappedPort(ctx, "5672")
	if err != nil {
		t.Fatalf("获取容器端口失败: %v", err)
	}

	// 配置RabbitMQ连接信息
	cfg := &config.RabbitMQConfig{
		Host:     host,
		Port:     port.Int(),
		Username: "guest",
		Password: "guest",
		VHost:    "/",
		Exchange: "test-events",
	}

	return container, cfg
}

// TestNewRabbitMQEventBus 测试创建RabbitMQ事件总线
func TestNewRabbitMQEventBus(t *testing.T) {
	container, cfg := setupRabbitMQContainer(t)
	defer container.Terminate(context.Background())

	bus, err := NewRabbitMQEventBus(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, bus)

bus.Close()
}

// TestRabbitMQEventBus_SubAndPub 测试订阅和发布事件
func TestRabbitMQEventBus_SubAndPub(t *testing.T) {
	container, cfg := setupRabbitMQContainer(t)
	defer container.Terminate(context.Background())

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(cfg)
	assert.NoError(t, err)
	defer bus.Close()

	// 准备事件数据
	const eventType = "test.event"
	event := eventbus.Event{
		Id:        "test-event-1",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value"},
	}

	// 接收事件的通道
	eventReceived := make(chan struct{}, 1)

	// 订阅事件
	err = bus.Sub(eventType, func(ctx context.Context, evt eventbus.Event) error {
		assert.Equal(t, eventType, evt.EventType)
		assert.Equal(t, event.Id, evt.Id)
		eventReceived <- struct{}{}
		return nil
	})
	assert.NoError(t, err)

	// 发布事件
	ctx := context.Background()
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待事件接收
	select {
	case <-eventReceived:
		// 成功接收到事件
	case <-time.After(5 * time.Second):
		t.Fatalf("未能在超时时间内接收到事件")
	}
}

// TestRabbitMQEventBus_MultipleSubscribers 测试多个订阅者接收同一事件
func TestRabbitMQEventBus_MultipleSubscribers(t *testing.T) {
	container, cfg := setupRabbitMQContainer(t)
	defer container.Terminate(context.Background())

	// 创建两个事件总线实例
	bus1, err := NewRabbitMQEventBus(cfg)
	assert.NoError(t, err)
	defer bus1.Close()

	bus2, err := NewRabbitMQEventBus(cfg)
	assert.NoError(t, err)
	defer bus2.Close()

	// 准备事件数据
	const eventType = "test.event.multiple"
	event := eventbus.Event{
		Id:        "test-event-2",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value2"},
	}

	// 接收计数
	var wg sync.WaitGroup
	wg.Add(2)

	// 第一个订阅者
	err = bus1.Sub(eventType, func(ctx context.Context, evt eventbus.Event) error {
		defer wg.Done()
		assert.Equal(t, eventType, evt.EventType)
		return nil
	})
	assert.NoError(t, err)

	// 第二个订阅者
	err = bus2.Sub(eventType, func(ctx context.Context, evt eventbus.Event) error {
		defer wg.Done()
		assert.Equal(t, eventType, evt.EventType)
		return nil
	})
	assert.NoError(t, err)

	// 发布事件
	ctx := context.Background()
	err = bus1.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待所有订阅者处理完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// 等待事件处理完成
	select {
	case <-done:
		// 所有订阅者都处理了事件
	case <-time.After(10 * time.Second):
		t.Fatalf("超时：并非所有订阅者都接收到了事件")
	}
}

// TestRabbitMQEventBus_Unsub 测试取消订阅
func TestRabbitMQEventBus_Unsub(t *testing.T) {
	container, cfg := setupRabbitMQContainer(t)
	defer container.Terminate(context.Background())

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(cfg)
	assert.NoError(t, err)
	defer bus.Close()

	// 事件类型
	const eventType = "test.event.unsub"

	// 订阅事件的标志
	isCalled := false

	// 订阅事件
	err = bus.Sub(eventType, func(ctx context.Context, evt eventbus.Event) error {
		isCalled = true
		return nil
	})
	assert.NoError(t, err)

	// 取消订阅
	err = bus.Unsub(eventType)
	assert.NoError(t, err)

	// 发布事件
	event := eventbus.Event{
		Id:        "test-event-3",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value3"},
	}

	ctx := context.Background()
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待一会儿，确认没有收到事件
	time.Sleep(1 * time.Second)
	assert.False(t, isCalled, "取消订阅后仍接收到了事件")
}

// TestRabbitMQEventBus_Close 测试关闭事件总线
func TestRabbitMQEventBus_Close(t *testing.T) {
	container, cfg := setupRabbitMQContainer(t)
	defer container.Terminate(context.Background())

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(cfg)
	assert.NoError(t, err)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)

	// 尝试在已关闭的总线上发布事件
	event := eventbus.Event{
		Id:        "test-event-4",
		EventType: "test.event.close",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value4"},
	}

	ctx := context.Background()
	err = bus.Pub(ctx, event)
	assert.Error(t, err)
}

// TestRabbitMQEventBus_PubToClosedBus 测试向已关闭的事件总线发布事件
func TestRabbitMQEventBus_PubToClosedBus(t *testing.T) {
	container, cfg := setupRabbitMQContainer(t)
	defer container.Terminate(context.Background())

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(cfg)
	assert.NoError(t, err)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)

	// 尝试在已关闭的总线上发布事件
	event := eventbus.Event{
		Id:        "test-event-5",
		EventType: "test.event.closed",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value5"},
	}

	ctx := context.Background()
	err = bus.Pub(ctx, event)
	assert.Error(t, err)
}

// TestRabbitMQEventBus_HandlerError 测试处理器返回错误
func TestRabbitMQEventBus_HandlerError(t *testing.T) {
	container, cfg := setupRabbitMQContainer(t)
	defer container.Terminate(context.Background())

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(cfg)
	assert.NoError(t, err)
	defer bus.Close()

	// 事件类型
	const eventType = "test.event.error"

	// 订阅事件并返回错误
	err = bus.Sub(eventType, func(ctx context.Context, evt eventbus.Event) error {
		return fmt.Errorf("故意的错误")
	})
	assert.NoError(t, err)

	// 发布事件
	event := eventbus.Event{
		Id:        "test-event-6",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value6"},
	}

	ctx := context.Background()
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待一会儿，确保处理器有时间执行
	time.Sleep(500 * time.Millisecond)

	// 即使处理器返回错误，发布操作也应该成功
	// 这里只是验证代码不会崩溃
}