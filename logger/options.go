package logger

import (
	"io"
	"log/slog"
	"os"
)

type (
	config struct {
		level     slog.Level
		addCaller bool
		writer    io.Writer
		format    Format
	}

	Option interface {
		apply(*config)
	}

	optionFunc func(*config)
)

func (fn optionFunc) apply(s *config) {
	fn(s)
}

// WithWriter sets writer for the logger
func WithWriter(w io.Writer) Option {
	return optionFunc(func(c *config) {
		c.writer = w
	})
}

// WithFormat sets whether records are encoded as JSON or as text.
func WithFormat(format Format) Option {
	return optionFunc(func(c *config) {
		c.format = format
	})
}

// WithLevel sets the minimum level a record must reach to be logged.
func WithLevel(l slog.Level) Option {
	return optionFunc(func(c *config) {
		c.level = l
	})
}

// WithAddCaller attaches the source file and line of each call site.
func WithAddCaller() Option {
	return optionFunc(func(c *config) {
		c.addCaller = true
	})
}

func defaultConfig() *config {
	return &config{
		level:     slog.LevelDebug,
		addCaller: false,
		writer:    os.Stdout,
		format:    FormatText,
	}
}
