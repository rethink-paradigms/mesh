package service

import "fmt"

// NotFoundError is returned when a body is not found. Maps to HTTP 404 / RPC -32001.
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string { return fmt.Sprintf("body not found: %s", e.ID) }

// ConflictError is returned when a state transition is invalid. Maps to HTTP 409 / RPC -32002.
type ConflictError struct {
	State    string
	Required string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("body state conflict: current state %s, required %s", e.State, e.Required)
}

// ValidationError is returned when input validation fails. Maps to HTTP 400 / RPC -32602.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}
