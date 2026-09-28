package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"

	"github.com/nats-io/nats.go/jetstream"
)

// DLQManager 死信队列管理能力（供管理接口/测试使用）
type DLQManager interface {
	// DLQCount 查询当前死信队列中的条目数量
	DLQCount(ctx context.Context) (int, error)
	// ReplayDLQ 重放死信队列中的全部条目（按原主题重新发布），返回重放成功的条目数
	ReplayDLQ(ctx context.Context) (int, error)
}

// DLQEntry 死信条目：承载原始事件、原始主题与失败原因，用于审计与后续重放
type DLQEntry struct {
	Subject  string           `json:"subject"`
	Event    entity.BaseEvent `json:"event"`
	Cause    string           `json:"cause"`
	FailedAt time.Time        `json:"failed_at"`
}

// DLQWriter 死信队列写入器：把超限失败的事件持久化到 {serviceName}-DLQ 流
type DLQWriter struct {
	js         jetstream.JetStream
	streamName string // 死信流名：{serviceName}-DLQ（NATS 流名不允许包含点号）
	subject    string // 死信主题：{serviceName}.dlq
	logger     *logger.Logger
}

// NewDLQWriter 创建死信写入器
func NewDLQWriter(js jetstream.JetStream, serviceName string, logger *logger.Logger) *DLQWriter {
	return &DLQWriter{
		js:         js,
		streamName: serviceName + "-DLQ",
		subject:    serviceName + ".dlq",
		logger:     logger,
	}
}

// EnsureStream 确保死信流存在（幂等，已存在则忽略）
func (w *DLQWriter) EnsureStream(ctx context.Context) error {
	_, err := w.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:        w.streamName,
		Description: w.streamName + " dead letter stream",
		Subjects:    []string{w.subject},
		Retention:   jetstream.LimitsPolicy,
	})
	if err == nil || isStreamAlreadyExists(err) {
		return nil
	}
	w.logger.Error("创建死信流失败", "stream", w.streamName, "error", err)
	return fmt.Errorf("创建死信流失败: %w", err)
}

// isStreamAlreadyExists 判断是否"流已存在"错误（jetstream 不同版本错误包装不一致，兼容判断）
func isStreamAlreadyExists(err error) bool {
	return err == nil || err.Error() == "nats: stream name already in use" ||
		err.Error() == "nats: stream name already in use with a different configuration"
}

// Write 把处理失败的事件写入死信流（可审计、可重放）
func (w *DLQWriter) Write(ctx context.Context, subject string, evt *entity.BaseEvent, cause string) error {
	// 写入前确保死信流存在（幂等），避免流未初始化时直接发布失败
	if err := w.EnsureStream(ctx); err != nil {
		return err
	}
	entry := DLQEntry{
		Subject:  subject,
		Event:    *evt,
		Cause:    cause,
		FailedAt: time.Now(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		w.logger.Error("死信条目序列化失败", "error", err, "subject", subject)
		return fmt.Errorf("死信条目序列化失败: %w", err)
	}
	if _, err := w.js.Publish(ctx, w.subject, data); err != nil {
		w.logger.Error("死信写入失败", "stream", w.streamName, "error", err, "subject", subject)
		return fmt.Errorf("死信写入失败: %w", err)
	}
	w.logger.Warn("事件已转入死信队列", "stream", w.streamName, "eventID", evt.GetEventID(), "subject", subject, "cause", cause)
	return nil
}

// dlqTracker 按事件维度统计连续失败次数，达到阈值判定转死信
type dlqTracker struct {
	maxDeliver int
	failures   map[string]int // 键：{aggregateID}:{eventID}，值：连续失败次数
	mu         sync.Mutex
}

// newDLQTracker 创建失败计数器（maxDeliver 为最大投递次数，小于 1 时按默认 3）
func newDLQTracker(maxDeliver int) *dlqTracker {
	if maxDeliver < 1 {
		maxDeliver = 3
	}
	return &dlqTracker{
		maxDeliver: maxDeliver,
		failures:   make(map[string]int),
	}
}

// record 记录一次失败；返回 true 表示已达阈值（该转死信）
func (t *dlqTracker) record(eventKey string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures[eventKey]++
	return t.failures[eventKey] >= t.maxDeliver
}
