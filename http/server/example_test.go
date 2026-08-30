package server_test

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pushkar-anand/build-with-go/http/server"
)

// Serve blocks until ctx is cancelled, then stops accepting connections and
// waits for in-flight requests to finish before returning. Cancelling on
// SIGINT/SIGTERM is what makes a deployment roll without dropping requests.
func Example() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := server.New(
		mux,
		server.WithHostPort("0.0.0.0", 8080),
		server.WithShutdownTimeout(10*time.Second),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Returns only once the drain has finished, so it is safe to exit after it.
	if err := srv.Serve(ctx); err != nil {
		slog.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}

// Binding before serving lets the caller learn the assigned port. Useful in
// tests: port 0 avoids racing to reserve a free one.
func ExampleServer_Listen() {
	srv := server.New(http.NewServeMux(), server.WithHostPort("127.0.0.1", 0))

	if err := srv.Listen(); err != nil {
		slog.Error("listen failed", slog.Any("error", err))
		return
	}

	addr := srv.Addr() // 127.0.0.1:<assigned port>

	go func() { _ = srv.Serve(context.Background()) }()

	_ = addr
}

// A streaming endpoint needs the write timeout off, since it caps how long a
// single response may take.
func ExampleWithWriteTimeout() {
	_ = server.New(
		http.NewServeMux(),
		server.WithWriteTimeout(0),
		server.WithIdleTimeout(2*time.Minute),
	)
}
