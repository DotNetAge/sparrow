package projection

import (
	"context"
	"fmt"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// FullProjection 全量投影器
type FullProjection struct {
	eventReader EventReader
	indexer     AggregateIndexer
}

func NewFullProjection(
	eventReader EventReader,
	indexer AggregateIndexer,
) *FullProjection {
	return &FullProjection{
		eventReader: eventReader,
		indexer:     indexer,
	}
}

// Project 对聚合类型下的所有聚合根执行全量投影
// 每个聚合根只接收属于它自己的事件流
func (p *FullProjection) Project(
	ctx context.Context,
	aggregateType string,
	projector Projector,
	events ...string) ([]entity.Entity, error) {
	if projector == nil {
		return nil, fmt.Errorf("投影器不能为空")
	}

	// 收集所有事件类型的全部事件
	var allEvents []entity.DomainEvent
	for _, eventType := range events {
		typeEvents, err := p.eventReader.GetEvents(aggregateType, eventType)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, typeEvents...)
	}

	// 获取该聚合类型下所有有效的聚合根ID
	ids, err := p.indexer.GetAllAggregateIDs(aggregateType)
	if err != nil {
		return nil, err
	}

	// 对每个聚合根ID，只将属于它的事件流传给投影器
	var entities []entity.Entity
	for _, id := range ids {
		aggEvents := make([]entity.DomainEvent, 0, len(allEvents))
		for _, event := range allEvents {
			if event.GetAggregateID() == id {
				aggEvents = append(aggEvents, event)
			}
		}
		view, err := projector.Project(ctx, aggregateType, id, aggEvents)
		if err != nil {
			return nil, err
		}
		entities = append(entities, view)
	}

	return entities, nil
}
