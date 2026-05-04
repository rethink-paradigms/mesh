package api

import (
	"encoding/json"
	"net/http"
)

// Error code constants for API error responses.
const (
	ErrCodeUnauthorized       = "unauthorized"
	ErrCodeBodyNotFound       = "body_not_found"
	ErrCodeNodeNotFound       = "node_not_found"
	ErrCodeBodyConflict       = "body_conflict"
	ErrCodeNomadUnreachable   = "nomad_unreachable"
	ErrCodeResourceExhausted  = "resource_exhausted"
	ErrCodeInternal           = "internal"
	ErrCodeBadRequest         = "bad_request"
)

// APIError represents a single error in an API response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// ErrorResponse wraps APIError as the top-level error payload.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// WriteError writes a JSON error response to the HTTP response writer.
func WriteError(w http.ResponseWriter, code string, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
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
	json.NewEncoder(w).Encode(v)
}
