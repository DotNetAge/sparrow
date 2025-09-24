package handlers

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// DomainEventHandler 领域事件处理器接口
type DomainEventHandler func(ctx context.Context, event entity.DomainEvent) error
