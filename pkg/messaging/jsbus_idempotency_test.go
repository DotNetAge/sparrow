package messaging

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// TestJetStreamBusDeduplicatesDuplicateEvents 验证订阅端幂等（P0-3）
// RED→GREEN：同一事件 ID 重复投递时，handler 只应执行一次
func TestJetStreamBusDeduplicatesDuplicateEvents(t *testing.T) {
	conn, cleanup := startNatsContainer(t)
	defer cleanup()

	l := testNatsLogger(t)
	ctx := context.Background()

	// 创建发布器，确保流存在
	publisher := NewJetStreamPublisher(
		conn,
		"idempotency-service",
		[]string{"MockOrder"},
		l,
	)

	// 创建总线
	bus := NewJetStreamBus(
		conn,
		"idempotency-consumer",
		"idempotency-service",
		l,
	).(*JetStreamBus)

	// handler 调用计数
	var handlerCalls atomic.Int32
	bus.AddHandler("MockOrder", "MockOrderCreatedEvent", func(ctx context.Context, event *entity.BaseEvent) error {
		handlerCalls.Add(1)
		return nil
	})

	if err := bus.Start(ctx); err != nil {
		t.Fatalf("启动总线失败: %v", err)
	}
	defer bus.Stop(context.Background())

	// 等待订阅器就绪
	time.Sleep(1 * time.Second)

	// 构造同一事件（相同事件 ID），重复投递两次
	evt := &MockOrderCreatedEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "dup-evt-1",
			EventType:     "MockOrderCreatedEvent",
			AggregateID:   "order-dedup",
			AggregateType: "MockOrder",
			Timestamp:     time.Now(),
			Version:       1,
		},
		OrderNo: "ORD-DEDUP",
	}

	for i := 0; i < 2; i++ {
		if err := publisher.Publish(ctx, evt); err != nil {
			t.Fatalf("发布事件失败: %v", err)
		}
	}

	// 等待处理完成
	time.Sleep(2 * time.Second)

	if got := handlerCalls.Load(); got != 1 {
		t.Errorf("订阅端幂等失败: 同一事件投递 2 次，handler 期望执行 1 次，实际执行 %d 次", got)
	}
}
