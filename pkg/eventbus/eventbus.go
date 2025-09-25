package eventbus

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// EventBus 定义事件总线接口
// 所有事件总线实现必须满足这个接口
// 支持发布/订阅模式
// 2023-07-15: 初始版本
// 2024-05-10: 简化接口设计，移除不必要的复杂性
type EventBus interface {
	// Pub 发布事件到事件总线
	// ctx: 上下文，用于超时控制和取消操作
	// evt: 要发布的事件
	// 返回值: 错误信息
	Pub(ctx context.Context, evt entity.Event) error

	// Sub 订阅指定类型的事件
	// eventType: 事件类型
	// handler: 事件处理器函数
	// 返回值: 错误信息
	Sub(eventType string, handler EventHandler) error

	// Unsub 取消订阅指定类型的事件
	// eventType: 事件类型
	// 返回值: 错误信息
	Unsub(eventType string) error

	// Close 关闭事件总线，释放资源
	// 返回值: 错误信息
	Close() error
}

// EventHandler 事件处理器接口
type EventHandler func(ctx context.Context, event entity.Event) error
