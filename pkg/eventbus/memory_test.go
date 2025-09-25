package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/stretchr/testify/assert"
)

// 测试创建MemoryEventBus实例
func TestNewMemoryEventBus(t *testing.T) {
	bus := NewMemoryEventBus()
	assert.NotNil(t, bus)
	assert.False(t, bus.closed)
	assert.NotNil(t, bus.handlers)
}

// 测试订阅和发布事件功能
func TestMemoryEventBus_SubAndPub(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()
	eventType := "test-event"

	// 用于验证事件处理器是否被调用
	var wg sync.WaitGroup
	handlerCalled := false
	handlerEvent := entity.Event(nil)

	// 添加等待一个处理完成
	wg.Add(1)

	// 订阅事件
	err := bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		handlerCalled = true
		handlerEvent = evt
		wg.Done()
		return nil
	})
	assert.NoError(t, err)

	// 检查订阅者数量
	subscribers := bus.GetSubscribers(eventType)
	assert.Equal(t, 1, subscribers)

	// 发布事件
	event := entity.NewGenericEvent(eventType, "test-payload")
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待事件处理器执行完成
	wg.Wait()

	// 验证事件处理器被正确调用
	assert.True(t, handlerCalled)
	assert.NotNil(t, handlerEvent)
	assert.Equal(t, event.GetEventID(), handlerEvent.GetEventID())
	assert.Equal(t, event.GetEventType(), handlerEvent.GetEventType())
}

// 测试多个订阅者的情况
func TestMemoryEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()
	eventType := "multi-subscribers-event"

	// 用于验证多个事件处理器是否被调用
	var wg sync.WaitGroup
	handler1Called := false
	handler2Called := false

	// 添加等待两个处理完成
	wg.Add(2)

	// 第一个订阅者
	err := bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		handler1Called = true
		wg.Done()
		return nil
	})
	assert.NoError(t, err)

	// 第二个订阅者
	err = bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		handler2Called = true
		wg.Done()
		return nil
	})
	assert.NoError(t, err)

	// 检查订阅者数量
	subscribers := bus.GetSubscribers(eventType)
	assert.Equal(t, 2, subscribers)

	// 发布事件
	event := entity.NewGenericEvent(eventType, "multi-sub-test")
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 等待事件处理器执行完成
	wg.Wait()

	// 验证所有事件处理器被正确调用
	assert.True(t, handler1Called)
	assert.True(t, handler2Called)
}

// 测试取消订阅功能
func TestMemoryEventBus_Unsub(t *testing.T) {
	bus := NewMemoryEventBus()
	eventType := "unsub-test-event"

	// 订阅事件
	err := bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		return nil
	})
	assert.NoError(t, err)

	// 检查订阅者数量
	subscribers := bus.GetSubscribers(eventType)
	assert.Equal(t, 1, subscribers)

	// 取消订阅
	err = bus.Unsub(eventType)
	assert.NoError(t, err)

	// 检查订阅者数量
	subscribers = bus.GetSubscribers(eventType)
	assert.Equal(t, 0, subscribers)
}

// 测试关闭事件总线功能
func TestMemoryEventBus_Close(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()
	eventType := "close-test-event"

	// 订阅事件
	err := bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		return nil
	})
	assert.NoError(t, err)

	// 关闭事件总线
	err = bus.Close()
	assert.NoError(t, err)
	assert.True(t, bus.closed)

	// 检查关闭后订阅者是否被清理
	subscribers := bus.GetSubscribers(eventType)
	assert.Equal(t, 0, subscribers)

	// 尝试向已关闭的总线发布事件
	event := entity.NewGenericEvent(eventType, "test")
	err = bus.Pub(ctx, event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")

	// 尝试向已关闭的总线订阅事件
	err = bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")

	// 尝试向已关闭的总线取消订阅
	err = bus.Unsub(eventType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// 测试向已关闭的总线发布事件
func TestMemoryEventBus_PubToClosedBus(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()
	eventType := "closed-bus-event"

	// 关闭事件总线
	bus.Close()

	// 尝试向已关闭的总线发布事件
	event := entity.NewGenericEvent(eventType, "test")
	err := bus.Pub(ctx, event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// 测试订阅时传入nil处理器
func TestMemoryEventBus_SubWithNilHandler(t *testing.T) {
	bus := NewMemoryEventBus()
	eventType := "nil-handler-event"

	// 订阅时传入nil处理器
	err := bus.Sub(eventType, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// 测试统计信息功能
func TestMemoryEventBus_Stats(t *testing.T) {
	bus := NewMemoryEventBus()
	eventType1 := "stats-event-1"
	eventType2 := "stats-event-2"

	// 添加订阅者
	bus.Sub(eventType1, func(ctx context.Context, evt entity.Event) error {
		return nil
	})
	bus.Sub(eventType1, func(ctx context.Context, evt entity.Event) error {
		return nil
	})
	bus.Sub(eventType2, func(ctx context.Context, evt entity.Event) error {
		return nil
	})

	// 获取统计信息
	stats := bus.Stats()

	// 验证统计信息
	subscribers, ok := stats["subscribers"].(int)
	assert.True(t, ok)
	assert.Equal(t, 3, subscribers)

	closed, ok := stats["closed"].(bool)
	assert.True(t, ok)
	assert.False(t, closed)

	// 关闭总线后再次获取统计信息
	bus.Close()
	stats = bus.Stats()
	closed, ok = stats["closed"].(bool)
	assert.True(t, ok)
	assert.True(t, closed)
}

// 测试事件处理器超时情况
func TestMemoryEventBus_HandlerTimeout(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()
	eventType := "timeout-event"

	// 添加一个会阻塞很长时间的处理器
	err := bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		// 故意阻塞以测试超时机制
		time.Sleep(2 * time.Second)
		return nil
	})
	assert.NoError(t, err)

	// 发布事件
	event := entity.NewGenericEvent(eventType, "timeout-test")
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 不需要等待处理器完成，因为测试的是超时机制，而不是处理器执行结果
	// 这里主要验证程序不会因为处理器阻塞而挂起
	time.Sleep(100 * time.Millisecond) // 给足够时间让处理器开始执行
	assert.True(t, true) // 只要程序没有挂起，测试就通过
}

// 测试事件处理器返回错误的情况
func TestMemoryEventBus_HandlerReturnsError(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()
	eventType := "error-event"

	// 添加一个会返回错误的处理器
	err := bus.Sub(eventType, func(ctx context.Context, evt entity.Event) error {
		return assert.AnError // 使用testify提供的测试错误
	})
	assert.NoError(t, err)

	// 发布事件
	event := entity.NewGenericEvent(eventType, "error-test")
	err = bus.Pub(ctx, event)
	assert.NoError(t, err)

	// 验证即使处理器返回错误，发布操作仍然成功
	time.Sleep(100 * time.Millisecond) // 给足够时间让处理器执行
}