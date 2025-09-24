package entity

type Command interface {
	GetCommandID() string
	GetAggregateID() string
}

type BaseCommmand struct {
	CommandID   string `json:"command_id"`
	AggregateID string `json:"aggregate_id"`	
}

func (c *BaseCommmand) GetCommandID() string {
	return c.CommandID
}

func (c *BaseCommmand) GetAggregateID() string {
	return c.AggregateID
}