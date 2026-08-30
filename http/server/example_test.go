package server_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pushkar-anand/build-with-go/http/server"
)

// Serve returns only once in-flight requests have finished, so cancelling the
// context is enough to stop cleanly.
func Example() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	srv := server.New(mux, server.WithHostPort("127.0.0.1", 0))

	// Binding before serving means Addr reports the port the kernel assigned.
	if err := srv.Listen(); err != nil {
		log.Fatal(err)
	}

	ctx, stop := context.WithCancel(context.Background())

	stopped := make(chan error, 1)
	go func() { stopped <- srv.Serve(ctx) }()

	resp, err := http.Get("http://" + srv.Addr() + "/healthz")
	if err != nil {
		log.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	_ = resp.Body.Close()

	fmt.Print(string(body))

	stop()

	if err := <-stopped; err != nil {
		log.Fatal(err)
	}

	fmt.Println("stopped cleanly")

	// Output:
	// ok
	// stopped cleanly
}

// In a real service the context comes from the signals the platform sends when
// it wants the process to go away.
func Example_gracefulShutdown() {
	srv := server.New(
		http.NewServeMux(),
		server.WithHostPort("0.0.0.0", 8080),
		server.WithShutdownTimeout(10*time.Second),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}

// A streaming endpoint needs the write timeout off, since it caps how long a
// single response may take.
func Example_streaming() {
	srv := server.New(
		http.NewServeMux(),
		server.WithWriteTimeout(0),
		server.WithIdleTimeout(2*time.Minute),
	)

	if err := srv.Serve(context.Background()); err != nil {
		log.Fatal(err)
	}
}
