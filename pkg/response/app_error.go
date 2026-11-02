// Package response owns the RFC 9457 problem body and the application error
// type shared by handlers, services, stores and middleware.
package response

import "net/http"

// Field is one validation failure attached to a request field.
type Field struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AppError is the error type that crosses layers. It carries everything the
// client needs and keeps the internal cause out of the response body.
type AppError struct {
	Status  int
	Code    string
	Title   string
	Message string
	Fields  []Field
	cause   error
}

// Error implements error.
func (e *AppError) Error() string {
	return e.Code + ": " + e.Message
}

// Unwrap exposes the cause for errors.Is and logging only.
func (e *AppError) Unwrap() error {
	return e.cause
}

// Unauthorized is a missing, invalid or expired session.
func Unauthorized(message string) *AppError {
	return &AppError{
		Status:  http.StatusUnauthorized,
		Code:    "unauthorized",
		Title:   "Unauthorized",
		Message: message,
	}
}

// Forbidden is an authenticated principal the route does not allow.
func Forbidden(message string) *AppError {
	return &AppError{
		Status:  http.StatusForbidden,
		Code:    "forbidden",
		Title:   "Forbidden",
		Message: message,
	}
}

// Internal hides the cause from the client but keeps it for the log.
func Internal(cause error) *AppError {
	return &AppError{
		Status:  http.StatusInternalServerError,
		Code:    "internal",
		Title:   "Internal Server Error",
		Message: "Terjadi kesalahan pada server.",
		cause:   cause,
	}
}
