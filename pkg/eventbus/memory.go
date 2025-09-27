package eventbus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryEventBus 纯Go内存事件总线实现
type MemoryEventBus struct {
	handlers map[string][]*eventHandler
	mu       sync.RWMutex
	closed   bool
}

var _ EventBus = (*MemoryEventBus)(nil)

// eventHandler 包装事件处理器和元数据
type eventHandler struct {
	fn      EventHandler
	created time.Time
}

// NewMemoryEventBus 创建新的内存事件总线实例
func NewMemoryEventBus() EventBus {
	return &MemoryEventBus{
		handlers: make(map[string][]*eventHandler),
	}
}

// Pub 发布事件到所有订阅者
func (b *MemoryEventBus) Pub(ctx context.Context, evt Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	eventType := evt.EventType

	// 处理非分组订阅者
	if handlers, exists := b.handlers[eventType]; exists {
		for _, h := range handlers {
			// 异步执行处理器
			go func(h *eventHandler) {
				_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				if err := h.fn(ctx, evt); err != nil {
					// 这里可以添加错误日志
					// log.Printf("Event handler error: %v", err)
				}
			}(h)
		}
	}

	return nil
}

// Sub 订阅事件类型
func (b *MemoryEventBus) Sub(eventType string, handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	h := &eventHandler{
		fn:      handler,
		created: time.Now(),
	}

	b.handlers[eventType] = append(b.handlers[eventType], h)
	return nil
}

// Unsub 取消订阅指定类型的事件
func (b *MemoryEventBus) Unsub(eventType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	// 删除指定事件类型的所有订阅者
	delete(b.handlers, eventType)

	return nil
}

// GetSubscribers 获取订阅者信息（调试方法）
func (b *MemoryEventBus) GetSubscribers(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.handlers[eventType])
}

// Close 关闭事件总线
func (b *MemoryEventBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	// 清理所有订阅者
	b.handlers = make(map[string][]*eventHandler)
	b.closed = true

	return nil
}

// Stats 获取统计信息
func (b *MemoryEventBus) Stats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := make(map[string]interface{})

	subscribers := 0
	for _, handlers := range b.handlers {
		subscribers += len(handlers)
	}

	stats["subscribers"] = subscribers
	stats["closed"] = b.closed

	return stats
}
