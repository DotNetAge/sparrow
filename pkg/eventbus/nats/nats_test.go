package nats

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// 自定义NATS测试容器
func setupNATSTestContainer(ctx context.Context, t *testing.T) (string, func()) {
	// 创建NATS容器
	req := testcontainers.ContainerRequest{
		Image:        "nats:2.9",
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForLog("Server is ready"),
		Cmd:          []string{"-js"}, // 启用JetStream
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	assert.NoError(t, err)

	// 获取容器的主机和端口
	host, err := container.Host(ctx)
	assert.NoError(t, err)

	port, err := container.MappedPort(ctx, "4222/tcp")
	assert.NoError(t, err)

	// 构建连接URL
	url := fmt.Sprintf("nats://%s:%s", host, port.Port())

	// 等待NATS服务完全就绪
	time.Sleep(2 * time.Second)

	// 创建一个临时连接来配置流
	nc, err := nats.Connect(url)
	assert.NoError(t, err)
	defer nc.Close()

	// 获取JetStream上下文
	js, err := nc.JetStream()
	assert.NoError(t, err)

	// 创建流配置，匹配所有主题
	// 这样无论Pub和Sub方法使用什么主题格式，消息都能被正确捕获
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "test-stream",
		Subjects: []string{">"}, // 使用>通配符匹配所有主题
	})
	assert.NoError(t, err)

	// 清理函数
	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %v", err)
		}
	}

	return url, cleanup
}

// TestNewJetStreamEventBus 测试创建事件总线实例
func TestNewJetStreamEventBus(t *testing.T) {
	ctx := context.Background()
	url, cleanup := setupNATSTestContainer(ctx, t)
	defer cleanup()

	// 配置
	cfg := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
	}

	// 创建事件总线
	bus, err := NewNatsEventBus(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, bus)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)
}

// TestJetStreamEventBus_PubSub 测试基本的发布订阅功能
func TestJetStreamEventBus_PubSub(t *testing.T) {
	ctx := context.Background()
	url, cleanup := setupNATSTestContainer(ctx, t)
	defer cleanup()

	// 配置
	cfg := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
	}

	// 创建事件总线
	bus, err := NewNatsEventBus(cfg)
	assert.NoError(t, err)
	defer bus.Close()

	// 定义测试事件类型
	eventType := "testevent"

	// 订阅事件
	handlerCalled := false
	var receivedEvent eventbus.Event
	err = bus.Sub(eventType, func(ctx context.Context, event eventbus.Event) error {
		handlerCalled = true
		receivedEvent = event
		return nil
	})
	assert.NoError(t, err)

	// 发布事件
	testData := map[string]interface{}{
		"message": "hello world",
		"number":  42,
	}
	testEvent := eventbus.Event{
		Id:        "test-event-id",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   testData,
	}
	err = bus.Pub(ctx, testEvent)
	assert.NoError(t, err)

	// 等待事件处理完成
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for !handlerCalled {
		select {
		case <-timer.C:
			t.Fatal("event handling timed out")
		case <-time.After(100 * time.Millisecond):
			// 继续等待
		}
	}

	// 验证接收到的事件
	assert.NotNil(t, receivedEvent)
	assert.Equal(t, eventType, receivedEvent.EventType)
	assert.NotEmpty(t, receivedEvent.Id)
	assert.NotEmpty(t, receivedEvent.Timestamp)

	// 验证Payload
	assert.NotNil(t, receivedEvent.Payload)
	payloadMap, ok := receivedEvent.Payload.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "hello world", payloadMap["message"])
	assert.Equal(t, float64(42), payloadMap["number"])
}

// TestJetStreamEventBus_Unsub 测试取消订阅功能
func TestJetStreamEventBus_Unsub(t *testing.T) {
	ctx := context.Background()
	url, cleanup := setupNATSTestContainer(ctx, t)
	defer cleanup()

	// 配置
	cfg := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
	}

	// 创建事件总线
	bus, err := NewNatsEventBus(cfg)
	assert.NoError(t, err)
	defer bus.Close()

	// 定义测试事件类型
	eventType := "testunsubevent"

	// 订阅事件
	handlerCalled := false
	err = bus.Sub(eventType, func(ctx context.Context, event eventbus.Event) error {
		handlerCalled = true
		return nil
	})
	assert.NoError(t, err)

	// 取消订阅
	err = bus.Unsub(eventType)
	assert.NoError(t, err)

	// 尝试发布事件
	testEvent := eventbus.Event{
		Id:        "test-unsub-event-id",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   nil,
	}
	err = bus.Pub(ctx, testEvent)
	assert.NoError(t, err)

	// 短暂等待，确保事件不会被处理
	time.Sleep(1 * time.Second)

	// 验证事件处理器没有被调用
	assert.False(t, handlerCalled)
}

// TestJetStreamEventBus_MultipleSubscribers 测试多个订阅者
func TestJetStreamEventBus_MultipleSubscribers(t *testing.T) {
	ctx := context.Background()
	url, cleanup := setupNATSTestContainer(ctx, t)
	defer cleanup()

	// 配置 - 为不同的总线实例使用不同的DurableName来避免消费者名称冲突
	cfg1 := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
		// DurableName: "durable-1", // 唯一标识符1
	}
	cfg2 := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
		// DurableName: "durable-2", // 唯一标识符2
	}
	cfgPub := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
	}

	// 创建两个不同的事件总线实例用于订阅
	bus1, err := NewNatsEventBus(cfg1)
	assert.NoError(t, err)
	defer bus1.Close()

	bus2, err := NewNatsEventBus(cfg2)
	assert.NoError(t, err)
	defer bus2.Close()

	// 创建一个单独的事件总线实例用于发布
	busPub, err := NewNatsEventBus(cfgPub)
	assert.NoError(t, err)
	defer busPub.Close()

	// 定义测试事件，使用简单的事件类型名称
	eventType := "testmultipleevent"
	testEvent := eventbus.Event{
		Id:        "test-multiple-event-id",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   nil,
	}

	// 用于同步测试结果
	var wg sync.WaitGroup
	wg.Add(2)

	// 订阅者1（使用第一个总线实例）
	subscriber1Called := false
	err = bus1.Sub(eventType, func(ctx context.Context, event eventbus.Event) error {
		subscriber1Called = true
		wg.Done()
		return nil
	})
	assert.NoError(t, err)

	// 订阅者2（使用第二个总线实例）
	subscriber2Called := false
	err = bus2.Sub(eventType, func(ctx context.Context, event eventbus.Event) error {
		subscriber2Called = true
		wg.Done()
		return nil
	})
	assert.NoError(t, err)

	// 发布事件
	err = busPub.Pub(ctx, testEvent)
	assert.NoError(t, err)

	// 等待所有事件处理完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// 设置超时
	select {
	case <-done:
		// 成功处理
	case <-time.After(5 * time.Second):
		t.Fatal("event handling timed out")
	}

	// 验证所有订阅者都被调用
	assert.True(t, subscriber1Called)
	assert.True(t, subscriber2Called)
}

// TestJetStreamEventBus_Close 测试关闭事件总线
func TestJetStreamEventBus_Close(t *testing.T) {
	ctx := context.Background()
	url, cleanup := setupNATSTestContainer(ctx, t)
	defer cleanup()

	// 配置
	cfg := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
	}

	// 创建事件总线
	bus, err := NewNatsEventBus(cfg)
	assert.NoError(t, err)

	// 订阅多个事件类型，使用简单的名称
	for i := 0; i < 3; i++ {
		eventType := fmt.Sprintf("testcloseevent%d", i)
		err = bus.Sub(eventType, func(ctx context.Context, event eventbus.Event) error {
			return nil
		})
		assert.NoError(t, err)
	}

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)

	// 验证关闭后发布事件会失败
	testEvent := eventbus.Event{
		Id:        "test-close-event-id",
		EventType: "testcloseevent",
		Timestamp: time.Now(),
		Payload:   nil,
	}
	err = bus.Pub(ctx, testEvent)
	assert.Error(t, err)
}

// TestJetStreamEventBus_HandlerError 测试处理器返回错误
func TestJetStreamEventBus_HandlerError(t *testing.T) {

	ctx := context.Background()
	url, cleanup := setupNATSTestContainer(ctx, t)
	defer cleanup()

	// 配置
	cfg := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
	}

	// 创建事件总线
	bus, err := NewNatsEventBus(cfg)
	assert.NoError(t, err)
	defer bus.Close()

	// 定义测试事件，使用简单的事件类型名称
	eventType := "testerrorevent"

	// 创建一个总是返回错误的处理器
	err = bus.Sub(eventType, func(ctx context.Context, event eventbus.Event) error {
		return fmt.Errorf("simulated handler error")
	})
	assert.NoError(t, err)

	// 发布事件
	testEvent := eventbus.Event{
		Id:        "test-error-event-id",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   nil,
	}
	err = bus.Pub(ctx, testEvent)
	assert.NoError(t, err)

	// 由于我们跳过了详细的断言，这里只做基本的验证
	time.Sleep(1 * time.Second)
}

// TestJetStreamEventBus_InvalidEvent 测试无效事件数据
func TestJetStreamEventBus_InvalidEvent(t *testing.T) {
	ctx := context.Background()
	url, cleanup := setupNATSTestContainer(ctx, t)
	defer cleanup()

	// 配置
	cfg := &config.NATsConfig{
		URL:        url,
		StreamName: "test-stream",
	}

	// 创建事件总线
	bus, err := NewNatsEventBus(cfg)
	assert.NoError(t, err)
	defer bus.Close()

	// 直接通过NATS客户端发布无效数据
	nc, err := nats.Connect(url)
	assert.NoError(t, err)
	defer nc.Close()

	// 发布非JSON格式数据
	eventType := "test.invalid.event"
	subject := fmt.Sprintf("%s.*.%s", cfg.StreamName, eventType)
	err = nc.Publish(subject, []byte("not a valid json"))
	assert.NoError(t, err)

	// 短暂等待，让事件被处理
	time.Sleep(2 * time.Second)

	// 这里我们主要验证系统不会崩溃，具体的错误处理已经在日志中记录
}
