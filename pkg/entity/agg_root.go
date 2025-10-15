package entity

// AggregateRoot 聚合根接口约束
// 聚合根是领域驱动设计中的核心概念，代表一个聚合的入口点它负责维护聚合内所有实体的一致性边界并确保业务规则的执行
// 在事件溯源架构中，聚合根还负责生成、应用和管理领域事件
type AggregateRoot interface {
	Entity // 继承Entity接口

	// 聚合标识 - 符合DDD的定义准则，每个的聚合根都拥有一个唯一的标识符
	GetAggregateType() string // 返回聚合类型名称
	GetAggregateID() string   // 返回聚合唯一标识符

	// 版本控制（用于乐观锁）
	GetVersion() int        // 获取当前聚合版本
	SetVersion(version int) // 设置聚合版本
	IncrementVersion()      // 版本号递增

	// 事件溯源支持
	HandleCommand(cmd interface{}) ([]DomainEvent, error) // 处理命令并生成领域事件,但此时并不会对聚合根本身产生任何的影响
	ApplyEvent(event interface{}) error                   // 将事件的属性应用到聚合根，让聚合根的状态与事件状态一至，并且将事件添加到未提交事件列表
	GetUncommittedEvents() []DomainEvent                  // 获取未提交的领域事件，用于事件存储和发布
	MarkEventsAsCommitted()                               // 标记所有事件为已提交，用于清空未提交事件列表

	LoadFromEvents(events []DomainEvent) error   // 从事件流重建聚合状态（又称事件重播/Replay）
	LoadFromSnapshot(snapshot interface{}) error // 从快照恢复聚合状态（对LoadFromEvents的扩展，对具有大量修改历史的聚合根进行优化，普通聚合根无需实现）

	// 业务验证
	Validate() error // 验证聚合当前状态是否符合业务规则，属于预留接口
}
