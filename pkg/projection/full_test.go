package projection

import (
	"context"
	"testing"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// fakeEventReader 模拟事件读取器，按事件类型返回事件列表
type fakeEventReader struct {
	events map[string][]entity.DomainEvent
}

func (f *fakeEventReader) GetEvents(aggregateType, eventType string) ([]entity.DomainEvent, error) {
	return f.events[eventType], nil
}

func (f *fakeEventReader) GetAggregateEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	var result []entity.DomainEvent
	for _, events := range f.events {
		for _, e := range events {
			if e.GetAggregateID() == aggregateID {
				result = append(result, e)
			}
		}
	}
	return result, nil
}

// fakeIndexer 模拟聚合根索引器
type fakeIndexer struct {
	ids []string
}

func (f *fakeIndexer) GetAllAggregateIDs(aggregateType string) ([]string, error) {
	return f.ids, nil
}

// recordingProjector 记录每次投影调用收到的聚合ID与事件列表
type recordingProjector struct {
	calls map[string][]entity.DomainEvent
}

func (p *recordingProjector) Project(ctx context.Context, aggregateType, aggregateID string, events []entity.DomainEvent) (entity.Entity, error) {
	p.calls[aggregateID] = events
	return &entity.BaseEntity{Id: aggregateID}, nil
}

// TestFullProjectionFiltersEventsByAggregateID 验证全量投影时每个聚合只收到属于自身的事件
func TestFullProjectionFiltersEventsByAggregateID(t *testing.T) {
	// 构造事件：agg-1 有 2 条、agg-2 有 1 条，均为同一事件类型
	ev1 := &entity.BaseEvent{Id: "e1", AggregateID: "agg-1", EventType: "UserRegistered", AggregateType: "User", Version: 1}
	ev2 := &entity.BaseEvent{Id: "e2", AggregateID: "agg-1", EventType: "UserRegistered", AggregateType: "User", Version: 2}
	ev3 := &entity.BaseEvent{Id: "e3", AggregateID: "agg-2", EventType: "UserRegistered", AggregateType: "User", Version: 1}

	reader := &fakeEventReader{events: map[string][]entity.DomainEvent{
		"UserRegistered": {ev1, ev2, ev3},
	}}
	indexer := &fakeIndexer{ids: []string{"agg-1", "agg-2"}}
	projector := &recordingProjector{calls: make(map[string][]entity.DomainEvent)}

	fp := NewFullProjection(reader, indexer)
	entities, err := fp.Project(context.Background(), "User", projector, "UserRegistered")
	if err != nil {
		t.Fatalf("全量投影失败: %v", err)
	}

	if len(entities) != 2 {
		t.Fatalf("期望投影 2 个实体，实际: %d", len(entities))
	}

	// agg-1 只能收到属于 agg-1 的事件
	if got := len(projector.calls["agg-1"]); got != 2 {
		t.Errorf("agg-1 期望收到 2 条事件，实际: %d", got)
	}
	for _, e := range projector.calls["agg-1"] {
		if e.GetAggregateID() != "agg-1" {
			t.Errorf("agg-1 收到了不属于它的事件: %s", e.GetEventID())
		}
	}

	// agg-2 只能收到属于 agg-2 的事件
	if got := len(projector.calls["agg-2"]); got != 1 {
		t.Errorf("agg-2 期望收到 1 条事件，实际: %d", got)
	}
	for _, e := range projector.calls["agg-2"] {
		if e.GetAggregateID() != "agg-2" {
			t.Errorf("agg-2 收到了不属于它的事件: %s", e.GetEventID())
		}
	}
}
