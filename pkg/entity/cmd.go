package entity

type Command interface {
	GetCommandID() string
	GetAggregateID() string
}

type BaseCommand struct {
	CommandID   string `json:"command_id" form:"command_id"`
	AggregateID string `json:"aggregate_id" form:"aggregate_id"`
}

func (c *BaseCommand) GetCommandID() string {
	return c.CommandID
}

func (c *BaseCommand) GetAggregateID() string {
	return c.AggregateID
}
