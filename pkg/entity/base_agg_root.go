package entity

import (
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

// GetUncommittedEvents 获取未提交的事件
func (a *BaseAggregateRoot) GetUncommittedEvents() []DomainEvent {
	return a.uncommittedEvents
}

// MarkEventsAsCommitted 标记事件为已提交
func (a *BaseAggregateRoot) MarkEventsAsCommitted() {
	a.uncommittedEvents = []DomainEvent{}
}

// AddUncommittedEvents 批量添加事件到未提交事件列表
func (a *BaseAggregateRoot) AddUncommittedEvents(events []DomainEvent) {
	a.uncommittedEvents = append(a.uncommittedEvents, events...)
}

// AddEvent 添加单个事件到未提交事件列表
func (a *BaseAggregateRoot) AddEvent(event DomainEvent) {
	a.uncommittedEvents = append(a.uncommittedEvents, event)
}

// HasUncommittedEvents 检查是否有未提交的事件
func (a *BaseAggregateRoot) HasUncommittedEvents() bool {
	return len(a.uncommittedEvents) > 0
}
