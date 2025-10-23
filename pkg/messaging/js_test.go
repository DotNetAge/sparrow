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

func (e *MockOrderCreatedEvent) GetAggregateID() string   { return e.AggregateID }
func (e *MockOrderCreatedEvent) GetAggregateType() string { return e.AggregateType }
func (e *MockOrderCreatedEvent) GetEventType() string     { return e.EventType }
func (e *MockOrderCreatedEvent) GetID() string            { return e.Id }
func (e *MockOrderCreatedEvent) GetTimestamp() time.Time  { return e.Timestamp }
func (e *MockOrderCreatedEvent) GetVersion() int          { return e.Version }

type MockPlaceOrderEvent struct {
	entity.BaseEvent
	State       int     `json:"state"`
	Remarks     string  `json:"remarks"`      // Remarks 备注
	TotalAmount float64 `json:"total_amount"` // TotalAmount 订单总金额
}

func (e *MockPlaceOrderEvent) GetAggregateID() string   { return e.AggregateID }
func (e *MockPlaceOrderEvent) GetAggregateType() string { return e.AggregateType }
func (e *MockPlaceOrderEvent) GetEventType() string     { return e.EventType }
func (e *MockPlaceOrderEvent) GetID() string            { return e.Id }
func (e *MockPlaceOrderEvent) GetTimestamp() time.Time  { return e.Timestamp }
func (e *MockPlaceOrderEvent) GetVersion() int          { return e.Version }

type MockPaymentCreatedEvent struct {
	entity.BaseEvent
	PaymentNo     string    `json:"payment_no"`
	OrderID       string    `json:"order_id"`
	Amount        float64   `json:"amount"`
	PaymentMethod string    `json:"payment_method"`
	PaidAt        time.Time `json:"paid_at"`
}

func (e *MockPaymentCreatedEvent) GetAggregateID() string   { return e.AggregateID }
func (e *MockPaymentCreatedEvent) GetAggregateType() string { return e.AggregateType }
func (e *MockPaymentCreatedEvent) GetEventType() string     { return e.EventType }
func (e *MockPaymentCreatedEvent) GetID() string            { return e.Id }
func (e *MockPaymentCreatedEvent) GetTimestamp() time.Time  { return e.Timestamp }
func (e *MockPaymentCreatedEvent) GetVersion() int          { return e.Version }

type MockPaymentFailedEvent struct {
	entity.BaseEvent
	PaymentNo string  `json:"payment_no"`
	OrderID   string  `json:"order_id"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason"`
}

func (e *MockPaymentFailedEvent) GetAggregateID() string   { return e.AggregateID }
func (e *MockPaymentFailedEvent) GetAggregateType() string { return e.AggregateType }
func (e *MockPaymentFailedEvent) GetEventType() string     { return e.EventType }
func (e *MockPaymentFailedEvent) GetID() string            { return e.Id }
func (e *MockPaymentFailedEvent) GetTimestamp() time.Time  { return e.Timestamp }
func (e *MockPaymentFailedEvent) GetVersion() int          { return e.Version }

func (a *MockOrder) GetAggregateType() string { return "MockOrder" }

// GetAggregateID 返回聚合ID（与GetID相同）
func (a *MockOrder) GetAggregateID() string { return a.Id }

func TestJetStreamBusMultiAggregate(t *testing.T) {
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
		Level:  "info",
		Format: "console",
	}
	testLogger, err := logger.NewLogger(logCfg)
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 4. 先创建发布器，确保流存在（Publisher会自动创建流）
	publisher := NewJetStreamPublisher(
		conn,
		"multi-aggregate-service",
		[]string{"MockOrder", "MockPayment"},
		testLogger,
	)

	// 5. 创建JetStreamBus
	bus := NewJetStreamBus(
		conn,
		"multi-aggregate-service",
		testLogger,
	).(*JetStreamBus)

	// 初始化handlers映射
	// bus.handlers = make(map[string]DomainEventHandler[*entity.BaseEvent])

	// 6. 创建事件接收通道
	orderCreatedCh := make(chan *entity.BaseEvent, 1)
	orderUpdatedCh := make(chan *entity.BaseEvent, 1)
	paymentCreatedCh := make(chan *entity.BaseEvent, 1)
	paymentFailedCh := make(chan *entity.BaseEvent, 1)

	// 7. 添加事件处理器
	bus.AddEventHandler("MockOrder", "MockOrderCreatedEvent", func(ctx context.Context, event *entity.BaseEvent) error {
		orderCreatedCh <- event
		return nil
	})

	bus.AddEventHandler("MockOrder", "MockPlaceOrderEvent", func(ctx context.Context, event *entity.BaseEvent) error {
		orderUpdatedCh <- event
		return nil
	})

	bus.AddEventHandler("MockPayment", "MockPaymentCreatedEvent", func(ctx context.Context, event *entity.BaseEvent) error {
		paymentCreatedCh <- event
		return nil
	})

	bus.AddEventHandler("MockPayment", "MockPaymentFailedEvent", func(ctx context.Context, event *entity.BaseEvent) error {
		paymentFailedCh <- event
		return nil
	})

	// 8. 启动总线
	if err := bus.Start(ctx); err != nil {
		t.Fatalf("启动JetStreamBus失败: %v", err)
	}
	defer bus.Stop(ctx)

	// 9. 创建并发布多种聚合类型的事件
	// 订单创建事件
	orderCreatedEvent := &MockOrderCreatedEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "order-event-1",
			EventType:     "MockOrderCreatedEvent",
			AggregateID:   "order-123",
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

	// 订单更新事件
	orderUpdatedEvent := &MockPlaceOrderEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "order-event-2",
			EventType:     "MockPlaceOrderEvent",
			AggregateID:   "order-123",
			AggregateType: "MockOrder",
			Timestamp:     time.Now(),
			Version:       2,
		},
		State:       1,
		Remarks:     "加急订单",
		TotalAmount: 199.99,
	}

	// 支付创建事件
	paymentCreatedEvent := &MockPaymentCreatedEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "payment-event-1",
			EventType:     "MockPaymentCreatedEvent",
			AggregateID:   "payment-456",
			AggregateType: "MockPayment",
			Timestamp:     time.Now(),
			Version:       1,
		},
		PaymentNo:     "PAY-2023-001",
		OrderID:       "order-123",
		Amount:        199.99,
		PaymentMethod: "支付宝",
		PaidAt:        time.Now(),
	}

	// 支付失败事件
	paymentFailedEvent := &MockPaymentFailedEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "payment-event-2",
			EventType:     "MockPaymentFailedEvent",
			AggregateID:   "payment-456",
			AggregateType: "MockPayment",
			Timestamp:     time.Now(),
			Version:       2,
		},
		PaymentNo: "PAY-2023-001",
		OrderID:   "order-123",
		Amount:    199.99,
		Reason:    "余额不足",
	}

	// 等待订阅器启动完成
	time.Sleep(1 * time.Second)

	// 10. 使用现有的Publisher发布事件
	if err := publisher.Publish(ctx, orderCreatedEvent); err != nil {
		t.Fatalf("发布订单创建事件失败: %v", err)
	}
	if err := publisher.Publish(ctx, orderUpdatedEvent); err != nil {
		t.Fatalf("发布订单更新事件失败: %v", err)
	}
	if err := publisher.Publish(ctx, paymentCreatedEvent); err != nil {
		t.Fatalf("发布支付创建事件失败: %v", err)
	}
	if err := publisher.Publish(ctx, paymentFailedEvent); err != nil {
		t.Fatalf("发布支付失败事件失败: %v", err)
	}

	// 11. 验证所有事件都被正确处理
	// 验证订单创建事件
	select {
	case receivedEvent := <-orderCreatedCh:
		if receivedEvent.EventType != "MockOrderCreatedEvent" {
			t.Errorf("接收到的事件类型不匹配，期望: MockOrderCreatedEvent, 实际: %s", receivedEvent.EventType)
		}
		if receivedEvent.AggregateID != "order-123" {
			t.Errorf("接收到的聚合ID不匹配，期望: order-123, 实际: %s", receivedEvent.AggregateID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未能在超时时间内接收到订单创建事件")
	}

	// 验证订单更新事件
	select {
	case receivedEvent := <-orderUpdatedCh:
		if receivedEvent.EventType != "MockPlaceOrderEvent" {
			t.Errorf("接收到的事件类型不匹配，期望: MockPlaceOrderEvent, 实际: %s", receivedEvent.EventType)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未能在超时时间内接收到订单更新事件")
	}

	// 验证支付创建事件
	select {
	case receivedEvent := <-paymentCreatedCh:
		if receivedEvent.EventType != "MockPaymentCreatedEvent" {
			t.Errorf("接收到的事件类型不匹配，期望: MockPaymentCreatedEvent, 实际: %s", receivedEvent.EventType)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未能在超时时间内接收到支付创建事件")
	}

	// 验证支付失败事件
	select {
	case receivedEvent := <-paymentFailedCh:
		if receivedEvent.EventType != "MockPaymentFailedEvent" {
			t.Errorf("接收到的事件类型不匹配，期望: MockPaymentFailedEvent, 实际: %s", receivedEvent.EventType)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未能在超时时间内接收到支付失败事件")
	}

	// 12. 测试优雅关闭
	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := bus.Stop(stopCtx); err != nil {
		t.Fatalf("停止JetStreamBus失败: %v", err)
	}
}

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
		[]string{"MockOrder"},
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

	reader := NewJetStreamReader(
		conn,
		"order-service",
		"MockOrder",
		testLogger,
	)

	events, err := reader.GetEvents(ctx, "67890")
	if err != nil {
		t.Fatalf("读取事件失败: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("期望读取2个事件，实际读取: %d", len(events))
	}
}
