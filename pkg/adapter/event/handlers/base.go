package handlers

import (
	"context"
	"purchase-service/pkg/entity"
)

// EventHandler 事件处理器接口
type EventHandler func(ctx context.Context, event entity.Event) error
