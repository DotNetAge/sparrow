package projection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
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
	indexer     AggregateIndexer                  // 聚合根索引器
	eventReader messaging.EventReader             // 事件读取器
	viewStore   usecase.Repository[entity.Entity] // 视图存储
	projector   Projector                         // 投影逻辑（事件→视图）
	interval    time.Duration                     // 定时周期（如24小时）
	wg          sync.WaitGroup                    // 用于等待所有任务完成
	ctx         context.Context                   // 上下文（用于取消任务）
	cancel      context.CancelFunc                // 取消函数
}

// NewProjectionScheduler 创建投影任务
func NewProjectionScheduler(
	indexer AggregateIndexer,
	eventReader messaging.EventReader,
	viewStore usecase.Repository[entity.Entity],
	projector Projector,
	interval time.Duration,
) *ProjectionScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProjectionScheduler{
		indexer:     indexer,
		eventReader: eventReader,
		viewStore:   viewStore,
		projector:   projector,
		interval:    interval,
		ctx:         ctx,
		cancel:      cancel,
	}
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
		return errors.New("至少指定一个聚合类型")
	}

	// 遍历所有聚合类型，并行处理（提高效率）
	var errCh = make(chan error, len(aggregateTypes))
	for _, aggType := range aggregateTypes {
		f.wg.Add(1)
		go func(aggType string) {
			defer f.wg.Done()
			if err := f.processAggregateType(aggType); err != nil {
				errCh <- fmt.Errorf("处理聚合类型 %s 失败: %w", aggType, err)
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
		return fmt.Errorf("获取聚合根ID失败: %w", err)
	}
	if len(aggIDs) == 0 {
		fmt.Printf("聚合类型 %s 无有效聚合根，跳过\n", aggregateType)
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
				errCh <- fmt.Errorf("聚合根 %s:%s 处理失败: %w", aggType, aggID, err)
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
		return fmt.Errorf("读取事件失败: %w", err)
	}
	if len(events) == 0 {
		fmt.Printf("聚合根 %s:%s 无事件，跳过\n", aggregateType, aggregateID)
		return nil
	}

	// 2. 执行投影逻辑，生成视图
	view, err := f.projector.Project(f.ctx, aggregateType, events)
	if err != nil {
		return fmt.Errorf("投影失败: %w", err)
	}

	// 3. 写入视图存储（通常用UPSERT确保幂等性）
	if err := f.viewStore.Save(f.ctx, view); err != nil {
		return fmt.Errorf("更新视图失败: %w", err)
	}

	fmt.Printf("聚合根 %s:%s 全量投影完成，事件数: %d\n", aggregateType, aggregateID, len(events))
	return nil
}

// Stop 停止定时任务
func (f *ProjectionScheduler) Stop() {
	f.cancel()  // 发送取消信号
	f.wg.Wait() // 等待当前任务完成
	fmt.Println("全量投影任务已停止")
}
