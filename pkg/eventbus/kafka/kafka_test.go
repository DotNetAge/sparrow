package kafka

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/eventbus"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupKafkaContainer 设置Kafka测试容器
func setupKafkaContainer(t *testing.T) (testcontainers.Container, *config.KafkaConfig) {
	t.Helper()

	ctx := context.Background()

	// 使用bitnami/kafka镜像，确保配置完整且易于测试
	kafkaReq := testcontainers.ContainerRequest{
		Image:        "bitnami/kafka:3.4",
		ExposedPorts: []string{"9092/tcp"},
		Env: map[string]string{
			// 完整的Kraft模式配置
			"ALLOW_PLAINTEXT_LISTENER":           "yes",
			"KAFKA_ENABLE_KRAFT":                 "yes",
			"KAFKA_KRAFT_CLUSTER_ID":             "MkU3OEVBNTcwNTJENDM2Qk",
			"KAFKA_BROKER_ID":                    "1",
			"KAFKA_CFG_PROCESS_ROLES":            "broker,controller",
			"KAFKA_CFG_NODE_ID":                  "1",
			// 添加控制器仲裁投票者配置
			"KAFKA_CFG_CONTROLLER_QUORUM_VOTERS": "1@localhost:9093",
			// 添加必要的安全协议映射
			"KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP": "PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT",
			"KAFKA_CFG_LISTENERS":                "PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093",
			"KAFKA_CFG_CONTROLLER_LISTENER_NAMES": "CONTROLLER",
			"KAFKA_CFG_ADVERTISED_LISTENERS":     "PLAINTEXT://localhost:9092",
			"KAFKA_AUTO_CREATE_TOPICS_ENABLE":    "true",
		},
		WaitingFor: wait.ForLog("Kafka Server started"),
	}

	kafkaContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: kafkaReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("启动Kafka容器失败: %v", err)
	}

	// 确保测试结束时停止容器
	t.Cleanup(func() {
		if err := kafkaContainer.Terminate(ctx); err != nil {
			t.Logf("停止Kafka容器失败: %v", err)
		}
	})

	// 获取容器的主机和映射端口
	mappedPort, err := kafkaContainer.MappedPort(ctx, "9092/tcp")
	if err != nil {
		t.Fatalf("获取映射端口失败: %v", err)
	}

	// 获取容器的IP地址
	containerIP, err := kafkaContainer.Host(ctx)
	if err != nil {
		t.Fatalf("获取容器IP地址失败: %v", err)
	}

	// 构建正确的Kafka连接地址
	kafkaAddress := fmt.Sprintf("%s:%s", containerIP, mappedPort.Port())
	t.Logf("Kafka连接地址: %s", kafkaAddress)

	// 使用容器的实际地址配置Kafka连接
	kafkaConfig := &config.KafkaConfig{
		Brokers: []string{kafkaAddress},
		GroupID: "test-group",
		Topic:   "test_events",
	}

	// 记录连接配置
	t.Logf("Kafka配置: %+v", kafkaConfig)

	// 等待Kafka完全就绪
	t.Logf("等待Kafka就绪... 使用地址: %s", kafkaAddress)
	time.Sleep(10 * time.Second)

	return kafkaContainer, kafkaConfig
}

// createTestTopic 创建测试主题
func createTestTopic(t *testing.T, cfg *config.KafkaConfig) {
	t.Helper()

	t.Logf("创建测试主题，使用brokers: %v", cfg.Brokers)
	// 配置Kafka admin client
	adminConfig := sarama.NewConfig()
	adminConfig.Version = sarama.V3_4_0_0 // 使用与容器版本匹配的Kafka版本

	// 创建admin client - 使用配置中的动态映射地址和端口
	admin, err := sarama.NewClusterAdmin(cfg.Brokers, adminConfig)
	if err != nil {
		t.Logf("创建Admin Client失败: %v", err)
		return
	}
	defer admin.Close()

	// 创建主题
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     3,
		ReplicationFactor: 1,
	}

	err = admin.CreateTopic(cfg.Topic, topicDetail, false)
	if err != nil {
		t.Logf("创建主题失败: %v", err)
	}
}

// TestKafkaEventBus_Create 测试创建Kafka事件总线
func TestKafkaEventBus_Create(t *testing.T) {
	ctx := context.Background()
	container, config := setupKafkaContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewKafkaEventBus(config)
	assert.NoError(t, err)
	assert.NotNil(t, bus)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)
}

// TestKafkaEventBus_PubSub 测试发布和订阅功能
func TestKafkaEventBus_PubSub(t *testing.T) {
	ctx := context.Background()
	container, config := setupKafkaContainer(t)
	defer container.Terminate(ctx)

	// 记录配置的brokers地址
	t.Logf("测试中使用的brokers地址: %v", config.Brokers)

	// 创建事件总线
	bus, err := NewKafkaEventBus(config)
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

	// 等待订阅完成
	time.Sleep(2 * time.Second)

	// 发布事件
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待事件接收
	select {
	case <-eventReceived:
		// 成功接收到事件
	case <-time.After(10 * time.Second):
		t.Fatal("超时: 未接收到事件")
	}
}

// TestKafkaEventBus_MultipleSubscribers 测试多个订阅者
func TestKafkaEventBus_MultipleSubscribers(t *testing.T) {
	ctx := context.Background()
	container, config := setupKafkaContainer(t)
	defer container.Terminate(ctx)

	// 创建两个事件总线实例（模拟两个不同的服务）
	bus1, err := NewKafkaEventBus(config)
	assert.NoError(t, err)
	defer bus1.Close()

	bus2, err := NewKafkaEventBus(config)
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

	// 等待订阅完成
	time.Sleep(2 * time.Second)

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
	case <-time.After(10 * time.Second):
		t.Fatal("超时: 未接收到所有事件")
	}
}

// TestKafkaEventBus_Unsub 测试取消订阅
func TestKafkaEventBus_Unsub(t *testing.T) {
	ctx := context.Background()
	container, config := setupKafkaContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewKafkaEventBus(config)
	assert.NoError(t, err)
	defer bus.Close()

	// 准备事件数据
	const eventType = "test.event.unsub"
	event := eventbus.Event{
		Id:        "test-event-3",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value3"},
	}

	// 计数器
	var counter int32 = 0

	// 订阅事件
	err = bus.Sub(eventType, func(ctx context.Context, evt eventbus.Event) error {
		counter++
		return nil
	})
	assert.NoError(t, err)

	// 等待订阅完成
	time.Sleep(2 * time.Second)

	// 发布第一个事件
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待事件处理
	time.Sleep(3 * time.Second)
	assert.Equal(t, int32(1), counter)

	// 取消订阅
	err = bus.Unsub(eventType)
	assert.NoError(t, err)

	// 修改事件ID（避免消息重复）
	event.Id = "test-event-4"

	// 等待取消订阅生效
	time.Sleep(2 * time.Second)

	// 发布第二个事件
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待一段时间，确认没有收到事件
	time.Sleep(3 * time.Second)
	assert.Equal(t, int32(1), counter) // 计数器不应增加
}

// TestKafkaEventBus_HandlerError 测试处理器错误
func TestKafkaEventBus_HandlerError(t *testing.T) {
	ctx := context.Background()
	container, config := setupKafkaContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewKafkaEventBus(config)
	assert.NoError(t, err)
	defer bus.Close()

	// 准备事件数据
	const eventType = "test.event.error"
	event := eventbus.Event{
		Id:        "test-event-5",
		EventType: eventType,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value5"},
	}

	// 错误处理器
	err = bus.Sub(eventType, func(ctx context.Context, evt eventbus.Event) error {
		return fmt.Errorf("故意的错误")
	})
	assert.NoError(t, err)

	// 等待订阅完成
	time.Sleep(2 * time.Second)

	// 发布事件（即使处理器返回错误，Pub也应该成功）
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待一段时间，确保处理器执行
	time.Sleep(3 * time.Second)
}

// TestKafkaEventBus_Close 测试关闭事件总线
func TestKafkaEventBus_Close(t *testing.T) {
	ctx := context.Background()
	container, config := setupKafkaContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewKafkaEventBus(config)
	assert.NoError(t, err)

	// 订阅一个事件类型
	err = bus.Sub("test.event.close", func(ctx context.Context, evt eventbus.Event) error {
		return nil
	})
	assert.NoError(t, err)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)

	// 验证关闭后无法发布事件
	event := eventbus.Event{
		Id:        "test-event-close",
		EventType: "test.event.close",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value"},
	}

	err = bus.Pub(ctx, event)
	assert.Error(t, err)

	// 验证关闭后无法订阅事件
	err = bus.Sub("test.event.afterclose", func(ctx context.Context, evt eventbus.Event) error {
		return nil
	})
	assert.Error(t, err)

	// 验证关闭后调用取消订阅不会出错
	err = bus.Unsub("test.event.close")
	assert.NoError(t, err)
}

// TestKafkaEventBus_PublishAfterClose 测试关闭后发布消息
func TestKafkaEventBus_PublishAfterClose(t *testing.T) {
	ctx := context.Background()
	container, config := setupKafkaContainer(t)
	defer container.Terminate(ctx)

	// 创建事件总线
	bus, err := NewKafkaEventBus(config)
	assert.NoError(t, err)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)

	// 尝试发布事件
	event := eventbus.Event{
		Id:        "test-event-after-close",
		EventType: "test.event",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"key": "value"},
	}

	err = bus.Pub(ctx, event)
	assert.Error(t, err)
}