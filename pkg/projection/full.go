package projection

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// FullProjection 全量投影器
type FullProjection struct {
	eventReader EventReader
	indexer     AggregateIndexer
	projector   Projector
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

func (p *FullProjection) Project(
	ctx context.Context,
	aggregateType string,
	projector Projector,
	events ...string) ([]entity.Entity, error) {
	var entities []entity.Entity

	for _, event := range events {
		events, err := p.eventReader.GetEvents(aggregateType, event)
		if err != nil {
			return nil, err
		}

		ids, err := p.indexer.GetAllAggregateIDs(aggregateType)
		if err != nil {
			return nil, err
		}

		for _, id := range ids {
			entity, err := p.projector.Project(ctx, aggregateType, id, events)
			if err != nil {
				return nil, err
			}
			entities = append(entities, entity)
		}
	}

	return entities, nil
}
