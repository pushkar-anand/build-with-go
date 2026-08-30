// Package logger builds slog loggers, and the HTTP middleware that logs
// requests through one.
//
// Loggers built here read the request ID from the context, so a handler that
// logs with the *Context methods gets it attached without asking.
package logger

import (
	"log/slog"
)

// Format selects how log records are encoded.
type Format int

const (
	FormatJSON Format = iota
	FormatText
)

// New returns a slog.Logger that writes JSON to stderr at info level unless
// options say otherwise.
//
// The returned logger reads the request ID from the context, so records logged
// with the *Context methods carry it without the caller passing it along.
func New(options ...Option) *slog.Logger {
	c := defaultConfig()

	for _, option := range options {
		option.apply(c)
	}

	opts := &slog.HandlerOptions{
		AddSource:   c.addCaller,
		Level:       c.level,
		ReplaceAttr: nil,
	}

	var h slog.Handler

	switch c.format {
	case FormatJSON:
		h = slog.NewJSONHandler(c.writer, opts)
	case FormatText:
		h = slog.NewTextHandler(c.writer, opts)
	default:
		h = slog.NewJSONHandler(c.writer, opts)
	}

	return slog.New(&contextHandler{h})
}
