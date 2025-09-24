package entity

// DomainEvent 领域事件接口，继承自通用事件接口，添加领域特定属性
// 领域事件是在领域模型中发生的、对业务有意义的事件
type DomainEvent interface {
	Event // 继承通用事件接口
	GetAggregateID() string  // 聚合根ID
	GetAggregateType() string // 聚合根类型
	GetVersion() int         // 事件版本，用于事件溯源
}
