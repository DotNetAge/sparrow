package projection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/messaging"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// ProjectionScheduler 定时投影任务
// 功能：定期遍历所有聚合根，根据其事件流重新计算视图（如订单详情、用户信息等）
// 优势：确保视图与实际状态一致，支持查询任意时间点的状态
// 缺点：处理速度慢，处理量大时性能下降
// 适用场景：
// 1. 系统初始化后，需要将所有历史事件重新投影以构建初始视图（重建视图）
// 2. 系统状态需要定期刷新，以确保查询到的状态是最新的
// 3. 建立特定的统计视图（如订单总数、用户活跃数等）
type ProjectionScheduler struct {
	usecase.GracefulClose
	indexer     AggregateIndexer                             // 聚合根索引器
	eventReader messaging.StreamReader                       // 事件读取器
	viewStores  map[string]usecase.Repository[entity.Entity] // 视图存储
	projectors  map[string]Projector                         // 投影逻辑（事件→视图）
	interval    time.Duration                                // 定时周期（如24小时）
	logger      *logger.Logger
	wg          sync.WaitGroup     // 用于等待所有任务完成
	ctx         context.Context    // 上下文（用于取消任务）
	cancel      context.CancelFunc // 取消函数
}

// NewProjectionScheduler 创建投影任务
func NewProjectionScheduler(
	indexer AggregateIndexer,
	eventReader messaging.StreamReader,
	viewStores map[string]usecase.Repository[entity.Entity],
	projectors map[string]Projector,
	interval time.Duration,
	logger *logger.Logger,
) *ProjectionScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProjectionScheduler{
		indexer:     indexer,
		eventReader: eventReader,
		viewStores:  viewStores,
		projectors:  projectors,
		interval:    interval,
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
	}
}

func (f *ProjectionScheduler) GetProjector(aggregateType string) (Projector, error) {
	projector, ok := f.projectors[aggregateType]
	if !ok {
		f.logger.Fatal("[投影任务]未注册聚合类型的投影器", "aggregateType", aggregateType)
		return nil, fmt.Errorf("未注册聚合类型的投影器: %s", aggregateType)
	}
	return projector, nil
}

func (f *ProjectionScheduler) GetViewStore(aggregateType string) usecase.Repository[entity.Entity] {
	viewStore, ok := f.viewStores[aggregateType]
	if !ok {
		f.logger.Fatal("[投影任务]未注册聚合类型的视图存储", "aggregateType", aggregateType)
		panic(fmt.Errorf("未注册聚合类型的视图存储: %s", aggregateType))
	}
	return viewStore
}

func (f *ProjectionScheduler) AddViewStore(aggregateType string, viewStore usecase.Repository[entity.Entity]) {
	f.viewStores[aggregateType] = viewStore
}

func (f *ProjectionScheduler) AddProjector(aggregateType string, projector Projector) {
	f.projectors[aggregateType] = projector
}

// Start 启动定时任务
func (f *ProjectionScheduler) Start(aggregateTypes ...string) {
	// 立即执行一次全量投影（可选）
	f.Run(aggregateTypes...)

	// 启动定时器，按间隔执行
	ticker := time.NewTicker(f.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-f.ctx.Done():
				return // 收到取消信号，退出
			case <-ticker.C:
				f.Run(aggregateTypes...) // 定时执行
			}
		}
	}()
}

// Run 执行全量投影（核心方法）
func (f *ProjectionScheduler) Run(aggregateTypes ...string) error {
	if len(aggregateTypes) == 0 {
		f.logger.Fatal("[投影任务]至少指定一个聚合类型")
		return errors.New("至少指定一个聚合类型")
	}

	// 遍历所有聚合类型，并行处理（提高效率）
	var errCh = make(chan error, len(aggregateTypes))
	for _, aggType := range aggregateTypes {
		f.wg.Add(1)
		go func(aggType string) {
			defer f.wg.Done()
			if err := f.processAggregateType(aggType); err != nil {
				f.logger.Error("[投影任务]处理聚合类型 %s 失败", aggType, "error", err)
				errCh <- fmt.Errorf("[投影任务]处理聚合类型 %s 失败: %w", aggType, err)
			}
		}(aggType)
	}

	// 等待所有聚合类型处理完成，收集错误
	go func() {
		f.wg.Wait()
		close(errCh)
	}()

	// 汇总错误（只返回第一个错误，或组合错误）
	var allErrors error
	for err := range errCh {
		allErrors = errors.Join(allErrors, err)
	}
	return allErrors
}

// processAggregateType 处理单个聚合类型的全量投影
func (f *ProjectionScheduler) processAggregateType(aggregateType string) error {
	// 1. 获取该聚合类型的所有ID
	aggIDs, err := f.indexer.GetAllAggregateIDs(aggregateType)
	if err != nil {
		f.logger.Fatal("[投影任务]获取聚合根ID失败", "aggregateType", aggregateType, "error", err)
		return fmt.Errorf("获取聚合根ID失败: %w", err)
	}
	if len(aggIDs) == 0 {
		f.logger.Info(fmt.Sprintf("[投影任务]聚合类型 %s 无有效聚合根，跳过", aggregateType), "aggregateType", aggregateType)
		return nil
	}

	// 2. 并发处理每个聚合根（控制并发数，避免压垮系统）
	const maxConcurrent = 10 // 最大并发数
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var errCh = make(chan error, len(aggIDs))

	for _, aggID := range aggIDs {
		sem <- struct{}{} // 控制并发
		wg.Add(1)
		go func(aggType, aggID string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := f.replayAndProject(aggType, aggID); err != nil {
				f.logger.Error("[投影任务]聚合根 %s:%s 处理失败", aggType, aggID, "error", err)
				errCh <- fmt.Errorf("[投影任务]聚合根 %s:%s 处理失败: %w", aggType, aggID, err)
			}
		}(aggregateType, aggID)
	}

	// 等待所有聚合根处理完成
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// 收集错误
	var allErrors error
	for err := range errCh {
		allErrors = errors.Join(allErrors, err)
	}
	return allErrors
}

// replayAndProject 重放单个聚合根的事件流并更新视图
func (f *ProjectionScheduler) replayAndProject(aggregateType, aggregateID string) error {
	// 1. 读取该聚合根的所有事件（按版本排序）
	events, err := f.eventReader.GetEvents(f.ctx, aggregateID)
	if err != nil {
		f.logger.Error("[投影任务]读取事件失败", "aggregateID", aggregateID, "error", err)
		return fmt.Errorf("读取事件失败: %w", err)
	}
	if len(events) == 0 {
		f.logger.Info(fmt.Sprintf("[投影任务]聚合根 %s:%s 无事件，跳过", aggregateType, aggregateID), "aggregateID", aggregateID)
		return nil
	}

	// 2. 执行投影逻辑，生成视图
	projector, err := f.GetProjector(aggregateType)
	if err != nil {
		f.logger.Error("[投影任务]获取投影器失败", "aggregateID", aggregateID, "error", err)
		return fmt.Errorf("获取投影器失败: %w", err)
	}
	view, err := projector.Project(f.ctx, aggregateType, events)
	if err != nil {
		f.logger.Error("[投影任务]投影失败", "aggregateID", aggregateID, "error", err)
		return fmt.Errorf("投影失败: %w", err)
	}

	viewStore := f.GetViewStore(aggregateType)
	// 3. 写入视图存储（通常用UPSERT确保幂等性）
	if err := viewStore.Save(f.ctx, view); err != nil {
		f.logger.Error("[投影任务]更新视图失败", "aggregateID", aggregateID, "error", err)
		return fmt.Errorf("更新视图失败: %w", err)
	}

	f.logger.Info(fmt.Sprintf("[投影任务]聚合根 %s:%s 全量投影完成，事件数: %d", aggregateType, aggregateID, len(events)), "aggregateID", aggregateID)
	return nil
}

// Stop 停止定时任务
func (f *ProjectionScheduler) Stop() {
	f.cancel()  // 发送取消信号
	f.wg.Wait() // 等待当前任务完成
	f.logger.Info("[投影任务]全量投影任务已停止")
}

func (f *ProjectionScheduler) Close() error {
	f.Stop()
	return nil
}
