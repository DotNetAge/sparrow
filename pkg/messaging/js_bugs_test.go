package messaging

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startTestNATS 启动 TestContainers NATS 容器并返回连接
func startTestNATS(t *testing.T) *nats.Conn {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:latest",
			ExposedPorts: []string{"4222/tcp", "8222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	}

	container, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		t.Fatalf("启动NATS容器失败: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("获取容器地址失败: %v", err)
	}
	port, err := container.MappedPort(ctx, "4222/tcp")
	if err != nil {
		t.Fatalf("获取映射端口失败: %v", err)
	}

	natsURL := "nats://" + host + ":" + port.Port()

	// 容器监听就绪后 NATS 可能仍需短暂时间才可接受连接，重试连接
	var conn *nats.Conn
	for i := 0; i < 10; i++ {
		conn, err = nats.Connect(natsURL)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("连接NATS服务器失败: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return conn
}

// newTestLogger 创建测试用日志器
func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	logCfg := &config.LogConfig{
		Level:  "info",
		Format: "console",
	}
	l, err := logger.NewLogger(logCfg)
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}
	return l
}

// TestJetStreamPublisherPayloadIntegrity 验证发布端到读端的负载完整性：
// 1. 原始事件时间戳不被改写
// 2. 负载经 base64 编码还原后字段一致
func TestJetStreamPublisherPayloadIntegrity(t *testing.T) {
	conn := startTestNATS(t)
	log := newTestLogger(t)

	publisher := NewJetStreamPublisher(conn, "order-service", []string{"MockOrder"}, log)
	reader := NewJetStreamReader(conn, "order-service", "MockOrder", log)

	createdAt := time.Now()
	event := &MockOrderCreatedEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "event-id-payload-1",
			EventType:     "MockOrderCreatedEvent",
			AggregateID:   "agg-payload-1",
			AggregateType: "MockOrder",
			Timestamp:     createdAt,
			Version:       1,
		},
		OrderNo:      "ORD-2023-001",
		CustomerName: "张三",
	}

	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("发布事件失败: %v", err)
	}

	events, err := reader.GetEvents(context.Background(), "agg-payload-1")
	if err != nil {
		t.Fatalf("读取事件失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("期望 1 条事件，实际: %d", len(events))
	}

	// 读回的事件必须保留原始时间戳
	got := events[0]
	if !got.GetCreatedAt().Equal(event.Timestamp) {
		t.Errorf("事件时间戳被改写，期望: %v, 实际: %v", event.Timestamp, got.GetCreatedAt())
	}

	// 负载必须能还原出原始领域事件
	decoded, err := entity.UnmarshalEvent[*MockOrderCreatedEvent](got.(*entity.BaseEvent))
	if err != nil {
		t.Fatalf("解码事件负载失败: %v", err)
	}
	if decoded.OrderNo != "ORD-2023-001" || decoded.CustomerName != "张三" {
		t.Errorf("负载还原字段不匹配: %+v", decoded)
	}
	if decoded.AggregateID != "agg-payload-1" {
		t.Errorf("聚合ID还原不匹配: %s", decoded.AggregateID)
	}
}

// TestJetStreamReaderReadsAllEventsBeyondBatchSize 验证单个聚合事件数超过单批上限时不会截断
func TestJetStreamReaderReadsAllEventsBeyondBatchSize(t *testing.T) {
	conn := startTestNATS(t)
	log := newTestLogger(t)

	publisher := NewJetStreamPublisher(conn, "order-service", []string{"MockOrder"}, log)
	reader := NewJetStreamReader(conn, "order-service", "MockOrder", log)

	const total = 1200
	for i := 0; i < total; i++ {
		ev := &entity.BaseEvent{
			Id:            fmt.Sprintf("event-batch-%d", i),
			EventType:     "MockOrderCreatedEvent",
			AggregateID:   "agg-batch-1",
			AggregateType: "MockOrder",
			Timestamp:     time.Now(),
			Version:       i + 1,
		}
		if err := publisher.Publish(context.Background(), ev); err != nil {
			t.Fatalf("发布事件 %d 失败: %v", i, err)
		}
	}

	events, err := reader.GetEvents(context.Background(), "agg-batch-1")
	if err != nil {
		t.Fatalf("读取事件失败: %v", err)
	}
	if len(events) != total {
		t.Fatalf("期望读取 %d 条事件，实际: %d", total, len(events))
	}
}

// TestJetStreamBusHandlesMalformedSubject 验证订阅器收到主题段数不足的消息时不崩溃，
// 后续正常消息仍能被处理。
func TestJetStreamBusHandlesMalformedSubject(t *testing.T) {
	conn := startTestNATS(t)
	log := newTestLogger(t)

	// 创建发布器确保流存在
	publisher := NewJetStreamPublisher(conn, "order-service", []string{"MockOrder"}, log)

	bus := NewJetStreamBus(conn, "test-service", "order-service", log).(*JetStreamBus)

	received := make(chan *entity.BaseEvent, 1)

	// 先启动订阅器（此时未注册任何处理器），再注册处理器
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("启动订阅器失败: %v", err)
	}
	defer bus.Stop(context.Background())

	bus.AddHandler("MockOrder", "MockOrderCreatedEvent", func(ctx context.Context, event *entity.BaseEvent) error {
		received <- event
		return nil
	})

	// 等待订阅就绪
	time.Sleep(1 * time.Second)

	// 直接向流发布一条只有两段的畸形主题消息（绕过发布器）
	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatalf("创建JetStream客户端失败: %v", err)
	}
	malformed := []byte(`{"id":"bad","aggregate_id":"x","event_type":"Bad","aggregate_type":"MockOrder","version":1}`)
	if _, err := js.Publish(context.Background(), "MockOrder.bad", malformed); err != nil {
		t.Fatalf("发布畸形消息失败: %v", err)
	}

	// 发布一条未注册事件类型的消息（主题格式正确但无处理器）
	unknown := &entity.BaseEvent{
		Id:            "unknown-1",
		EventType:     "MockUnknownEvent",
		AggregateID:   "agg-unknown-1",
		AggregateType: "MockOrder",
		Timestamp:     time.Now(),
		Version:       1,
	}
	if err := publisher.Publish(context.Background(), unknown); err != nil {
		t.Fatalf("发布未知事件消息失败: %v", err)
	}

	// 发布一条正常消息，验证订阅器未因畸形/未知消息而崩溃或阻塞
	normal := &entity.BaseEvent{
		Id:            "normal-1",
		EventType:     "MockOrderCreatedEvent",
		AggregateID:   "agg-normal-1",
		AggregateType: "MockOrder",
		Timestamp:     time.Now(),
		Version:       1,
	}
	if err := publisher.Publish(context.Background(), normal); err != nil {
		t.Fatalf("发布正常消息失败: %v", err)
	}

	select {
	case ev := <-received:
		if ev.AggregateID != "agg-normal-1" {
			t.Errorf("收到意外事件: %+v", ev)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("未能在超时时间内收到正常事件，订阅器可能已崩溃或被阻塞")
	}
}
