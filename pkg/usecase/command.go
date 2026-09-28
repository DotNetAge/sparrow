package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// CommandPipeline 统一命令执行管线
// 将各服务 usecase 中重复编排的"加载聚合 → 处理命令 → 发布事件 → 提交 → 保存快照"
// 固化为统一流程。聚合加载策略（快照优先/全量重放）、事件发布与快照保存由各服务注入，
// 从而保留各服务差异化的加载逻辑，同时消除执行骨架的重复。
type CommandPipeline[T entity.AggregateRoot] struct {
	load    func(ctx context.Context, aggregateID string) (T, error) // 聚合加载策略
	publish func(ctx context.Context, events []entity.DomainEvent) error
	save    func(ctx context.Context, agg T) error // 可选：快照保存策略
	logger  *logger.Logger
}

// NewCommandPipeline 创建统一命令管线
// load 加载聚合（必须）；publish 发布事件（必须）；save 保存快照（可选，nil 表示不保存快照）
func NewCommandPipeline[T entity.AggregateRoot](
	load func(ctx context.Context, aggregateID string) (T, error),
	publish func(ctx context.Context, events []entity.DomainEvent) error,
	save func(ctx context.Context, agg T) error,
	log *logger.Logger,
) *CommandPipeline[T] {
	return &CommandPipeline[T]{
		load:    load,
		publish: publish,
		save:    save,
		logger:  log,
	}
}

// Exec 执行命令
// 统一流程：断言命令 → 校验聚合ID → 加载聚合 → HandleCommand 生成事件 → 发布事件 → 提交 → 保存快照。
// 发布失败不落快照（避免快照超前于事件流）；命令处理失败不发布不保存。
func (p *CommandPipeline[T]) Exec(ctx context.Context, action interface{}) (T, error) {
	var zero T

	c, ok := action.(entity.Command)
	if !ok {
		err := errors.New("命令断言失败")
		p.logger.Error("命令断言失败", "error", err)
		return zero, fmt.Errorf("命令断言失败: %w", err)
	}

	if c.GetAggregateID() == "" {
		err := errors.New("聚合ID不能为空")
		p.logger.Error("命令聚合ID不能为空", "error", err)
		return zero, fmt.Errorf("命令聚合ID不能为空: %w", err)
	}

	// 加载聚合（快照优先/全量重放由服务注入的 load 策略决定）
	agg, err := p.load(ctx, c.GetAggregateID())
	if err != nil {
		p.logger.Error("加载聚合失败", "error", err)
		return zero, fmt.Errorf("加载聚合失败: %w", err)
	}

	// 处理命令并生成事件（聚合根 HandleCommand 不变）
	events, err := agg.HandleCommand(c)
	if err != nil {
		p.logger.Error("执行命令并生成事件失败", "error", err)
		return zero, fmt.Errorf("执行命令并生成事件失败: %w", err)
	}

	// 发布事件（发布即存储）
	if err := p.publish(ctx, events); err != nil {
		p.logger.Error("发布事件流失败", "error", err)
		return zero, fmt.Errorf("发布事件流失败: %w", err)
	}

	agg.MarkEventsAsCommitted()

	// 保存聚合快照（可选）
	if p.save != nil {
		if err := p.save(ctx, agg); err != nil {
			p.logger.Error("保存聚合快照失败", "error", err, "aggregate_id", agg.GetAggregateID())
			return zero, fmt.Errorf("保存聚合快照失败: %w", err)
		}
	}

	return agg, nil
}
