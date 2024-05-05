package server

import (
	"fmt"
	"log/slog"
	"time"
)

type (
	Option interface {
		apply(*Server)
	}

	optionFunc func(*Server)
)

func (fn optionFunc) apply(s *Server) {
	fn(s)
}

// WithHostPort sets the host and port for the server
func WithHostPort(addr string, port int) Option {
	return optionFunc(func(s *Server) {
		s.server.Addr = fmt.Sprintf("%s:%d", addr, port)
	})
}

// WithReadTimeout sets the read timeout for the server
func WithReadTimeout(d time.Duration) Option {
	return optionFunc(func(s *Server) {
		s.server.ReadTimeout = d
	})
}

// WithWriteTimeout sets the write timeout for the server
func WithWriteTimeout(d time.Duration) Option {
	return optionFunc(func(s *Server) {
		s.server.WriteTimeout = d
	})
}

// WithLogger can be used to set a custom slog handler for the logs of the server
func WithLogger(log *slog.Logger) Option {
	return optionFunc(func(s *Server) {
		s.log = log
	})
}
