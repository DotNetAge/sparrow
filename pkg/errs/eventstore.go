package errs

import (
	"fmt"
)

// EventStoreError 事件存储错误
type EventStoreError struct {
	Type      string
	Message   string
	Aggregate string
	Cause     error
}

func (e *EventStoreError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("event store error [%s]: %s (aggregate: %s), cause: %v", e.Type, e.Message, e.Aggregate, e.Cause)
	}
	return fmt.Sprintf("event store error [%s]: %s (aggregate: %s)", e.Type, e.Message, e.Aggregate)
}

// Unwrap 实现错误解包接口，使errors.As可以正确识别嵌套的错误类型
func (e *EventStoreError) Unwrap() error {
	return e.Cause
}

// NewConcurrencyConflictError 创建并发冲突错误
func NewConcurrencyConflictError(aggregateID string, expectedVersion, currentVersion int) *EventStoreError {
	return &EventStoreError{
		Type:      "concurrency_conflict",
		Message:   fmt.Sprintf("expected version %d, got %d", expectedVersion, currentVersion),
		Aggregate: aggregateID,
	}
}

// NewAggregateNotFoundError 创建聚合不存在错误
func NewAggregateNotFoundError(aggregateID string) *EventStoreError {
	return &EventStoreError{
		Type:      "aggregate_not_found",
		Message:   "aggregate not found",
		Aggregate: aggregateID,
	}
}

// NewEventStoreError 创建自定义类型的事件存储错误
func NewEventStoreError(errType, message, aggregateID string, cause error) *EventStoreError {
	return &EventStoreError{
		Type:      errType,
		Message:   message,
		Aggregate: aggregateID,
		Cause:     cause,
	}
}