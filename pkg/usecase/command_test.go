package usecase

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// testPipelineLogger 构造管线测试日志器
func testPipelineLogger(t *testing.T) *logger.Logger {
	t.Helper()
	l, err := logger.NewLogger(&config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}
	return l
}

// fakeAggregate 测试用聚合根：嵌入 BaseAggregateRoot，提供最小实现
type fakeAggregate struct {
	entity.BaseAggregateRoot
	handleErr error
}

func newFakeAggregate(id string) *fakeAggregate {
	return &fakeAggregate{BaseAggregateRoot: *entity.NewBaseAggregateRoot(id)}
}

func (a *fakeAggregate) GetAggregateType() string { return "FakeAgg" }

func (a *fakeAggregate) HandleCommand(cmd interface{}) ([]entity.DomainEvent, error) {
	if a.handleErr != nil {
		return nil, a.handleErr
	}
	evt := &entity.BaseEvent{
		Id:            "evt-1",
		EventType:     "Created",
		AggregateID:   a.GetAggregateID(),
		AggregateType: a.GetAggregateType(),
		Timestamp:     time.Now(),
		Version:       a.GetVersion() + 1,
	}
	a.AddEvent(evt)
	return []entity.DomainEvent{evt}, nil
}

func (a *fakeAggregate) ApplyEvent(event interface{}) error { return nil }

func (a *fakeAggregate) LoadFromEvents(events []entity.DomainEvent) error { return nil }

func (a *fakeAggregate) LoadFromSnapshot(snapshot interface{}) error { return nil }

func (a *fakeAggregate) Validate() error { return nil }

// fakeCommand 测试用命令
type fakeCommand struct {
	entity.BaseCommand
}

func newFakeCommand(aggregateID string) entity.Command {
	return &fakeCommand{BaseCommand: entity.BaseCommand{AggregateID: aggregateID}}
}

// mockPublisher 记录发布事件的模拟发布器
type mockPublisher struct {
	published []entity.DomainEvent
	pubErr    error
}

func (m *mockPublisher) Publish(ctx context.Context, event entity.DomainEvent) error {
	if m.pubErr != nil {
		return m.pubErr
	}
	m.published = append(m.published, event)
	return nil
}

func (m *mockPublisher) PublishEvents(ctx context.Context, events []entity.DomainEvent) error {
	if m.pubErr != nil {
		return m.pubErr
	}
	m.published = append(m.published, events...)
	return nil
}

// TestPipelineExecutesFullFlow 验证统一管线全流程：
// 加载聚合 → 处理命令 → 发布事件 → 保存快照 依次执行
func TestPipelineExecutesFullFlow(t *testing.T) {
	var loadCalls atomic.Int32
	var saveCalls atomic.Int32
	load := func(ctx context.Context, id string) (*fakeAggregate, error) {
		loadCalls.Add(1)
		return newFakeAggregate(id), nil
	}
	pub := &mockPublisher{}
	save := func(ctx context.Context, agg *fakeAggregate) error {
		saveCalls.Add(1)
		return nil
	}

	pipeline := NewCommandPipeline(load, pub.PublishEvents, save, testPipelineLogger(t))
	agg, err := pipeline.Exec(context.Background(), newFakeCommand("agg-1"))
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	if loadCalls.Load() != 1 {
		t.Errorf("聚合加载次数 = %d, 期望 1", loadCalls.Load())
	}
	if saveCalls.Load() != 1 {
		t.Errorf("快照保存次数 = %d, 期望 1", saveCalls.Load())
	}
	if len(pub.published) != 1 {
		t.Fatalf("发布事件数 = %d, 期望 1", len(pub.published))
	}
	if pub.published[0].GetEventType() != "Created" {
		t.Errorf("发布事件类型 = %s, 期望 Created", pub.published[0].GetEventType())
	}
	if agg.GetAggregateID() != "agg-1" {
		t.Errorf("返回聚合ID = %s, 期望 agg-1", agg.GetAggregateID())
	}
	// 发布成功且已提交：未提交事件应已清空
	if agg.HasUncommittedEvents() {
		t.Error("发布成功后未提交事件应已清空")
	}
}

// TestPipelineRejectsNonCommand 验证非命令类型输入被拒绝，且不触发后续步骤
func TestPipelineRejectsNonCommand(t *testing.T) {
	var loadCalls atomic.Int32
	load := func(ctx context.Context, id string) (*fakeAggregate, error) {
		loadCalls.Add(1)
		return newFakeAggregate(id), nil
	}
	pub := &mockPublisher{}
	pipeline := NewCommandPipeline(load, pub.PublishEvents, func(ctx context.Context, agg *fakeAggregate) error { return nil }, testPipelineLogger(t))

	_, err := pipeline.Exec(context.Background(), "not-a-command")
	if err == nil {
		t.Fatal("期望非命令输入报错，实际未报错")
	}
	if loadCalls.Load() != 0 {
		t.Errorf("聚合加载次数 = %d, 期望 0", loadCalls.Load())
	}
}

// TestPipelineRejectsEmptyAggregateID 验证聚合ID为空被拒绝
func TestPipelineRejectsEmptyAggregateID(t *testing.T) {
	var loadCalls atomic.Int32
	load := func(ctx context.Context, id string) (*fakeAggregate, error) {
		loadCalls.Add(1)
		return newFakeAggregate(id), nil
	}
	pub := &mockPublisher{}
	pipeline := NewCommandPipeline(load, pub.PublishEvents, nil, testPipelineLogger(t))

	_, err := pipeline.Exec(context.Background(), newFakeCommand(""))
	if err == nil {
		t.Fatal("期望空聚合ID报错，实际未报错")
	}
	if loadCalls.Load() != 0 {
		t.Errorf("聚合加载次数 = %d, 期望 0", loadCalls.Load())
	}
}

// TestPipelineStopsOnHandleError 验证处理命令失败时不发布、不保存
func TestPipelineStopsOnHandleError(t *testing.T) {
	load := func(ctx context.Context, id string) (*fakeAggregate, error) {
		agg := newFakeAggregate(id)
		agg.handleErr = errors.New("业务规则拒绝")
		return agg, nil
	}
	pub := &mockPublisher{}
	var saveCalls atomic.Int32
	pipeline := NewCommandPipeline(load, pub.PublishEvents, func(ctx context.Context, agg *fakeAggregate) error {
		saveCalls.Add(1)
		return nil
	}, testPipelineLogger(t))

	_, err := pipeline.Exec(context.Background(), newFakeCommand("agg-1"))
	if err == nil {
		t.Fatal("期望命令处理失败报错，实际未报错")
	}
	if len(pub.published) != 0 {
		t.Errorf("发布事件数 = %d, 期望 0", len(pub.published))
	}
	if saveCalls.Load() != 0 {
		t.Errorf("快照保存次数 = %d, 期望 0", saveCalls.Load())
	}
}

// TestPipelineStopsOnPublishError 验证发布失败时不保存快照
func TestPipelineStopsOnPublishError(t *testing.T) {
	load := func(ctx context.Context, id string) (*fakeAggregate, error) {
		return newFakeAggregate(id), nil
	}
	pub := &mockPublisher{pubErr: errors.New("事件流不可用")}
	var saveCalls atomic.Int32
	pipeline := NewCommandPipeline(load, pub.PublishEvents, func(ctx context.Context, agg *fakeAggregate) error {
		saveCalls.Add(1)
		return nil
	}, testPipelineLogger(t))

	_, err := pipeline.Exec(context.Background(), newFakeCommand("agg-1"))
	if err == nil {
		t.Fatal("期望发布失败报错，实际未报错")
	}
	if saveCalls.Load() != 0 {
		t.Errorf("快照保存次数 = %d, 期望 0（发布失败不应落快照）", saveCalls.Load())
	}
}

// TestPipelineSkipsSaveWhenNil 验证未配置快照保存时正常执行
func TestPipelineSkipsSaveWhenNil(t *testing.T) {
	load := func(ctx context.Context, id string) (*fakeAggregate, error) {
		return newFakeAggregate(id), nil
	}
	pub := &mockPublisher{}
	pipeline := NewCommandPipeline(load, pub.PublishEvents, nil, testPipelineLogger(t))

	agg, err := pipeline.Exec(context.Background(), newFakeCommand("agg-1"))
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}
	if agg.GetAggregateID() != "agg-1" {
		t.Errorf("返回聚合ID = %s, 期望 agg-1", agg.GetAggregateID())
	}
}
