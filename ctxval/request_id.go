// Package ctxval carries request-scoped values through a context.
//
// The request ID placed here by http/middleware is picked up automatically by
// the logger, so every line logged during a request is attributable to it
// without handlers passing the ID around.
package ctxval

import "context"

type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID adds a request ID to the given context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext extracts the request ID from the context, if any.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}
