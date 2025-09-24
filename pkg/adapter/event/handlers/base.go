package handlers

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// EventHandler 事件处理器接口
type EventHandler func(ctx context.Context, event entity.Event) error
