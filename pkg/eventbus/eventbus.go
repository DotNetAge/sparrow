package eventbus

import (
	"context"
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
	Pub(ctx context.Context, evt Event) error

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
// 受到Go语言的限制无法从字符串中直接反射类型，因此事件处理不能直接返回有具体类型的数据，只能折中使用事件类型名称
// 以及事件数据的字典型表示（JSON),而在接收方再对字典数据进行具体类型的反序化进行还原；
type EventHandler func(ctx context.Context, evt Event) error
