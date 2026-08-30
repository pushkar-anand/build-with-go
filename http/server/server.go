// Package server runs an http.Server with a lifecycle tied to a context.
//
// Cancelling the context stops the server accepting connections and drains the
// requests already in flight. Serve returns only once that drain has finished,
// so a caller can exit as soon as it does.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pushkar-anand/build-with-go/logger"
)

// Server runs an http.Server whose lifetime follows a context.
type Server struct {
	log             *slog.Logger
	server          *http.Server
	shutdownTimeout time.Duration

	mu       sync.Mutex
	listener net.Listener
}

// New creates an instance of Server
func New(
	handler http.Handler,
	opts ...Option,
) *Server {
	s := &Server{
		log:             slog.Default(),
		shutdownTimeout: defaultShutdownTimeout,
		server: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", defaultHost, defaultPort),
			Handler:           handler,
			ReadTimeout:       defaultReadTimeout,
			ReadHeaderTimeout: defaultReadHeaderTimeout,
			WriteTimeout:      defaultWriteTimeout,
			IdleTimeout:       defaultIdleTimeout,
		},
	}

	for _, opt := range opts {
		opt.apply(s)
	}

	return s
}

// Listen binds the server's listener without accepting requests yet.
//
// Serve calls it if it has not been called already, so most callers can ignore
// it. It exists for the case where the bound address must be known before the
// server starts: binding to port 0 and then reading Addr yields a free port
// without the race of reserving one and hoping it stays free.
//
// The listener is owned by the Server once bound; Serve closes it on return.
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listen()
}

// listen must be called with s.mu held.
func (s *Server) listen() error {
	if s.listener != nil {
		return nil
	}

	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("server.Listen: error binding %s: %w", s.server.Addr, err)
	}

	s.listener = ln

	return nil
}

// Addr returns the address the server is listening on.
//
// Once the server has bound, this is the resolved address, so a server
// configured with port 0 reports the port the kernel actually assigned. Before
// then it reports the configured address.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return s.listener.Addr().String()
	}

	return s.server.Addr
}

// Serve starts the HTTP server, binding first if Listen has not already been called.
//
// It accepts a context.Context. When the context is canceled the server stops
// accepting new connections and waits for the in-flight requests to complete,
// up to the configured shutdown timeout. Serve only returns once that drain has
// finished, so callers can safely exit as soon as it does.
func (s *Server) Serve(ctx context.Context) error {
	s.mu.Lock()
	err := s.listen()
	ln := s.listener
	s.mu.Unlock()

	if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "starting server", slog.String("address", ln.Addr().String()))

	// Derived so that a Serve failure also releases the goroutine below.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	wg.Go(func() {
		<-ctx.Done()

		s.log.InfoContext(ctx, "shutting down server")

		// WithoutCancel keeps the context values while dropping the cancellation,
		// so the drain gets the full shutdown timeout.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.shutdownTimeout)
		defer cancel()

		err := s.server.Shutdown(shutdownCtx)
		if err != nil {
			s.log.ErrorContext(shutdownCtx, "failed to shutdown server", logger.Err(err))
		}
	})

	err = s.server.Serve(ln)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server.Serve: error starting server: %w", err)
	}

	// Serve returns as soon as Shutdown is called, so wait for the drain to
	// actually finish before reporting that the server has stopped.
	wg.Wait()

	return nil
}
