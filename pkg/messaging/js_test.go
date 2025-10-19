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

type MockOrder struct {
	entity.BaseAggregateRoot           // 嵌入基础聚合根，继承通用字段和方法
	OrderNo                  string    `json:"order_no"`         // OrderNo 订单编号
	CustomerName             string    `json:"customer_name"`    // CustomerName 客户名称
	Receiver                 string    `json:"receiver"`         // Receiver 收货人姓名
	DeliveryAddress          string    `json:"delivery_address"` // DeliveryAddress 收货地址
	OrderedAt                time.Time `json:"ordered_at"`       // OrderedAt 订单日期
	TotalAmount              float64   `json:"total_amount"`     // TotalAmount 订单总金额
	Remarks                  string    `json:"remarks"`          // Remarks 备注
	State                    int       `json:"state"`
}

type MockOrderCreatedEvent struct {
	entity.BaseEvent
	OrderNo         string    `json:"order_no"`         // OrderNo 订单编号
	CustomerName    string    `json:"customer_name"`    // CustomerName 客户名称
	Receiver        string    `json:"receiver"`         // Receiver 收货人
	DeliveryAddress string    `json:"delivery_address"` // DeliveryAddress 收货地址
	OrderedAt       time.Time `json:"ordered_at"`       // OrderedAt 订单日期
}

type MockPlaceOrderEvent struct {
	entity.BaseEvent
	State       int     `json:"state"`
	Remarks     string  `json:"remarks"`      // Remarks 备注
	TotalAmount float64 `json:"total_amount"` // TotalAmount 订单总金额
}

func (a *MockOrder) GetAggregateType() string { return "MockOrder" }

// GetAggregateID 返回聚合ID（与GetID相同）
func (a *MockOrder) GetAggregateID() string { return a.Id }

func TestJetStreamPubSub(t *testing.T) {
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
		if err := container.Terminate(ctx); err != nil {
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

	// 3. 创建模拟事件
	orderCreatedEvent := &MockOrderCreatedEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "event-id-1",
			EventType:     "MockOrderCreatedEvent",
			AggregateID:   "67890",
			AggregateType: "MockOrder",
			Timestamp:     time.Now(),
			Version:       1,
		},
		OrderNo:         "ORD-2023-001",
		CustomerName:    "张三",
		Receiver:        "李四",
		DeliveryAddress: "北京市海淀区",
		OrderedAt:       time.Now(),
	}

	placeOrderEvent := &MockPlaceOrderEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "event-id-2",
			EventType:     "MockPlaceOrderEvent",
			AggregateID:   "67890",
			AggregateType: "MockOrder",
			Timestamp:     time.Now(),
			Version:       2,
		},
		State:       1,
		Remarks:     "加急订单",
		TotalAmount: 199.99,
	}

	// 4. 创建发布器
	// 创建日志配置和日志器
	logCfg := &config.LogConfig{
		Level:  "info",
		Format: "console",
	}
	testLogger, err := logger.NewLogger(logCfg)
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}
	publisher := NewJetStreamPublisher(
		conn,
		"order-service",
		"MockOrder",
		testLogger,
	)

	// 5. 创建订阅器和处理器
	orderCreatedCh := make(chan *MockOrderCreatedEvent, 1)
	orderCreatedSubscriber := NewJetStreamSubscriber(
		conn,
		"order-service",
		"MockOrder",
		testLogger,
		func(ctx context.Context, event *MockOrderCreatedEvent) error {
			orderCreatedCh <- event
			return nil
		},
	)

	placeOrderCh := make(chan *MockPlaceOrderEvent, 1)
	placeOrderSubscriber := NewJetStreamSubscriber(
		conn,
		"order-service",
		"MockOrder",

		testLogger,
		func(ctx context.Context, event *MockPlaceOrderEvent) error {
			placeOrderCh <- event
			return nil
		},
	)

	// 6. 启动订阅器
	if err := orderCreatedSubscriber.Start(ctx); err != nil {
		t.Fatalf("启动订单创建事件订阅器失败: %v", err)
	}
	defer orderCreatedSubscriber.Stop(ctx)

	if err := placeOrderSubscriber.Start(ctx); err != nil {
		t.Fatalf("启动下单事件订阅器失败: %v", err)
	}
	defer placeOrderSubscriber.Stop(ctx)

	// 等待订阅器启动完成
	time.Sleep(1 * time.Second)

	// 7. 发布事件
	if err := publisher.Publish(ctx, orderCreatedEvent); err != nil {
		t.Fatalf("发布订单创建事件失败: %v", err)
	}

	if err := publisher.Publish(ctx, placeOrderEvent); err != nil {
		t.Fatalf("发布下单事件失败: %v", err)
	}

	// 8. 等待接收事件（设置超时）
	select {
	case receivedEvent := <-orderCreatedCh:
		if receivedEvent.OrderNo != orderCreatedEvent.OrderNo {
			t.Errorf("接收到的订单号不匹配，期望: %s, 实际: %s",
				orderCreatedEvent.OrderNo, receivedEvent.OrderNo)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未能在超时时间内接收到订单创建事件")
	}

	select {
	case receivedEvent := <-placeOrderCh:
		if receivedEvent.TotalAmount != placeOrderEvent.TotalAmount {
			t.Errorf("接收到的订单金额不匹配，期望: %f, 实际: %f",
				placeOrderEvent.TotalAmount, receivedEvent.TotalAmount)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未能在超时时间内接收到下单事件")
	}
}
