package projection

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/persistence/repo"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

func testProjectionLogger(t *testing.T) *logger.Logger {
	t.Helper()
	logCfg := &config.LogConfig{Level: "info", Format: "console"}
	l, err := logger.NewLogger(logCfg)
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}
	return l
}

// fakeProjector 记录被重建的聚合ID与事件流版本序列
type fakeProjector struct {
	projected []string
	versions  map[string][]int
}

func (f *fakeProjector) Project(ctx context.Context, aggregateType string, aggregateID string, events []entity.DomainEvent) (entity.Entity, error) {
	f.projected = append(f.projected, aggregateID)
	if f.versions == nil {
		f.versions = map[string][]int{}
	}
	for _, e := range events {
		f.versions[aggregateID] = append(f.versions[aggregateID], e.GetVersion())
	}
	return &viewEntity{BaseEntity: entity.BaseEntity{Id: aggregateID}}, nil
}

type viewEntity struct {
	entity.BaseEntity
}

func newTestEvent(id, aggregateID string, version int) *entity.BaseEvent {
	return &entity.BaseEvent{
		Id:            id,
		EventType:     "MockEvent",
		AggregateID:   aggregateID,
		AggregateType: "MockAgg",
		Timestamp:     time.Now(),
		Version:       version,
	}
}

// TestCheckpointedSkipsDuplicateEvent 验证同一事件重复处理只执行一次 handler
// RED→GREEN：Checkpointed 包装后，第二次处理被持久化 checkpoint 拦截
func TestCheckpointedSkipsDuplicateEvent(t *testing.T) {
	mgr := NewProjectionManager(testProjectionLogger(t), repo.NewMemoryRepository[*ProjectionCheckpoint]())

	var calls atomic.Int32
	handler := func(ctx context.Context, evt *entity.BaseEvent) error {
		calls.Add(1)
		return nil
	}
	checkpointed := mgr.Checkpointed("test-view", handler)

	evt := newTestEvent("evt-1", "agg-1", 1)

	if err := checkpointed(context.Background(), evt); err != nil {
		t.Fatalf("首次处理失败: %v", err)
	}
	// 第二次重复处理：应被 checkpoint 拦截
	if err := checkpointed(context.Background(), evt); err != nil {
		t.Fatalf("重复处理失败: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("同一事件处理 2 次，handler 期望执行 1 次，实际 %d 次", got)
	}
}

// TestCheckpointedPersistsAcrossRestart 验证 checkpoint 持久化：重启（新管理器）后重复事件仍被跳过
func TestCheckpointedPersistsAcrossRestart(t *testing.T) {
	checkpointRepo := repo.NewMemoryRepository[*ProjectionCheckpoint]()
	handler := func(ctx context.Context, evt *entity.BaseEvent) error { return nil }

	// 第一次运行：处理事件，写入 checkpoint
	mgr1 := NewProjectionManager(testProjectionLogger(t), checkpointRepo)
	if err := mgr1.Checkpointed("test-view", handler)(context.Background(), newTestEvent("evt-1", "agg-1", 1)); err != nil {
		t.Fatalf("首次处理失败: %v", err)
	}

	// 模拟重启：同一 checkpoint repo 新建管理器
	mgr2 := NewProjectionManager(testProjectionLogger(t), checkpointRepo)
	if err := mgr2.Checkpointed("test-view", handler)(context.Background(), newTestEvent("evt-1", "agg-1", 1)); err != nil {
		t.Fatalf("重启后处理失败: %v", err)
	}
	// 无副作用可断言时，验证 checkpoint 已持久化
	cp, err := checkpointRepo.FindByID(context.Background(), "test-view:agg-1")
	if err != nil {
		t.Fatalf("checkpoint 未持久化: %v", err)
	}
	if cp.EventID != "evt-1" {
		t.Errorf("checkpoint 事件ID期望 evt-1，实际 %s", cp.EventID)
	}
	if cp.EventVersion != 1 {
		t.Errorf("checkpoint 版本期望 1，实际 %d", cp.EventVersion)
	}
}

// TestCheckpointedHandlerFailureNotRecorded 验证 handler 失败时不写 checkpoint（可重试）
func TestCheckpointedHandlerFailureNotRecorded(t *testing.T) {
	mgr := NewProjectionManager(testProjectionLogger(t), repo.NewMemoryRepository[*ProjectionCheckpoint]())

	fail := func(ctx context.Context, evt *entity.BaseEvent) error { return &errs{msg: "boom"} }
	checkpointed := mgr.Checkpointed("test-view", fail)
	if err := checkpointed(context.Background(), newTestEvent("evt-1", "agg-1", 1)); err == nil {
		t.Fatal("handler 失败应返回错误")
	}
}

// TestRebuildProjectsAggregateViews 验证全量重建：按聚合ID拉事件流，经 Projector 重建视图并更新 checkpoint
func TestRebuildProjectsAggregateViews(t *testing.T) {
	checkpointRepo := repo.NewMemoryRepository[*ProjectionCheckpoint]()
	mgr := NewProjectionManager(testProjectionLogger(t), checkpointRepo)

	fp := &fakeProjector{}
	if err := mgr.Register("agg-view", fp, "MockAgg"); err != nil {
		t.Fatalf("注册投影器失败: %v", err)
	}

	reader := &fakeEventReader{
		events: map[string][]entity.DomainEvent{
			"MockEvent": {
				newTestEvent("e1", "agg-1", 1),
				newTestEvent("e3", "agg-2", 1),
				newTestEvent("e2", "agg-1", 2), // 乱序写入，验证 Rebuild 按版本升序传入投影器
			},
		},
	}

	views, err := mgr.Rebuild(context.Background(), reader, "agg-view", []string{"agg-1", "agg-2", "agg-404"})
	if err != nil {
		t.Fatalf("全量重建失败: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("重建视图期望 2 个，实际 %d", len(views))
	}

	// 校验投影器收到的事件流按版本升序
	if len(fp.projected) != 2 {
		t.Errorf("投影器期望重建 2 个聚合，实际 %d", len(fp.projected))
	}
	wantVersions := map[string][]int{"agg-1": {1, 2}, "agg-2": {1}}
	for aggID, want := range wantVersions {
		got, ok := fp.versions[aggID]
		if !ok {
			t.Errorf("投影器未收到 %s 的事件流", aggID)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("投影器收到 %s 事件数期望 %d，实际 %d", aggID, len(want), len(got))
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("投影器收到 %s 事件版本序列期望 %v，实际 %v", aggID, want, got)
				break
			}
		}
	}

	// 校验 checkpoint 已推进到最后事件
	for aggID, wantVersion := range map[string]int{"agg-1": 2, "agg-2": 1} {
		cp, err := checkpointRepo.FindByID(context.Background(), "agg-view:"+aggID)
		if err != nil {
			t.Fatalf("重建后 checkpoint 未写入 %s: %v", aggID, err)
		}
		if cp.EventVersion != wantVersion {
			t.Errorf("checkpoint %s 版本期望 %d，实际 %d", aggID, wantVersion, cp.EventVersion)
		}
	}

	// 不存在的聚合应被跳过且不写 checkpoint
	if _, err := checkpointRepo.FindByID(context.Background(), "agg-view:agg-404"); err == nil {
		t.Error("不存在的聚合不应写入 checkpoint")
	}
}

// TestRebuildAllProjectsFromCheckpoint 验证 RebuildAll：从 checkpoint 推断聚合清单，
// 仅重建该投影器已处理过的全部聚合视图（不涉及其他投影器的聚合）
func TestRebuildAllProjectsFromCheckpoint(t *testing.T) {
	checkpointRepo := repo.NewMemoryRepository[*ProjectionCheckpoint]()
	mgr := NewProjectionManager(testProjectionLogger(t), checkpointRepo)

	fp := &fakeProjector{}
	if err := mgr.Register("agg-view", fp, "MockAgg"); err != nil {
		t.Fatalf("注册投影器失败: %v", err)
	}

	// 预置该投影器的两个聚合 checkpoint，模拟增量处理过的范围
	for _, aggID := range []string{"agg-1", "agg-2"} {
		cp := &ProjectionCheckpoint{
			BaseEntity:    *entity.NewBaseEntity("agg-view:" + aggID),
			ProjectorName: "agg-view",
			AggregateID:   aggID,
			EventVersion:  1,
		}
		if err := checkpointRepo.Save(context.Background(), cp); err != nil {
			t.Fatalf("预置 checkpoint 失败: %v", err)
		}
	}
	// 其他投影器的 checkpoint 不应被重建
	other := &ProjectionCheckpoint{
		BaseEntity:    *entity.NewBaseEntity("other-view:agg-3"),
		ProjectorName: "other-view",
		AggregateID:   "agg-3",
		EventVersion:  1,
	}
	if err := checkpointRepo.Save(context.Background(), other); err != nil {
		t.Fatalf("预置其他投影器 checkpoint 失败: %v", err)
	}

	reader := &fakeEventReader{
		events: map[string][]entity.DomainEvent{
			"MockEvent": {
				newTestEvent("e1", "agg-1", 1),
				newTestEvent("e2", "agg-2", 1),
				newTestEvent("e3", "agg-3", 1), // 属于 other-view，不应被重建
			},
		},
	}

	count, err := mgr.RebuildAll(context.Background(), reader, "agg-view")
	if err != nil {
		t.Fatalf("全量重建失败: %v", err)
	}
	if count != 2 {
		t.Errorf("重建数量期望 2，实际 %d", count)
	}
	if len(fp.projected) != 2 {
		t.Errorf("投影器期望重建 2 个聚合，实际 %d", len(fp.projected))
	}
}

// TestAggregateTypeOf 验证投影器聚合类型查询（供管理接口选择事件流读取器）
func TestAggregateTypeOf(t *testing.T) {
	mgr := NewProjectionManager(testProjectionLogger(t), repo.NewMemoryRepository[*ProjectionCheckpoint]())
	if err := mgr.Register("agg-view", &fakeProjector{}, "MockAgg"); err != nil {
		t.Fatalf("注册投影器失败: %v", err)
	}

	aggType, ok := mgr.AggregateTypeOf("agg-view")
	if !ok || aggType != "MockAgg" {
		t.Errorf("聚合类型期望 MockAgg，实际 %q (ok=%v)", aggType, ok)
	}
	if _, ok := mgr.AggregateTypeOf("missing"); ok {
		t.Error("未注册的投影器应返回 ok=false")
	}
}

// ---- 测试辅助 ----

type errs struct{ msg string }

func (e *errs) Error() string { return e.msg }

// 确保 ProjectionManager 依赖的接口与真实仓库兼容
var _ usecase.Repository[*ProjectionCheckpoint] = repo.NewMemoryRepository[*ProjectionCheckpoint]()
