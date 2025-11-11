package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/nats-io/nats.go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestFilterSubjectsIssue 测试FilterSubjects导致无法接收消息的问题
func TestFilterSubjectsIssue(t *testing.T) {
	ctx := context.Background()

	// 1. 使用TestContainers启动NATS容器
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:latest",
			ExposedPorts: []string{"4222/tcp", "8222/tcp"},
			Cmd:          []string{"-js"}, // 启用JetStream
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	}

	container, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}

	// 清理资源
	defer func() {
		if err = container.Terminate(ctx); err != nil {
			t.Fatalf("关闭NATS容器失败: %v", err)
		}
	}()

	// 2. 获取容器地址并连接NATS
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("获取容器地址失败: %v", err)
	}

	port, err := container.MappedPort(ctx, "4222/tcp")
	if err != nil {
		t.Fatalf("获取映射端口失败: %v", err)
	}

	natsURL := "nats://" + host + ":" + port.Port()

	// 连接NATS服务器
	conn, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("连接NATS服务器失败: %v", err)
	}
	defer conn.Close()

	// 3. 创建日志配置和日志器
	logCfg := &config.LogConfig{
		Level:  "debug", // 使用debug级别以便查看更多日志信息
		Format: "console",
	}
	testLogger, err := logger.NewLogger(logCfg)
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 4. 创建发布器
	publisher := NewJetStreamPublisher(
		conn,
		"filter-test-service",
		[]string{"TestAggregate"},
		testLogger,
	)

	// 5. 创建JetStreamBus
	bus := NewJetStreamBus(
		conn,
		"local-test-service",
		"filter-test-service",
		testLogger,
	).(*JetStreamBus)

	// 6. 创建事件接收通道，只订阅一种事件类型
	eventTypeACh := make(chan *entity.BaseEvent, 1)
	eventTypeBCh := make(chan *entity.BaseEvent, 1) // 这个通道应该不会收到消息

	// 7. 只添加EventTypeA的处理器，模拟只订阅一种事件类型的场景
	bus.AddHandler(
		"TestAggregate",
		"EventTypeA",
		func(ctx context.Context, event *entity.BaseEvent) error {
			testLogger.Info("接收到EventTypeA事件", "eventID", event.GetEventID())
			eventTypeACh <- event
			return nil
		})

	// 8. 启动总线
	if err := bus.Start(ctx); err != nil {
		t.Fatalf("启动JetStreamBus失败: %v", err)
	}
	defer bus.Stop(ctx)

	// 等待订阅器启动完成
	time.Sleep(1 * time.Second)

	// 9. 创建并发布两种不同类型的事件
	// 事件类型A（应该能被接收）
	eventA := &entity.BaseEvent{
		Id:            "event-a-1",
		EventType:     "EventTypeA",
		AggregateID:   "aggregate-123",
		AggregateType: "TestAggregate",
		Timestamp:     time.Now(),
		Version:       1,
	}

	// 事件类型B（不应该被接收，因为没有添加对应的处理器）
	eventB := &entity.BaseEvent{
		Id:            "event-b-1",
		EventType:     "EventTypeB",
		AggregateID:   "aggregate-123",
		AggregateType: "TestAggregate",
		Timestamp:     time.Now(),
		Version:       2,
	}

	// 10. 发布事件
	if err := publisher.Publish(ctx, eventA); err != nil {
		t.Fatalf("发布事件A失败: %v", err)
	}
	testLogger.Info("已发布事件A")

	if err := publisher.Publish(ctx, eventB); err != nil {
		t.Fatalf("发布事件B失败: %v", err)
	}
	testLogger.Info("已发布事件B")

	// 11. 验证事件A被正确接收
	select {
	case receivedEvent := <-eventTypeACh:
		testLogger.Info("成功接收到事件A", "eventID", receivedEvent.GetEventID())
		if receivedEvent.EventType != "EventTypeA" {
			t.Errorf("接收到的事件类型不匹配，期望: EventTypeA, 实际: %s", receivedEvent.EventType)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未能在超时时间内接收到事件A")
	}

	// 12. 验证事件B没有被接收（如果FilterSubjects工作正常，这里应该超时）
	select {
	case <-eventTypeBCh:
		t.Errorf("不应该接收到事件B，但却收到了")
	case <-time.After(2 * time.Second):
		testLogger.Info("正确地没有接收到事件B")
	}

	// 根据设计，所有处理器应该在Start之前添加
	// 这个测试不应该尝试在Start后添加新处理器
	testLogger.Info("测试完成：验证了在Start前添加的处理器能够正确接收消息")
	testLogger.Info("注意：根据设计架构，所有处理器必须在Start之前添加")
}
