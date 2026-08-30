package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/pushkar-anand/build-with-go/logger"
)

type Server struct {
	log             *slog.Logger
	server          *http.Server
	shutdownTimeout time.Duration
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
			Addr:         fmt.Sprintf("%s:%d", defaultHost, defaultPort),
			Handler:      handler,
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimout,
		},
	}

	for _, opt := range opts {
		opt.apply(s)
	}

	return s
}

// Serve starts the HTTP server on the specified host/port.
//
// It accepts a context.Context. When the context is canceled the server stops
// accepting new connections and waits for the in-flight requests to complete,
// up to the configured shutdown timeout. Serve only returns once that drain has
// finished, so callers can safely exit as soon as it does.
func (s *Server) Serve(ctx context.Context) error {
	s.log.InfoContext(ctx, "starting server", slog.String("address", s.server.Addr))

	// Derived so that a ListenAndServe failure also releases the goroutine below.
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
			s.log.ErrorContext(shutdownCtx, "failed to shutdown server", logger.Error(err))
		}
	})

	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server.Serve: error starting server: %w", err)
	}

	// ListenAndServe returns as soon as Shutdown is called, so wait for the
	// drain to actually finish before reporting that the server has stopped.
	wg.Wait()

	return nil
}
