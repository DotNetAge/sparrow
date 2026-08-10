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

// startNatsContainer 启动带 JetStream 的 NATS 容器，返回连接与清理函数
func startNatsContainer(t *testing.T) (*nats.Conn, func()) {
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

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("获取容器地址失败: %v", err)
	}
	port, err := container.MappedPort(ctx, "4222/tcp")
	if err != nil {
		t.Fatalf("获取映射端口失败: %v", err)
	}

	conn, err := nats.Connect("nats://" + host + ":" + port.Port())
	if err != nil {
		t.Fatalf("连接NATS服务器失败: %v", err)
	}

	cleanup := func() {
		conn.Close()
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("关闭NATS容器失败: %v", err)
		}
	}
	return conn, cleanup
}

func testNatsLogger(t *testing.T) *logger.Logger {
	t.Helper()
	logCfg := &config.LogConfig{Level: "info", Format: "console"}
	l, err := logger.NewLogger(logCfg)
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}
	return l
}

// TestJetStreamReaderGetAggregateEvents 验证按聚合ID读取完整事件流（P0-2 重放接口）
// RED→GREEN：先断言 GetAggregateEvents 方法存在且返回全量、按版本升序
func TestJetStreamReaderGetAggregateEvents(t *testing.T) {
	conn, cleanup := startNatsContainer(t)
	defer cleanup()

	l := testNatsLogger(t)

	// 先创建发布器，确保流存在
	publisher := NewJetStreamPublisher(
		conn,
		"reader-aggregate-service",
		[]string{"MockOrder"},
		l,
	)

	// 读取器
	reader := NewJetStreamReader(
		conn,
		"reader-aggregate-service",
		"MockOrder",
		l,
	)

	// 发布同一聚合的 3 个版本事件（乱序创建，验证读取时按版本升序）
	events := []*MockOrderCreatedEvent{
		{
			BaseEvent: entity.BaseEvent{
				Id:            "agg-evt-1",
				EventType:     "MockOrderCreatedEvent",
				AggregateID:   "agg-1",
				AggregateType: "MockOrder",
				Timestamp:     time.Now(),
				Version:       1,
			},
			OrderNo: "ORD-001",
		},
		{
			BaseEvent: entity.BaseEvent{
				Id:            "agg-evt-3",
				EventType:     "MockOrderCreatedEvent",
				AggregateID:   "agg-1",
				AggregateType: "MockOrder",
				Timestamp:     time.Now(),
				Version:       3,
			},
			OrderNo: "ORD-003",
		},
		{
			BaseEvent: entity.BaseEvent{
				Id:            "agg-evt-2",
				EventType:     "MockOrderCreatedEvent",
				AggregateID:   "agg-1",
				AggregateType: "MockOrder",
				Timestamp:     time.Now(),
				Version:       2,
			},
			OrderNo: "ORD-002",
		},
	}

	for _, evt := range events {
		if err := publisher.Publish(context.Background(), evt); err != nil {
			t.Fatalf("发布事件失败: %v", err)
		}
	}

	// 等待事件写入完成
	time.Sleep(500 * time.Millisecond)

	// GetAggregateEvents：按聚合ID取完整事件流
	r, ok := reader.(*JetStreamReader)
	if !ok {
		t.Fatalf("NewJetStreamReader 未返回 *JetStreamReader，实际类型: %T", reader)
	}
	got, err := r.GetAggregateEvents(context.Background(), "agg-1")
	if err != nil {
		t.Fatalf("GetAggregateEvents 失败: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("GetAggregateEvents 期望返回 3 个事件，实际: %d", len(got))
	}

	// 按版本升序
	for i := 0; i < len(got); i++ {
		if wantVersion := i + 1; got[i].GetVersion() != wantVersion {
			t.Errorf("事件流版本乱序: 位置 %d 期望版本 %d，实际 %d", i, wantVersion, got[i].GetVersion())
		}
	}

	// 不存在的聚合返回空切片且不报错
	missing, err := r.GetAggregateEvents(context.Background(), "agg-404")
	if err != nil {
		t.Fatalf("GetAggregateEvents 对不存在聚合不应报错: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("GetAggregateEvents 对不存在聚合期望空切片，实际 %d 个事件", len(missing))
	}
}
