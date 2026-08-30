package server

import (
	"fmt"
	"log/slog"
	"net"
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

// WithListener makes the server accept connections from a caller-supplied
// listener instead of binding its own. WithHostPort is ignored when set.
//
// This is mainly useful in tests, where an in-memory listener avoids real
// network I/O, and for handing the server a socket bound elsewhere.
func WithListener(ln net.Listener) Option {
	return optionFunc(func(s *Server) {
		s.listener = ln
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

// WithShutdownTimeout sets how long Serve waits for in-flight requests to
// complete after its context is canceled, before forcing the shutdown.
func WithShutdownTimeout(d time.Duration) Option {
	return optionFunc(func(s *Server) {
		s.shutdownTimeout = d
	})
}

// WithLogger can be used to set a custom slog handler for the logs of the server
func WithLogger(log *slog.Logger) Option {
	return optionFunc(func(s *Server) {
		s.log = log
	})
}
