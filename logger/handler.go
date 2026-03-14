package logger

import (
	"context"
	"log/slog"

	"github.com/pushkar-anand/build-with-go/ctxval"
)

// contextHandler is a slog.Handler that adds context values to log records.
type contextHandler struct {
	slog.Handler
}

// Handle adds context values to the log record before passing it to the underlying handler.
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID, ok := ctxval.RequestIDFromContext(ctx); ok {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes, retaining the context wrapper.
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group, retaining the context wrapper.
func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{h.Handler.WithGroup(name)}
}
