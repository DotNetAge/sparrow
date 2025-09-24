package errs

import (
	"fmt"
)

// RepositoryError 仓储错误
type RepositoryError struct {
	EntityType string
	ID         string
	Operation  string
	Message    string
	Cause      error
}

func (e *RepositoryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("repository error: %s %s, id=%s, message=%s, cause=%v", 
			e.EntityType, e.Operation, e.ID, e.Message, e.Cause)
	}
	return fmt.Sprintf("repository error: %s %s, id=%s, message=%s", 
			e.EntityType, e.Operation, e.ID, e.Message)
}

func (e *RepositoryError) Unwrap() error {
	return e.Cause
}

// NewRepositoryError 创建仓储错误
func NewRepositoryError(entityType, id, operation, message string, cause error) *RepositoryError {
	return &RepositoryError{
		EntityType: entityType,
		ID:         id,
		Operation:  operation,
		Message:    message,
		Cause:      cause,
	}
}

// NewRepositoryNotFoundError 创建未找到实体的错误
func NewRepositoryNotFoundError(entityType, id string) *RepositoryError {
	return &RepositoryError{
		EntityType: entityType,
		ID:         id,
		Operation:  "find",
		Message:    "entity not found",
	}
}

// NewRepositoryInvalidError 创建无效实体错误
func NewRepositoryInvalidError(entityType, operation, message string) *RepositoryError {
	return &RepositoryError{
		EntityType: entityType,
		ID:         "",
		Operation:  operation,
		Message:    message,
	}
}
