package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_Serve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(getTestHandler(), WithHostPort("127.0.0.1", 0))

	// Bind up front so Addr reports the assigned port and the listener is
	// already accepting: no polling, and no racing to reserve a free port.
	require.NoError(t, s.Listen())

	go func() { _ = s.Serve(ctx) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+s.Addr()+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_Serve_GracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		handlerStarted   = make(chan struct{})
		handlerCompleted atomic.Bool
	)

	m := http.NewServeMux()
	m.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		time.Sleep(300 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))

		handlerCompleted.Store(true)
	})

	s := New(
		m,
		WithHostPort("127.0.0.1", 0),
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(5*time.Second),
	)
	require.NoError(t, s.Listen())

	serveReturned := make(chan error, 1)
	go func() { serveReturned <- s.Serve(ctx) }()

	reqDone := make(chan struct{})

	go func() {
		defer close(reqDone)

		resp, err := http.Get("http://" + s.Addr() + "/slow")
		if !assert.NoError(t, err) {
			return
		}

		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "done", string(body))
	}()

	// Cancel while the request is still being served.
	<-handlerStarted
	cancel()

	select {
	case err := <-serveReturned:
		assert.NoError(t, err)
		assert.True(t, handlerCompleted.Load(),
			"Serve returned before the in-flight request finished")
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancellation")
	}

	<-reqDone
}

func TestServer_Addr(t *testing.T) {
	s := New(getTestHandler(), WithHostPort("127.0.0.1", 0))

	assert.Equal(t, "127.0.0.1:0", s.Addr(), "should report the configured address before binding")

	require.NoError(t, s.Listen())

	defer func() { _ = s.listener.Close() }()

	assert.NotEqual(t, "127.0.0.1:0", s.Addr(), "should report the assigned port after binding")
	assert.Regexp(t, `^127\.0\.0\.1:\d+$`, s.Addr())
}

func TestServer_Serve_GracefulShutdown_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ln := newPipeListener()

		var (
			handlerStarted   = make(chan struct{})
			handlerCompleted atomic.Bool
		)

		m := http.NewServeMux()
		m.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
			close(handlerStarted)

			// Fake time: the bubble advances this instantly.
			time.Sleep(300 * time.Millisecond)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("done"))

			handlerCompleted.Store(true)
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// net.Pipe is unbuffered and synchronous; deadlines would only get in
		// the way, and the drain behaviour under test does not need them.
		s := New(m, WithListener(ln), WithReadTimeout(0), WithWriteTimeout(0))

		serveReturned := make(chan error, 1)
		go func() { serveReturned <- s.Serve(ctx) }()

		client := &http.Client{Transport: &http.Transport{DialContext: ln.DialContext}}
		defer client.CloseIdleConnections()

		respReceived := make(chan struct{})

		go func() {
			defer close(respReceived)

			resp, err := client.Get("http://pipe/slow")
			if !assert.NoError(t, err) {
				return
			}

			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "done", string(body))
		}()

		// Cancel while the request is still being served.
		<-handlerStarted
		cancel()

		err := <-serveReturned
		assert.NoError(t, err)
		assert.True(t, handlerCompleted.Load(),
			"Serve returned before the in-flight request finished")

		<-respReceived
	})
}

// pipeListener is an in-memory net.Listener backed by net.Pipe.
//
// A synctest bubble only advances its clock once every goroutine is durably
// blocked, and a goroutine blocked on real network I/O never is. So a loopback
// listener would stall the bubble; connections have to stay in-process.
type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{
		conns:  make(chan net.Conn),
		closed: make(chan struct{}),
	}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })

	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

// DialContext returns the client end of a new in-memory connection, handing
// the server end to whoever is blocked in Accept.
func (l *pipeListener) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()

	select {
	case l.conns <- server:
		return client, nil
	case <-l.closed:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

func getTestHandler() http.Handler {
	m := http.NewServeMux()

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return m
}
