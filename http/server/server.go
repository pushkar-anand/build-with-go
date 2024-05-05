package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	log    *slog.Logger
	server *http.Server
}

// New creates an instance of Server
func New(
	handler http.Handler,
	opts ...Option,
) *Server {
	s := &Server{
		log: slog.Default(),
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
// It accepts a context.Context. When the context is canceled, the server is shutdown
func (s *Server) Serve(ctx context.Context) error {
	s.log.InfoContext(ctx, "starting server", slog.String("address", s.server.Addr))

	go func() {
		select {
		case <-ctx.Done():
			s.log.InfoContext(ctx, "shutting down server")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := s.server.Shutdown(ctx)
			if err != nil {
				s.log.ErrorContext(ctx, "failed to shutdown server", slog.Any("error", err))
				return
			}
		}
	}()

	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server.Start: error starting server: %w", err)
	}

	return nil
}
