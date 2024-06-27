package logger

import (
	"log/slog"
)

type Format int

const (
	FormatJSON Format = iota
	FormatText
)

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

	return slog.New(h)
}
