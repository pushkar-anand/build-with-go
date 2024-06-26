package server

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServer_Serve(t *testing.T) {
	ctx := context.Background()

	t.Run("test server start with handler", func(t *testing.T) {
		ctx, cancelFunc := context.WithCancel(ctx)
		defer cancelFunc()

		port, err := getFreePortForTest()
		if err != nil {
			t.Fatalf("failed to get free port: %v", err)
		}

		h := getTestHandler()
		s := New(h, WithHostPort("0.0.0.0", port))

		go func() {
			err := s.Serve(ctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		}()

		waitUntilPortIsOpen(t, port)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/", s.server.Addr), nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("unexpected status code: %d", resp.StatusCode)
			return
		}
	})
}

func getTestHandler() http.Handler {
	m := http.NewServeMux()

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return m
}

// getFreePortForTest asks the kernel for a free open port that is ready to use.
func getFreePortForTest() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, fmt.Errorf("GetFreePort: error resolving tcp address: %w", err)
	}

	tcp, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("GetFreePort: error listening to tcp: %w", err)
	}

	defer func() { _ = tcp.Close() }()

	return tcp.Addr().(*net.TCPAddr).Port, nil
}

func waitUntilPortIsOpen(t *testing.T, port int) {
	t.Helper()

	addr := fmt.Sprintf("localhost:%d", port)

	// Create a context with timeout of 2 seconds.
	// If the port is not open within 2 seconds, the test will fail.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Keep trying to connect to the port until the context times out.
	for {
		select {
		case <-ctx.Done():
			assert.Fail(t, "waitUntilPortIsOpen: port is still not open")
			return
		default:
			conn, err := net.Dial("tcp", addr)
			if err == nil {
				_ = conn.Close()
				return
			}

			<-time.Tick(100 * time.Millisecond)
		}
	}
}
