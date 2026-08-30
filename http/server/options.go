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

// WithReadHeaderTimeout sets how long the server allows for reading request
// headers. It bounds a connection that dribbles headers out slowly, which the
// read timeout alone does not once a request body is being read.
func WithReadHeaderTimeout(d time.Duration) Option {
	return optionFunc(func(s *Server) {
		s.server.ReadHeaderTimeout = d
	})
}

// WithWriteTimeout sets the write timeout for the server.
//
// Pass 0 to disable it. A write timeout caps the lifetime of a response, so
// streaming endpoints such as SSE need it off.
func WithWriteTimeout(d time.Duration) Option {
	return optionFunc(func(s *Server) {
		s.server.WriteTimeout = d
	})
}

// WithIdleTimeout sets how long the server keeps an idle keep-alive connection
// open while waiting for the next request.
func WithIdleTimeout(d time.Duration) Option {
	return optionFunc(func(s *Server) {
		s.server.IdleTimeout = d
	})
}

// WithMaxHeaderBytes sets the maximum size of the request headers the server
// will accept. It defaults to http.DefaultMaxHeaderBytes.
func WithMaxHeaderBytes(n int) Option {
	return optionFunc(func(s *Server) {
		s.server.MaxHeaderBytes = n
	})
}

// WithMaxHeaderValueCount sets the maximum number of header values the server
// will accept. It defaults to http.DefaultMaxHeaderValueCount.
func WithMaxHeaderValueCount(n int) Option {
	return optionFunc(func(s *Server) {
		s.server.MaxHeaderValueCount = n
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
