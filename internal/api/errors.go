package api

import (
	"encoding/json"
	"net/http"
)

// Error code constants for API error responses.
const (
	ErrCodeUnauthorized      = "unauthorized"
	ErrCodeBodyNotFound      = "body_not_found"
	ErrCodeNodeNotFound      = "node_not_found"
	ErrCodeBodyConflict      = "body_conflict"
	ErrCodeNomadUnreachable  = "nomad_unreachable"
	ErrCodeResourceExhausted = "resource_exhausted"
	ErrCodeInternal          = "internal"
	ErrCodeBadRequest        = "bad_request"
)

// APIError represents a single error in an API response.
type APIError struct {
	Code    string `json:"code" example:"body_not_found"`
	Message string `json:"message" example:"Body with ID 'xyz' not found"`
	Status  int    `json:"status" example:"404"`
}

// ErrorResponse wraps APIError as the top-level error payload.
// @Description Standard error response payload
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// WriteError writes a JSON error response to the HTTP response writer.
func WriteError(w http.ResponseWriter, code string, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: APIError{
			Code:    code,
			Message: message,
			Status:  status,
		},
	})
}

// WriteJSON writes a JSON success response to the HTTP response writer.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
