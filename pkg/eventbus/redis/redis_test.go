package redis

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupRedisContainer 设置Redis测试容器
func setupRedisContainer(t *testing.T) (testcontainers.Container, *config.RedisConfig) {
	t.Helper()

	// 创建Redis容器
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "redis:6",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}

	// 获取容器IP和端口
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	// 配置Redis连接信息
	redisConfig := &config.RedisConfig{
		Host:     host,
		Port:     port.Int(),
		Password: "",
		DB:       0,
	}

	return container, redisConfig
}

// TestNewRedisEventBus 测试创建Redis事件总线实例
func TestNewRedisEventBus(t *testing.T) {
	// 启动Redis容器
	container, redisConfig := setupRedisContainer(t)
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("Failed to terminate container: %v", err)
		}
	}()

	// 创建事件总线
	bus, err := NewRedisEventBus(redisConfig)
	assert.NoError(t, err, "创建Redis事件总线应该成功")
	assert.NotNil(t, bus, "事件总线实例不应该为空")

	// 关闭事件总线
	assert.NoError(t, bus.Close(), "关闭事件总线应该成功")
}

// TestRedisEventBus_PubSub 测试发布和订阅功能
func TestRedisEventBus_PubSub(t *testing.T) {
	// 启动Redis容器
	container, redisConfig := setupRedisContainer(t)
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("Failed to terminate container: %v", err)
		}
	}()

	// 创建事件总线
	bus, err := NewRedisEventBus(redisConfig)
	assert.NoError(t, err, "创建Redis事件总线应该成功")
	defer bus.Close()

	// 测试事件数据
	testEvent := eventbus.Event{
		Id:        "test-id-1",
		EventType: "testevent",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value"},
	}

	// 等待组，用于同步测试
	var wg sync.WaitGroup
	handlerCalled := false

	// 订阅事件
	wg.Add(1)
	err = bus.Sub(testEvent.EventType, func(ctx context.Context, event eventbus.Event) error {
		defer wg.Done()
		handlerCalled = true

		// 验证接收到的事件数据
		assert.Equal(t, testEvent.Id, event.Id, "事件ID应该匹配")
		assert.Equal(t, testEvent.EventType, event.EventType, "事件类型应该匹配")

		return nil
	})
	assert.NoError(t, err, "订阅事件应该成功")

	// 发布事件
	err = bus.Pub(context.Background(), testEvent)
	assert.NoError(t, err, "发布事件应该成功")

	// 等待订阅者处理完成
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
		t.Fatal("处理事件超时")
	}

	// 验证处理器被调用
	assert.True(t, handlerCalled, "事件处理器应该被调用")
}

// TestRedisEventBus_MultipleSubscribers 测试多个订阅者
func TestRedisEventBus_MultipleSubscribers(t *testing.T) {
	// 启动Redis容器
	container, redisConfig := setupRedisContainer(t)
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("Failed to terminate container: %v", err)
		}
	}()

	// 创建多个事件总线实例（模拟多个订阅者）
	bus1, err := NewRedisEventBus(redisConfig)
	assert.NoError(t, err, "创建事件总线1应该成功")
	defer bus1.Close()

	bus2, err := NewRedisEventBus(redisConfig)
	assert.NoError(t, err, "创建事件总线2应该成功")
	defer bus2.Close()

	busPub, err := NewRedisEventBus(redisConfig)
	assert.NoError(t, err, "创建发布者事件总线应该成功")
	defer busPub.Close()

	// 测试事件数据
	testEvent := eventbus.Event{
		Id:        "test-id-multi",
		EventType: "testmultipleevent",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"multi": "value"},
	}

	// 等待组，用于同步测试
	var wg sync.WaitGroup
	subscriber1Called := false
	subscriber2Called := false

	// 订阅者1
	wg.Add(1)
	err = bus1.Sub(testEvent.EventType, func(ctx context.Context, event eventbus.Event) error {
		defer wg.Done()
		subscriber1Called = true
		return nil
	})
	assert.NoError(t, err, "订阅者1订阅应该成功")

	// 订阅者2
	wg.Add(1)
	err = bus2.Sub(testEvent.EventType, func(ctx context.Context, event eventbus.Event) error {
		defer wg.Done()
		subscriber2Called = true
		return nil
	})
	assert.NoError(t, err, "订阅者2订阅应该成功")

	// 发布事件
	err = busPub.Pub(context.Background(), testEvent)
	assert.NoError(t, err, "发布事件应该成功")

	// 等待所有订阅者处理完成
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
		t.Fatal("处理事件超时")
	}

	// 验证两个订阅者都收到了事件
	assert.True(t, subscriber1Called, "订阅者1应该收到事件")
	assert.True(t, subscriber2Called, "订阅者2应该收到事件")
}

// TestRedisEventBus_Unsub 测试取消订阅功能
func TestRedisEventBus_Unsub(t *testing.T) {
	// 启动Redis容器
	container, redisConfig := setupRedisContainer(t)
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("Failed to terminate container: %v", err)
		}
	}()

	// 创建事件总线
	bus, err := NewRedisEventBus(redisConfig)
	assert.NoError(t, err, "创建Redis事件总线应该成功")
	defer bus.Close()

	// 测试事件类型
	eventType := "testunsubevent"

	// 订阅事件
	handlerCalled := false
	err = bus.Sub(eventType, func(ctx context.Context, event eventbus.Event) error {
		handlerCalled = true
		return nil
	})
	assert.NoError(t, err, "订阅事件应该成功")

	// 取消订阅
	err = bus.Unsub(eventType)
	assert.NoError(t, err, "取消订阅应该成功")

	// 发布事件
	testEvent := eventbus.Event{
		Id:        "test-id-unsub",
		EventType: eventType,
		Timestamp: time.Now(),
	}
	err = bus.Pub(context.Background(), testEvent)
	assert.NoError(t, err, "发布事件应该成功")

	// 等待一段时间，确保事件不会被处理
	time.Sleep(1 * time.Second)

	// 验证处理器没有被调用
	assert.False(t, handlerCalled, "取消订阅后，处理器不应该被调用")
}

// TestRedisEventBus_HandlerError 测试处理器错误
func TestRedisEventBus_HandlerError(t *testing.T) {
	// 启动Redis容器
	container, redisConfig := setupRedisContainer(t)
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("Failed to terminate container: %v", err)
		}
	}()

	// 创建事件总线
	bus, err := NewRedisEventBus(redisConfig)
	assert.NoError(t, err, "创建Redis事件总线应该成功")
	defer bus.Close()

	// 订阅事件，处理器返回错误
	handlerError := fmt.Errorf("handler error")
	err = bus.Sub("testerror", func(ctx context.Context, event eventbus.Event) error {
		return handlerError
	})
	assert.NoError(t, err, "订阅事件应该成功")

	// 发布事件
	testEvent := eventbus.Event{
		Id:        "test-id-error",
		EventType: "testerror",
		Timestamp: time.Now(),
	}
	err = bus.Pub(context.Background(), testEvent)
	assert.NoError(t, err, "发布事件应该成功")

	// 等待一段时间，让处理器有机会执行
	time.Sleep(1 * time.Second)

	// 此时错误应该已经在日志中记录，但不应影响事件总线的运行
	// 我们验证可以继续发布和接收其他事件
	successHandlerCalled := false
	err = bus.Sub("testsuccess", func(ctx context.Context, event eventbus.Event) error {
		successHandlerCalled = true
		return nil
	})
	assert.NoError(t, err, "订阅成功事件应该成功")

	// 发布成功事件
	successEvent := eventbus.Event{
		Id:        "test-id-success",
		EventType: "testsuccess",
		Timestamp: time.Now(),
	}
	err = bus.Pub(context.Background(), successEvent)
	assert.NoError(t, err, "发布成功事件应该成功")

	// 等待处理器执行
	time.Sleep(1 * time.Second)

	// 验证成功处理器被调用
	assert.True(t, successHandlerCalled, "成功处理器应该被调用")
}

// TestRedisEventBus_InvalidEvent 测试无效的事件数据
func TestRedisEventBus_InvalidEvent(t *testing.T) {
	// 启动Redis容器
	container, redisConfig := setupRedisContainer(t)
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("Failed to terminate container: %v", err)
		}
	}()

	// 创建事件总线
	bus, err := NewRedisEventBus(redisConfig)
	assert.NoError(t, err, "创建Redis事件总线应该成功")
	defer bus.Close()

	// 测试无效的JSON数据
	invalidData := []byte("{invalid json}")

	// 直接通过Redis客户端发布无效数据
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port),
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})
	defer client.Close()

	err = client.Publish(ctx, "testinvalid", invalidData).Err()
	assert.NoError(t, err, "通过Redis客户端发布无效数据应该成功")

	// 订阅事件
	handlerCalled := false
	err = bus.Sub("testinvalid", func(ctx context.Context, event eventbus.Event) error {
		handlerCalled = true
		return nil
	})
	assert.NoError(t, err, "订阅事件应该成功")

	// 等待一段时间，让处理器有机会执行
	time.Sleep(1 * time.Second)

	// 验证处理器没有被调用，因为数据格式无效
	assert.False(t, handlerCalled, "无效数据不应该触发处理器调用")
}
