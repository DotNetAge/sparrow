package entity

// AggregateRoot 聚合根接口约束
// 聚合根是领域驱动设计中的核心概念，代表一个聚合的入口点它负责维护聚合内所有实体的一致性边界并确保业务规则的执行
// 在事件溯源架构中，聚合根还负责生成、应用和管理领域事件
type AggregateRoot interface {
	Entity // 继承Entity接口

	// 聚合标识
	GetAggregateType() string // 返回聚合类型名称
	GetAggregateID() string   // 返回聚合唯一标识符
	
	// 版本控制（用于乐观锁）
	GetVersion() int         // 获取当前聚合版本
	SetVersion(version int)  // 设置聚合版本
	IncrementVersion()       // 版本号递增
	
	// 事件溯源支持
	GetUncommittedEvents() []DomainEvent      // 获取未提交的领域事件
	MarkEventsAsCommitted()                          // 标记所有事件为已提交
	AddUncommittedEvents(events []DomainEvent) // 批量添加事件到未提交事件列表
	LoadFromEvents(events []DomainEvent) error // 从事件流重建聚合状态
	LoadFromSnapshot(snapshot interface{}) error     // 从快照恢复聚合状态
	ApplyEvent(event interface{}) error             // 应用单个领域事件到聚合

	// 业务验证
	Validate() error // 验证聚合当前状态是否符合业务规则

	// 命令处理 - 聚合根的核心职责
	HandleCommand(cmd interface{}) ([]DomainEvent, error) // 处理命令并生成领域事件
}
