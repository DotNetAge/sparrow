package tasks

import (
	"context"
	"fmt"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/projection"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

type FullProjectionTask struct {
	aggreateType string
	indexer      projection.AggregateIndexer       // 聚合根索引器
	reader       messaging.StreamReader            // 事件读取器
	store        usecase.Repository[entity.Entity] // 视图存储
	projector    projection.Projector              // 投影逻辑（事件→视图）
	Logger       *logger.Logger
}

func NewFullProjectionTask(
	aggreateType string,
	indexer projection.AggregateIndexer,
	reader messaging.StreamReader,
	store usecase.Repository[entity.Entity],
	projector projection.Projector,
	logger *logger.Logger) *FullProjectionTask {
	return &FullProjectionTask{
		aggreateType: aggreateType,
		indexer:      indexer,
		reader:       reader,
		store:        store,
		projector:    projector,
		Logger:       logger,
	}
}

// Handler 返回任务的处理方法
func (t *FullProjectionTask) Project(ctx context.Context) error {
	// 1. 遍历索引器中的所有聚合根ID
	aggregateIDs, err := t.indexer.GetAllAggregateIDs(t.aggreateType)
	if err != nil {
		t.Logger.Error("[投影任务]获取聚合根ID失败", "error", err)
		return fmt.Errorf("获取聚合根ID失败: %w", err)
	}

	// 2. 对每个聚合根ID执行重放和投影
	for _, aggregateID := range aggregateIDs {
		if err := t.replayAndProject(ctx, t.aggreateType, aggregateID); err != nil {
			return err // 若有错误，立即返回
		}
	}

	t.Logger.Info("[投影任务]全量投影完成，处理了所有聚合根")
	return nil
}

// replayAndProject 重放单个聚合根的事件流并更新视图
func (f *FullProjectionTask) replayAndProject(ctx context.Context, aggregateType, aggregateID string) error {
	// 1. 读取该聚合根的所有事件（按版本排序）

	events, err := f.reader.GetEvents(ctx, aggregateID)
	if err != nil {
		f.Logger.Error("[投影任务]读取事件失败", "aggregateID", aggregateID, "error", err)
		return fmt.Errorf("读取事件失败: %w", err)
	}
	if len(events) == 0 {
		f.Logger.Info(fmt.Sprintf("[投影任务]聚合根 %s:%s 无事件，跳过", aggregateType, aggregateID), "aggregateID", aggregateID)
		return nil
	}

	// 2. 执行投影逻辑，生成视图
	view, err := f.projector.Project(ctx, aggregateType, aggregateID, events)
	if err != nil {
		f.Logger.Error("[投影任务]投影失败", "aggregateID", aggregateID, "error", err)
		return fmt.Errorf("投影失败: %w", err)
	}

	// 3. 写入视图存储（通常用UPSERT确保幂等性）
	if err := f.store.Save(ctx, view); err != nil {
		f.Logger.Error("[投影任务]更新视图失败", "aggregateID", aggregateID, "error", err)
		return fmt.Errorf("更新视图失败: %w", err)
	}

	f.Logger.Info(fmt.Sprintf("[投影任务]聚合根 %s:%s 全量投影完成，事件数: %d", aggregateType, aggregateID, len(events)), "aggregateID", aggregateID)
	return nil
}
