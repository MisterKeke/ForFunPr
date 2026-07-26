package backend

import (
	"database/sql"
	"fmt"
)

// ValidationError identifies caller-controlled input without exposing an
// internal provider, filesystem, or SQLite error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e == nil { return "invalid input" }
	return e.Message
}

// NotFoundError is the shared domain error for update/delete targets.
type NotFoundError struct {
	Resource string
	Key      string
}

func (e *NotFoundError) Error() string {
	if e == nil { return "resource not found" }
	if e.Key == "" { return fmt.Sprintf("%s does not exist", e.Resource) }
	return fmt.Sprintf("%s %s does not exist", e.Resource, e.Key)
}

type UnsupportedPaginationError struct {
	Source string
}

func (e *UnsupportedPaginationError) Error() string {
	return fmt.Sprintf("%s pagination is not supported by the provider feed", e.Source)
}

func requireSingleMutation(result sql.Result, operation string, resource string, idempotent bool) error {
	affected, err := result.RowsAffected()
	if err != nil { return fmt.Errorf("check %s result: %w", operation, err) }
	if affected == 0 {
		if idempotent { return nil }
		return &NotFoundError{Resource: resource}
	}
	if affected != 1 {
		return fmt.Errorf("%s affected %d rows", operation, affected)
	}
	return nil
}
