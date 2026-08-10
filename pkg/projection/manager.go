package projection

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// ProjectionCheckpoint 投影器处理游标（持久化）
// 记录某个投影器对某个聚合根已处理到的最后一个事件，
// 用于订阅端幂等（重启不重复处理）与全量重建后的续传
type ProjectionCheckpoint struct {
	entity.BaseEntity
	ProjectorName string    `json:"projector_name"` // 投影器名称（复合键前缀）
	AggregateID   string    `json:"aggregate_id"`   // 聚合根ID（复合键后缀）
	EventID       string    `json:"event_id"`       // 最后处理的事件ID
	EventVersion  int       `json:"event_version"`  // 最后处理的版本
	ProcessedAt   time.Time `json:"processed_at"`   // 处理时间
}

// EventHandler 单事件处理函数（与订阅器 handler 同类型，可直接用于 AddSub）
type EventHandler = messaging.DomainEventHandler[*entity.BaseEvent]

// projectorRegistration 投影器注册信息
type projectorRegistration struct {
	name          string
	projector     Projector
	aggregateType string
}

// ProjectionManager 读模型管理器
// 统一托管投影器：持久化 checkpoint（增量幂等）+ 全量重建（事件流投影）
type ProjectionManager struct {
	logger         *logger.Logger
	checkpointRepo usecase.Repository[*ProjectionCheckpoint]
	projectors     map[string]*projectorRegistration
}

// NewProjectionManager 创建读模型管理器
func NewProjectionManager(logger *logger.Logger, checkpointRepo usecase.Repository[*ProjectionCheckpoint]) *ProjectionManager {
	return &ProjectionManager{
		logger:         logger,
		checkpointRepo: checkpointRepo,
		projectors:     make(map[string]*projectorRegistration),
	}
}

// Register 注册投影器（用于全量重建）
func (m *ProjectionManager) Register(name string, projector Projector, aggregateType string) error {
	if name == "" {
		return fmt.Errorf("投影器名称不能为空")
	}
	if projector == nil {
		return fmt.Errorf("投影器不能为空")
	}
	if _, exists := m.projectors[name]; exists {
		return fmt.Errorf("投影器已注册: %s", name)
	}
	m.projectors[name] = &projectorRegistration{
		name:          name,
		projector:     projector,
		aggregateType: aggregateType,
	}
	return nil
}

// AggregateTypeOf 返回已注册投影器的聚合类型（供管理接口选择对应的事件流读取器）
func (m *ProjectionManager) AggregateTypeOf(name string) (string, bool) {
	reg, ok := m.projectors[name]
	if !ok {
		return "", false
	}
	return reg.aggregateType, true
}

// Checkpointed 包装增量处理函数，注入持久化 checkpoint 幂等：
//  1. 同一事件已成功处理过 → 直接跳过（不重复产生副作用）
//  2. 处理成功后写入 checkpoint（重启后不再重复）
//  3. 处理失败不写 checkpoint（可重试）
//
// 用法：hub.AddSub(aggregateType, eventType, mgr.Checkpointed("viewName", handler))
func (m *ProjectionManager) Checkpointed(projectorName string, handler EventHandler) EventHandler {
	return func(ctx context.Context, evt *entity.BaseEvent) error {
		// 已成功处理过的（聚合, 事件）不重复执行：优先精确匹配事件ID，
		// 事件ID为空（存量事件）时按版本兜底（事件版本唯一递增）
		cp, err := m.checkpointRepo.FindByID(ctx, checkpointID(projectorName, evt.GetAggregateID()))
		if err == nil {
			if cp.EventID == evt.GetEventID() && cp.EventID != "" {
				m.logger.Info("投影器已处理过该事件，跳过", "projector", projectorName, "eventID", evt.GetEventID())
				return nil
			}
			if evt.GetEventID() == "" && cp.EventVersion >= evt.GetVersion() {
				m.logger.Info("投影器已处理过该版本，跳过", "projector", projectorName, "version", evt.GetVersion())
				return nil
			}
		}

		if err := handler(ctx, evt); err != nil {
			return err
		}

		return m.saveCheckpoint(ctx, projectorName, evt.GetAggregateID(), evt)
	}
}

// Rebuild 全量重建：按聚合ID拉取完整事件流，经已注册投影器重建视图
// 事件流会先按版本升序排列再传入投影器；重建成功后推进 checkpoint 到该聚合最后一个事件
func (m *ProjectionManager) Rebuild(ctx context.Context, eventReader EventReader, projectorName string, aggregateIDs []string) ([]entity.Entity, error) {
	reg, ok := m.projectors[projectorName]
	if !ok {
		return nil, fmt.Errorf("投影器未注册: %s", projectorName)
	}

	var views []entity.Entity
	for _, aggregateID := range aggregateIDs {
		events, err := eventReader.GetAggregateEvents(ctx, aggregateID)
		if err != nil {
			m.logger.Error("全量重建读取事件流失败", "projector", projectorName, "aggregate_id", aggregateID, "error", err)
			return nil, fmt.Errorf("全量重建读取 %s 事件流失败: %w", aggregateID, err)
		}
		if len(events) == 0 {
			continue
		}

		// 事件流按版本升序排列，保证投影器重建的版本连续性
		sortedEvents := make([]entity.DomainEvent, len(events))
		copy(sortedEvents, events)
		sort.Slice(sortedEvents, func(i, j int) bool {
			return sortedEvents[i].GetVersion() < sortedEvents[j].GetVersion()
		})

		view, err := reg.projector.Project(ctx, reg.aggregateType, aggregateID, sortedEvents)
		if err != nil {
			m.logger.Error("全量重建投影失败", "projector", projectorName, "aggregate_id", aggregateID, "error", err)
			return nil, fmt.Errorf("全量重建投影器 %s 聚合 %s 失败: %w", projectorName, aggregateID, err)
		}
		views = append(views, view)

		// 推进 checkpoint 到该聚合最后一个事件
		if err := m.saveCheckpoint(ctx, projectorName, aggregateID, sortedEvents[len(sortedEvents)-1]); err != nil {
			return nil, err
		}
	}

	return views, nil
}

// RebuildAll 全量重建指定投影器的全部聚合视图（管理接口触发入口）：
// 聚合ID清单来自该投影器已持久化的 checkpoint（即增量处理过的聚合范围），
// 保证"先增量、后全量"语义下重建范围与已处理范围一致
func (m *ProjectionManager) RebuildAll(ctx context.Context, eventReader EventReader, projectorName string) (int, error) {
	if _, ok := m.projectors[projectorName]; !ok {
		return 0, fmt.Errorf("投影器未注册: %s", projectorName)
	}

	cps, err := m.checkpointRepo.FindAll(ctx)
	if err != nil {
		m.logger.Error("全量重建读取 checkpoint 列表失败", "projector", projectorName, "error", err)
		return 0, fmt.Errorf("全量重建读取 checkpoint 列表失败: %w", err)
	}

	var aggregateIDs []string
	for _, cp := range cps {
		if cp.ProjectorName == projectorName {
			aggregateIDs = append(aggregateIDs, cp.AggregateID)
		}
	}

	views, err := m.Rebuild(ctx, eventReader, projectorName, aggregateIDs)
	if err != nil {
		return 0, err
	}
	return len(views), nil
}

// saveCheckpoint 持久化投影器对该聚合的处理游标
func (m *ProjectionManager) saveCheckpoint(ctx context.Context, projectorName string, aggregateID string, evt entity.DomainEvent) error {
	cp, err := m.checkpointRepo.FindByID(ctx, checkpointID(projectorName, aggregateID))
	if err != nil {
		cp = &ProjectionCheckpoint{
			BaseEntity:   *entity.NewBaseEntity(checkpointID(projectorName, aggregateID)),
			ProjectorName: projectorName,
			AggregateID:   aggregateID,
		}
	}

	cp.EventID = evt.GetEventID()
	cp.EventVersion = evt.GetVersion()
	cp.ProcessedAt = time.Now()

	if err := m.checkpointRepo.Save(ctx, cp); err != nil {
		m.logger.Error("保存投影 checkpoint 失败", "projector", projectorName, "aggregate_id", aggregateID, "error", err)
		return fmt.Errorf("保存投影 checkpoint 失败: %w", err)
	}
	return nil
}

// checkpointID 生成 checkpoint 主键：{投影器名}:{聚合ID}
func checkpointID(projectorName, aggregateID string) string {
	return projectorName + ":" + aggregateID
}
