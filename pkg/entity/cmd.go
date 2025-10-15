package entity

type Command interface {
	GetCommandID() string
	GetAggregateID() string
}

// BaseCommand 基础命令结构体，提供标准字段和方法
// 所有命令都应该嵌入这个结构体来实现通用的Command接口方法
// NOTES: 一切的Command都不可以重新实现GetAggregateID()方法，以及重新定义 AggregateID 字段,否则会引发系统性异常。
// 除非所有命令处理机制都重新实现，系统会默认从JSON或HTTP的Body中将id转换成AggregateID字段。
type BaseCommand struct {
	CommandID   string `json:"command_id" form:"command_id"`
	AggregateID string `json:"id" form:"id"`
}

func (c *BaseCommand) GetCommandID() string {
	return c.CommandID
}

func (c *BaseCommand) GetAggregateID() string {
	return c.AggregateID
}
