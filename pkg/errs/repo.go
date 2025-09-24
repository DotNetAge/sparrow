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
