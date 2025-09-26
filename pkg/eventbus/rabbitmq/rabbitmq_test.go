package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
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
	rabbitConfig := &config.RabbitMQConfig{
		Host:     host,
		Port:     port.Int(),
		Username: "guest",
		Password: "guest",
		VHost:    "/",
		Exchange: "test_events",
	}

	// 等待RabbitMQ完全就绪
	time.Sleep(3 * time.Second)

	return container, rabbitConfig
}

// TestRabbitMQEventBus_Create 测试创建RabbitMQ事件总线
func TestRabbitMQEventBus_Create(t *testing.T) {
	ctx := context.Background()
	container, config := setupRabbitMQContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)
	assert.NotNil(t, bus)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)
}

// TestRabbitMQEventBus_PubSub 测试发布和订阅功能
func TestRabbitMQEventBus_PubSub(t *testing.T) {
	ctx := context.Background()
	container, config := setupRabbitMQContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)
	defer bus.Close()

	// 准备事件数据
	const eventType = "test.event"
	event := &entity.GenericEvent{
		Id:        "test-event-1",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value"},
	}

	// 接收事件的通道
	eventReceived := make(chan struct{}, 1)

	// 订阅事件
	err = bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		assert.Equal(t, eventType, evt.GetEventType())
		assert.Equal(t, event.GetEventID(), evt.GetEventID())
		eventReceived <- struct{}{}
		return nil
	})
	assert.NoError(t, err)

	// 发布事件
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待事件接收
	select {
	case <-eventReceived:
		// 成功接收到事件
	case <-time.After(5 * time.Second):
		t.Fatal("超时: 未接收到事件")
	}
}

// TestRabbitMQEventBus_MultipleSubscribers 测试多个订阅者
func TestRabbitMQEventBus_MultipleSubscribers(t *testing.T) {
	ctx := context.Background()
	container, config := setupRabbitMQContainer(t)
	defer container.Terminate(ctx)

	// 创建两个事件总线实例（模拟两个不同的服务）
	bus1, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)
	defer bus1.Close()

	bus2, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)
	defer bus2.Close()

	// 准备事件数据
	const eventType = "test.event.multiple"
	event := &entity.GenericEvent{
		Id:        "test-event-2",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value2"},
	}

	// 接收计数
	var wg sync.WaitGroup
	wg.Add(2)

	// 第一个订阅者
	err = bus1.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		defer wg.Done()
		assert.Equal(t, eventType, evt.GetEventType())
		return nil
	})
	assert.NoError(t, err)

	// 第二个订阅者
	err = bus2.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		defer wg.Done()
		assert.Equal(t, eventType, evt.GetEventType())
		return nil
	})
	assert.NoError(t, err)

	// 发布事件
	err = bus1.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待所有订阅者处理完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 成功
	case <-time.After(5 * time.Second):
		t.Fatal("超时: 未接收到所有事件")
	}
}

// TestRabbitMQEventBus_Unsub 测试取消订阅
func TestRabbitMQEventBus_Unsub(t *testing.T) {
	ctx := context.Background()
	container, config := setupRabbitMQContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)
	defer bus.Close()

	// 准备事件数据
	const eventType = "test.event.unsub"
	event := &entity.GenericEvent{
		Id:        "test-event-3",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value3"},
	}

	// 计数器
	var counter int32 = 0

	// 订阅事件
	err = bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		counter++
		return nil
	})
	assert.NoError(t, err)

	// 发布第一个事件
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待事件处理
	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(1), counter)

	// 取消订阅
	err = bus.Unsub(eventType)
	assert.NoError(t, err)

	// 修改事件ID（避免消息重复）
	event.Id = "test-event-4"

	// 发布第二个事件
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待一段时间，确认没有收到事件
	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(1), counter) // 计数器不应增加
}

// TestRabbitMQEventBus_HandlerError 测试处理器错误
func TestRabbitMQEventBus_HandlerError(t *testing.T) {
	ctx := context.Background()
	container, config := setupRabbitMQContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)
	defer bus.Close()

	// 准备事件数据
	const eventType = "test.event.error"
	event := &entity.GenericEvent{
		Id:        "test-event-5",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value5"},
	}

	// 错误处理器
	err = bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		return fmt.Errorf("故意的错误")
	})
	assert.NoError(t, err)

	// 发布事件（即使处理器返回错误，Pub也应该成功）
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待一段时间，确保处理器执行
	time.Sleep(1 * time.Second)
}

// TestRabbitMQEventBus_InvalidEvent 测试无效事件
func TestRabbitMQEventBus_InvalidEvent(t *testing.T) {
	ctx := context.Background()
	container, config := setupRabbitMQContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)
	defer bus.Close()

	// 准备无效事件数据
	const eventType = "test.event.invalid"
	// 这里我们直接使用amqp客户端发布一个无效格式的消息

	// 创建直接连接到RabbitMQ的客户端
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/%s", 
		config.Username, config.Password, config.Host, config.Port, config.VHost))
	assert.NoError(t, err)
	defer conn.Close()

	// 创建通道
	ch, err := conn.Channel()
	assert.NoError(t, err)
	defer ch.Close()

	// 订阅事件（用于验证我们可以接收到消息，但可能无法正确反序列化）
	err = bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		// 即使消息格式无效，我们也应该能够接收到事件
		return nil
	})
	assert.NoError(t, err)

	// 发布无效格式的消息
	msg := amqp.Publishing{
		ContentType:  "text/plain",
		Body:         []byte("这不是有效的JSON"),
		Timestamp:    time.Now(),
		MessageId:    "invalid-message-1",
		Headers:      amqp.Table{"event_type": eventType},
	}

	err = ch.PublishWithContext(ctx, config.Exchange, eventType, false, false, msg)
	assert.NoError(t, err)

	// 等待一段时间，确保消息被处理
	time.Sleep(1 * time.Second)
}

// TestRabbitMQEventBus_Close 测试关闭事件总线
func TestRabbitMQEventBus_Close(t *testing.T) {
	ctx := context.Background()
	container, config := setupRabbitMQContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)

	// 订阅一个事件类型
	err = bus.Sub("test.event.close", func(ctx context.Context, evt entity.Event) error {
		return nil
	})
	assert.NoError(t, err)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)

	// 验证关闭后无法发布事件
	event := &entity.GenericEvent{
		Id:        "test-event-close",
		EventType: "test.event.close",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value"},
	}

	err = bus.Pub(ctx, event)
	assert.Error(t, err)

	// 验证关闭后无法订阅事件
	err = bus.Sub("test.event.afterclose", func(ctx context.Context, evt entity.Event) error {
		return nil
	})
	assert.Error(t, err)

	// 验证关闭后调用取消订阅不会出错
	err = bus.Unsub("test.event.close")
	assert.NoError(t, err)
}

// TestRabbitMQEventBus_PublishAfterClose 测试关闭后发布消息
func TestRabbitMQEventBus_PublishAfterClose(t *testing.T) {
	ctx := context.Background()
	container, config := setupRabbitMQContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewRabbitMQEventBus(config)
	assert.NoError(t, err)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)

	// 尝试发布事件
	event := &entity.GenericEvent{
		Id:        "test-event-after-close",
		EventType: "test.event",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value"},
	}

	err = bus.Pub(ctx, event)
	assert.Error(t, err)
}