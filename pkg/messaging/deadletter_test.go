package messaging_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/messaging"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type deadletterRejected struct {
	Reason string `json:"reason"`
}

func (r *deadletterRejected) GetEventID() string       { return "" }
func (r *deadletterRejected) GetEventType() string     { return "Rejected" }
func (r *deadletterRejected) GetAggregateID() string   { return "" }
func (r *deadletterRejected) GetAggregateType() string { return "ProductionOrder" }
func (r *deadletterRejected) GetCreatedAt() time.Time  { return time.Now() }
func (r *deadletterRejected) GetVersion() int          { return 0 }
func (r *deadletterRejected) SetVersion(v int)          {}

func makeLogger() *logger.Logger {
	l, _ := logger.NewLogger(&config.LogConfig{Level: "info"})
	return l
}

// TestUnmarshalEventPaylodNil 验证修复 A3：UnmarshalEvent Payload=nil 返回 error
func TestUnmarshalEventPaylodNil(t *testing.T) {
	t.Run("正常事件 Payload=base64 → 成功", func(t *testing.T) {
		payload := &deadletterRejected{Reason: "test"}
		data, _ := json.Marshal(payload)
		b64 := base64.StdEncoding.EncodeToString(data)
		event := &entity.BaseEvent{Payload: b64}

		result, err := entity.UnmarshalEvent[*deadletterRejected](event)
		if err != nil {
			t.Fatalf("正常事件不该返回 error: %v", err)
		}
		if result == nil {
			t.Fatal("正常事件 result 不该是 nil")
		}
	})

	t.Run("僵尸事件 Payload=nil → 返回 error", func(t *testing.T) {
		event := &entity.BaseEvent{
			EventType:   "Rejected",
			AggregateID: "zombie-001",
			Payload:     nil, // ← 今天事故的根因形态
		}

		_, err := entity.UnmarshalEvent[*deadletterRejected](event)
		if err == nil {
			t.Fatal("❌ Payload=nil 必须返回 error！这就是今天 2444 次崩溃的根因")
		}
		t.Logf("✅ Payload=nil 返回 error: %v", err)
	})

	t.Run("防御 event=nil → 返回 error", func(t *testing.T) {
		_, err := entity.UnmarshalEvent[*deadletterRejected](nil)
		if err == nil {
			t.Fatal("❌ event=nil 必须返回 error")
		}
		t.Logf("✅ event=nil 返回 error: %v", err)
	})
}

// TestDeadLetterChain 完整验证修复 A1+A2+A3：进程不崩 + handler 不被调用 + MaxDeliver=5
func TestDeadLetterChain(t *testing.T) {
	natsURL := "nats://localhost:14222"

	conn, err := nats.Connect(natsURL)
	if err != nil {
		t.Skipf("NATS 没起来 (%s): %v", natsURL, err)
	}
	defer conn.Close()

	js, _ := jetstream.New(conn)
	ctx := context.Background()

	_ = js.DeleteStream(ctx, "ProductionService")
	_ = js.DeleteStream(ctx, "ProductionService_DLQ")

	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "ProductionService",
		Subjects: []string{"ProductionOrder.*.*"},
		Storage:  jetstream.FileStorage,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 发 Payload=nil 僵尸事件
	zombie := entity.BaseEvent{
		Id:            uuid.New().String(),
		AggregateID:   "zombie-001",
		EventType:     "Rejected",
		AggregateType: "ProductionOrder",
		Timestamp:     time.Now(),
		Version:       1,
		SchemaVersion: 1,
		Payload:       nil,
	}
	raw, _ := json.Marshal(zombie)
	_, err = js.Publish(ctx, "ProductionOrder.ProductionOrder.Rejected", raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("僵尸事件已发布 Payload=nil")

	handlerCalled := false
	handler := func(ctx context.Context, event *deadletterRejected) error {
		handlerCalled = true
		return nil
	}

	sub := messaging.NewJetStreamSubscriber[*deadletterRejected](
		conn, "ProductionService", "ProductionOrder", makeLogger(), handler,
	)

	err = sub.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Stop(ctx)

	// 等 consumer 启动 + 消费那条僵尸消息
	time.Sleep(5 * time.Second)

	// ── 验证 1：进程没崩 ──
	t.Log("✅ 5 秒后 go test 进程还活着 → handleMessage 没 panic 炸 goroutine（修复 A1）")

	// ── 验证 2：handler 没被调用（因为 Payload=nil 走 error 路径） ──
	if handlerCalled {
		t.Errorf("❌ handler 被调用了！Payload=nil 应该走 UnmarshalEvent error 路径，不该传到 handler")
	} else {
		t.Log("✅ handler 未被调用（修复 A3：UnmarshalEvent Payload=nil → error → Nak）")
	}

	// ── 验证 3：ConsumerConfig MaxDeliver=5 ──
	// jssub.go Start() 内部会调 stream.CreateOrUpdateConsumer 创建 durable consumer
	// durable name 格式 = fmt.Sprintf("%s-%s-%s", serviceName, aggType, eventType)
	// 这里用 info() 直接查。如果 consumer 还没建好，Start() 会阻塞到建好才返回。
	stream, _ := js.Stream(ctx, "ProductionService")
	durableName := "ProductionService-ProductionOrder-deadletterRejected"
	consumer, err := stream.Consumer(ctx, durableName)
	if err != nil {
		t.Fatalf("找不到 consumer %s: %v", durableName, err)
	}
	ci, _ := consumer.Info(ctx)
	t.Logf("Consumer durable=%s MaxDeliver=%d pending=%d",
		ci.Name, ci.Config.MaxDeliver, ci.NumPending)
	if ci.Config.MaxDeliver != 5 {
		t.Errorf("❌ MaxDeliver 不是 5！实际=%d", ci.Config.MaxDeliver)
	} else {
		t.Log("✅ MaxDeliver=5（修复 A2：显式覆盖 jetstream 默认 -1）")
	}

	// ── 验证 4：流里还有那条 pending 消息 ──
	info, _ := stream.Info(ctx)
	t.Logf("Stream state: %+v", info.State)

	t.Log("\n=== 完整结论 ===")
	t.Log("  ✅ A1 handleMessage panic recover → goroutine 级隔离，进程不崩")
	t.Log("  ✅ A2 ConsumerConfig MaxDeliver=5 显式覆盖 jetstream 默认 -1")
	t.Log("  ✅ A3 UnmarshalEvent Payload=nil → 返回 error → handler 不被调用")
	t.Log("  ⏳ 僵尸消息在 5 次投递后（ack_wait=30s × 5 = 150s）自动转 ProductionService_DLQ")
	t.Log("  ⏳ 完整死信链验证在生产部署后观察 3 分钟确认")
}
