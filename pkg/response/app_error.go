// Package response owns the JSON envelope, pagination metadata, the RFC 9457
// problem body, and the application error type shared by every layer.
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
	Status  int     // HTTP status
	Code    string  // stable snake_case code, safe for client logic
	Title   string  // short English summary, shown in problem.title
	Message string  // Indonesian, user-facing, no internals
	Fields  []Field // optional field errors
	cause   error   // internal only — never serialized
}

// Error implements error.
func (e *AppError) Error() string {
	return e.Code + ": " + e.Message
}

// Unwrap exposes the cause for errors.Is and logging only.
func (e *AppError) Unwrap() error {
	return e.cause
}

func newError(status int, code, title, message string) *AppError {
	return &AppError{
		Status:  status,
		Code:    code,
		Title:   title,
		Message: message,
	}
}

// BadRequest is a request that cannot be parsed or is otherwise malformed.
func BadRequest(message string) *AppError {
	return newError(http.StatusBadRequest, "bad_request", "Bad Request", message)
}

// InvalidQuery is a malformed filter, range or sort parameter.
func InvalidQuery(message string) *AppError {
	return newError(http.StatusBadRequest, "invalid_query", "Invalid Query", message)
}

// Unauthorized is a missing, invalid or expired session.
func Unauthorized(message string) *AppError {
	return newError(http.StatusUnauthorized, "unauthorized", "Unauthorized", message)
}

// Forbidden is an authenticated principal the route does not allow.
func Forbidden(message string) *AppError {
	return newError(http.StatusForbidden, "forbidden", "Forbidden", message)
}

// NotFound is a row that does not exist, is soft-deleted, or belongs to
// another farm.
func NotFound(resource string) *AppError {
	return newError(http.StatusNotFound, "not_found", "Not Found", "Data "+resource+" tidak ditemukan.")
}

// Conflict is a uniqueness, foreign-key or state-transition violation.
func Conflict(message string) *AppError {
	return newError(http.StatusConflict, "conflict", "Conflict", message)
}

// Gone is a removed endpoint or a one-time link that was already used.
func Gone(message string) *AppError {
	return newError(http.StatusGone, "gone", "Gone", message)
}

// PreconditionFailed is a stale If-Match version.
func PreconditionFailed(message string) *AppError {
	return newError(http.StatusPreconditionFailed, "version_conflict", "Precondition Failed", message)
}

// PreconditionRequired is a write or delete without the If-Match header.
func PreconditionRequired() *AppError {
	return newError(http.StatusPreconditionRequired, "precondition_required",
		"Precondition Required", "Header If-Match wajib diisi.")
}

// UnsupportedMedia is a request body with the wrong content type.
func UnsupportedMedia(message string) *AppError {
	return newError(http.StatusUnsupportedMediaType, "unsupported_media_type",
		"Unsupported Media Type", message)
}

// Unprocessable is a field or cross-field validation failure. The detail is
// omitted because each field carries its own Indonesian message.
func Unprocessable(fields ...Field) *AppError {
	appErr := newError(http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "")
	appErr.Fields = fields
	return appErr
}

// TooManyRequests is the rate limit.
func TooManyRequests(message string) *AppError {
	return newError(http.StatusTooManyRequests, "too_many_requests", "Too Many Requests", message)
}

// Internal hides the cause from the client but keeps it for the log.
func Internal(cause error) *AppError {
	appErr := newError(http.StatusInternalServerError, "internal", "Internal Server Error",
		"Terjadi kesalahan pada server.")
	appErr.cause = cause
	return appErr
}

// ServiceUnavailable is an unreachable dependency such as Redis or the
// database.
func ServiceUnavailable(message string) *AppError {
	return newError(http.StatusServiceUnavailable, "service_unavailable", "Service Unavailable", message)
}

// GatewayTimeout is a dependency that exceeded its deadline.
func GatewayTimeout(message string) *AppError {
	return newError(http.StatusGatewayTimeout, "gateway_timeout", "Gateway Timeout", message)
}
