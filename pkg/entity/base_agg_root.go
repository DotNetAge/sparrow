package entity

import (
	"fmt"
	"time"
)

// BaseAggregateRoot 基础聚合根结构体，提供标准字段和方法
// 所有聚合根都应该嵌入这个结构体来实现通用的AggregateRoot接口方法
type BaseAggregateRoot struct {
	BaseEntity                      // 嵌入基础实体
	Version           int           // 版本号用于乐观锁,NOTES:聚合根的初始版本为0表示该聚合根还没有被保存到事件存储中
	uncommittedEvents []DomainEvent // 未提交的事件列表
}

// NewBaseAggregateRoot 创建新的基础聚合根
// NOTES: 聚合根的初始版本为0表示该聚合根还没有被保存到事件存储中
func NewBaseAggregateRoot(id string) *BaseAggregateRoot {
	now := time.Now()
	return &BaseAggregateRoot{
		BaseEntity: BaseEntity{
			Id:        id,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Version:           0,
		uncommittedEvents: []DomainEvent{},
	}
}

// GetVersion 返回当前版本
func (a *BaseAggregateRoot) GetVersion() int {
	return a.Version
}

// GetAggregateID 返回聚合根的ID
func (a *BaseAggregateRoot) GetAggregateID() string {
	return a.Id
}

// SetVersion 设置版本号
func (a *BaseAggregateRoot) SetVersion(version int) {
	a.Version = version
}

// IncrementVersion 版本号递增
func (a *BaseAggregateRoot) IncrementVersion() {
	a.Version++
}

// GetUncommittedEvents 获取未提交的事件，用于事件存储和发布
func (a *BaseAggregateRoot) GetUncommittedEvents() []DomainEvent {
	return a.uncommittedEvents
}

// MarkEventsAsCommitted 标记事件为已提交，清空未提交事件列表
func (a *BaseAggregateRoot) MarkEventsAsCommitted() {
	a.uncommittedEvents = []DomainEvent{}
}

// AddUncommittedEvents 批量添加事件到未提交事件列表
// NOTES: 内部方法，外部绝对不能使用，外部使用此方法就会扰乱事件流的应用与重播！
func (a *BaseAggregateRoot) AddUncommittedEvents(events []DomainEvent) {
	for _, event := range events {
		a.AddEvent(event)
	}
}

// AddEvent 添加单个事件到未提交事件列表
// NOTES:内部方法，外部绝对不能使用，外部使用此方法就会扰乱事件流的应用与重播！
func (a *BaseAggregateRoot) AddEvent(event DomainEvent) {

	if event.GetEventID() == "" {
		panic("事件ID不能为空")
	}

	if event.GetEventType() == "" {
		panic("事件类型不能为空")
	}

	if event.GetAggregateID() != a.GetAggregateID() {
		panic("事件的聚合根ID与当前聚合根ID不匹配")
	}

	if event.GetVersion() < a.Version {
		panic("事件版本小于当前聚合根版本")
	}

	for _, i := range a.uncommittedEvents {
		if i.GetEventID() == event.GetEventID() {
			fmt.Printf("存在相同的事件ID：%s，当前事件ID：%s", i.GetEventID(), event.GetEventID())
			panic("存在相同的事件ID：" + event.GetEventID())
		}
	}

	a.uncommittedEvents = append(a.uncommittedEvents, event)
}

// HasUncommittedEvents 检查是否有未提交的事件，帮助方法
func (a *BaseAggregateRoot) HasUncommittedEvents() bool {
	return len(a.uncommittedEvents) > 0
}
