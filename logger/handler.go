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
//
// The client address is added only when the record lacks a client_ip of its
// own, so the request logger's line, which always sets one, does not carry it
// twice.
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID, ok := ctxval.RequestIDFromContext(ctx); ok {
		r.AddAttrs(slog.String("request_id", reqID))
	}

	if addr, ok := ctxval.ClientAddrFromContext(ctx); ok && !hasAttr(r, "client_ip") {
		r.AddAttrs(slog.String("client_ip", addr.String()))
	}

	return h.Handler.Handle(ctx, r)
}

// hasAttr reports whether r carries a top-level attribute named key.
func hasAttr(r slog.Record, key string) bool {
	found := false
	r.Attrs(func(a slog.Attr) bool {
		found = a.Key == key
		return !found
	})
	return found
}

// WithAttrs returns a new handler with the given attributes, retaining the context wrapper.
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group, retaining the context wrapper.
func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{h.Handler.WithGroup(name)}
}
