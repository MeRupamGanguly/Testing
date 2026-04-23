// Package contextkeys defines typed context keys to avoid collisions.
package utils

type contextKey string

const (
	// RequestIDKey is the context key for the unique request identifier.
	RequestIDKey contextKey = "request_id"
	// UserIDKey is the context key for the authenticated user ID.
	UserIDKey contextKey = "user_id"
	// UserRoleKey is the context key for the user's role (e.g., "admin", "customer").
	UserRoleKey contextKey = "user_role"
	// SessionIDKey is the context key for the session identifier.
	SessionIDKey contextKey = "session_id"
	// TraceIDKey is the context key for the OpenTelemetry trace ID.
	TraceIDKey contextKey = "trace_id"
)

// String returns the string representation of the context key.
func (c contextKey) String() string {
	return string(c)
}
