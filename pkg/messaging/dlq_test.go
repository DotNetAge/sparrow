package messaging

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// TestDLQDeadLetterAfterMaxDeliver 验证死信队列（P2）：
// handler 连续失败达到 MaxDeliver（默认 3）后，消息转入死信队列并确认原消息，避免无限重试阻塞消费
func TestDLQDeadLetterAfterMaxDeliver(t *testing.T) {
	conn, cleanup := startNatsContainer(t)
	defer cleanup()

	l := testNatsLogger(t)
	ctx := context.Background()

	// 创建发布器，确保事件流存在
	publisher := NewJetStreamPublisher(
		conn,
		"dlq-test-service",
		[]string{"MockOrder"},
		l,
	)

	bus := NewJetStreamBus(
		conn,
		"dlq-test-consumer",
		"dlq-test-service",
		l,
	).(*JetStreamBus)

	// handler 持续故障
	bus.AddHandler("MockOrder", "MockOrderCreatedEvent", func(ctx context.Context, event *entity.BaseEvent) error {
		return errors.New("模拟持续故障")
	})

	if err := bus.Start(ctx); err != nil {
		t.Fatalf("启动总线失败: %v", err)
	}
	defer bus.Stop(context.Background())

	time.Sleep(1 * time.Second)

	evt := &MockOrderCreatedEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "dlq-evt-1",
			EventType:     "MockOrderCreatedEvent",
			AggregateID:   "order-dlq",
			AggregateType: "MockOrder",
			Timestamp:     time.Now(),
			Version:       1,
		},
		OrderNo: "ORD-DLQ",
	}
	if err := publisher.Publish(ctx, evt); err != nil {
		t.Fatalf("发布事件失败: %v", err)
	}

	// 轮询等待死信写入（3 次重试后进入死信）
	deadline := time.Now().Add(20 * time.Second)
	for {
		count, err := bus.DLQCount(ctx)
		if err == nil && count >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("等待死信写入超时")
		}
		time.Sleep(500 * time.Millisecond)
	}

	count, err := bus.DLQCount(ctx)
	if err != nil {
		t.Fatalf("查询死信数量失败: %v", err)
	}
	if count != 1 {
		t.Errorf("死信数量期望 1，实际 %d", count)
	}
}

// TestDLQReplayRepublishes 验证死信重放：
// 死信消息按原主题重新发布回事件流，可被正常 handler 重新消费
func TestDLQReplayRepublishes(t *testing.T) {
	conn, cleanup := startNatsContainer(t)
	defer cleanup()

	l := testNatsLogger(t)
	ctx := context.Background()

	// 创建发布器，确保事件流存在
	NewJetStreamPublisher(
		conn,
		"dlq-replay-service",
		[]string{"MockOrder"},
		l,
	)

	bus := NewJetStreamBus(
		conn,
		"dlq-replay-consumer",
		"dlq-replay-service",
		l,
	).(*JetStreamBus)

	// 直接写入一条死信条目（模拟此前失败转入的死信），记录原始主题
	entryEvt := &MockOrderCreatedEvent{
		BaseEvent: entity.BaseEvent{
			Id:            "replay-evt-1",
			EventType:     "MockOrderCreatedEvent",
			AggregateID:   "order-replay",
			AggregateType: "MockOrder",
			Timestamp:     time.Now(),
			Version:       1,
		},
		OrderNo: "ORD-REPLAY",
	}
	base := entity.BaseEvent{
		Id:            entryEvt.Id,
		EventType:     entryEvt.EventType,
		AggregateID:   entryEvt.AggregateID,
		AggregateType: entryEvt.AggregateType,
		Timestamp:     entryEvt.Timestamp,
		Version:       entryEvt.Version,
	}
	if err := bus.dlqWriter.Write(ctx, "MockOrder.order-replay.MockOrderCreatedEvent", &base, "模拟故障"); err != nil {
		t.Fatalf("写入死信失败: %v", err)
	}

	// 注册成功 handler 接收重放事件
	var received atomic.Bool
	bus.AddHandler("MockOrder", "MockOrderCreatedEvent", func(ctx context.Context, event *entity.BaseEvent) error {
		if event.GetEventID() == "replay-evt-1" {
			received.Store(true)
		}
		return nil
	})
	if err := bus.Start(ctx); err != nil {
		t.Fatalf("启动总线失败: %v", err)
	}
	defer bus.Stop(context.Background())

	time.Sleep(1 * time.Second)

	replayed, err := bus.ReplayDLQ(ctx)
	if err != nil {
		t.Fatalf("重放死信失败: %v", err)
	}
	if replayed != 1 {
		t.Errorf("重放数量期望 1，实际 %d", replayed)
	}

	// 等待重放事件被重新消费
	deadline := time.Now().Add(10 * time.Second)
	for !received.Load() {
		if time.Now().After(deadline) {
			t.Fatal("等待重放事件被消费超时")
		}
		time.Sleep(300 * time.Millisecond)
	}
}
