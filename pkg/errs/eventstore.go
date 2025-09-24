package errs

import (
	"fmt"
)

// EventStoreError 事件存储错误
type EventStoreError struct {
	Type      string
	Message   string
	Aggregate string
}

func (e *EventStoreError) Error() string {
	return fmt.Sprintf("event store error [%s]: %s (aggregate: %s)", e.Type, e.Message, e.Aggregate)
}