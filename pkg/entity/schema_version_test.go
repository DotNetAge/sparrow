package entity

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// testFactEvent 测试用事件负载：仅承载事实字段
type testFactEvent struct {
	OrderNo string `json:"order_no"`
	Amount  int    `json:"amount"`
}

// TestNewBaseEventHasSchemaVersion 验证新建事件的默认契约版本为 1
func TestNewBaseEventHasSchemaVersion(t *testing.T) {
	ev := NewBaseEvent("agg-1", "OrderCreated", "Order", 1)
	if ev.SchemaVersion != 1 {
		t.Errorf("期望默认契约版本为 1，实际: %d", ev.SchemaVersion)
	}
}

// TestUnmarshalEventRejectsHigherSchemaVersion 验证未知的更高契约版本被明确拒绝，
// 为未来事件迁移钩子留位
func TestUnmarshalEventRejectsHigherSchemaVersion(t *testing.T) {
	payload, err := json.Marshal(testFactEvent{OrderNo: "ORD-001", Amount: 100})
	if err != nil {
		t.Fatalf("序列化负载失败: %v", err)
	}

	event := &BaseEvent{
		SchemaVersion: 2, // 模拟未来版本的未知契约
		Payload:       base64.StdEncoding.EncodeToString(payload),
	}

	_, err = UnmarshalEvent[testFactEvent](event)
	if err == nil {
		t.Fatal("期望返回明确错误，实际未报错")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("错误信息应包含 schema_version 标识，实际: %v", err)
	}
}

// TestUnmarshalEventAcceptsLegacyEvent 验证无 schema_version 字段的存量事件（反序列化为 0）
// 按版本 1 处理，正常解码
func TestUnmarshalEventAcceptsLegacyEvent(t *testing.T) {
	payload, err := json.Marshal(testFactEvent{OrderNo: "ORD-002", Amount: 200})
	if err != nil {
		t.Fatalf("序列化负载失败: %v", err)
	}

	event := &BaseEvent{
		SchemaVersion: 0, // 存量事件无该字段
		Payload:       base64.StdEncoding.EncodeToString(payload),
	}

	decoded, err := UnmarshalEvent[testFactEvent](event)
	if err != nil {
		t.Fatalf("存量事件应正常解码，实际报错: %v", err)
	}
	if decoded.OrderNo != "ORD-002" || decoded.Amount != 200 {
		t.Errorf("负载还原字段不匹配: %+v", decoded)
	}
}
