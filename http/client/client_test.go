package client

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pushkar-anand/build-with-go/ctxval"
	"github.com/pushkar-anand/build-with-go/http/transport"
	"github.com/pushkar-anand/build-with-go/secret"
)

// echoed is what the echo server reports about the request it received.
type echoed struct {
	Method string      `json:"method"`
	URL    string      `json:"url"`
	Header http.Header `json:"header"`
	Body   string      `json:"body"`
}

// newEchoServer answers every request with a description of it.
func newEchoServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_ = json.MarshalWrite(w, echoed{
			Method: r.Method,
			URL:    r.URL.String(),
			Header: r.Header,
			Body:   string(data),
		})
	}))
	t.Cleanup(srv.Close)

	return srv
}

func newClient(t *testing.T, opts ...Option) *Client {
	t.Helper()

	c, err := New(opts...)
	require.NoError(t, err)

	return c
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("rejects a base URL that does not parse", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithBaseURL("http://[::1"))
		assert.ErrorContains(t, err, "base URL")
	})

	t.Run("rejects a relative base URL", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithBaseURL("/v1"))
		assert.ErrorContains(t, err, "not absolute")
	})

	t.Run("defaults the timeout of its own client", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, defaultTimeout, newClient(t).httpClient.Timeout)
	})

	t.Run("keeps a supplied client's timeout, and does not modify it", func(t *testing.T) {
		t.Parallel()

		hc := &http.Client{}
		rt := http.DefaultTransport.(*http.Transport).Clone()

		c := newClient(t, WithHTTPClient(hc), WithTransport(rt))

		assert.Zero(t, c.httpClient.Timeout)
		assert.Same(t, rt, c.httpClient.Transport)
		assert.Nil(t, hc.Transport, "the caller's client was modified")
	})

	t.Run("applies WithTimeout whatever the option order", func(t *testing.T) {
		t.Parallel()

		c := newClient(t, WithTimeout(time.Second), WithHTTPClient(&http.Client{}))

		assert.Equal(t, time.Second, c.httpClient.Timeout)
	})

	t.Run("wraps the transport for debugging", func(t *testing.T) {
		t.Parallel()

		rt := http.DefaultTransport.(*http.Transport).Clone()

		c := newClient(t, WithDebug(io.Discard), WithTransport(rt))

		dt, ok := c.httpClient.Transport.(*transport.Debug)
		require.True(t, ok, "got %T", c.httpClient.Transport)
		assert.Same(t, rt, dt.Next)
	})
}

func TestClient_resolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		ref  string
		want string
	}{
		{"appends to the base path", "https://api.example.com/v1", "/posts", "https://api.example.com/v1/posts"},
		{"handles a trailing slash on the base", "https://api.example.com/v1/", "posts", "https://api.example.com/v1/posts"},
		{"keeps a trailing slash on the ref", "https://api.example.com", "/posts/", "https://api.example.com/posts/"},
		{"merges the queries", "https://api.example.com?key=a", "/posts?page=2", "https://api.example.com/posts?key=a&page=2"},
		{"keeps escaped segments", "https://api.example.com", "/files/a%2Fb", "https://api.example.com/files/a%2Fb"},
		{"leaves an absolute ref alone", "https://api.example.com/v1", "https://other.example.com/x", "https://other.example.com/x"},
		{"takes a query-only ref", "https://api.example.com/v1", "?page=2", "https://api.example.com/v1?page=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, err := newClient(t, WithBaseURL(tt.base)).resolve(tt.ref)
			require.NoError(t, err)
			assert.Equal(t, tt.want, u.String())
		})
	}

	t.Run("rejects a relative URL without a base", func(t *testing.T) {
		t.Parallel()

		_, err := newClient(t).resolve("/posts")
		assert.ErrorContains(t, err, "needs a base URL")
	})
}

func TestClient_request(t *testing.T) {
	t.Parallel()

	srv := newEchoServer(t)

	c := newClient(t,
		WithBaseURL(srv.URL),
		WithUserAgent("build-with-go-test"),
		WithDefaultHeader("X-Team", "core"),
		WithDefaultHeader("X-Env", "test"),
	)

	t.Run("sends client defaults, letting a request override them", func(t *testing.T) {
		t.Parallel()

		got, err := c.Get[echoed](t.Context(), "/", WithHeader("X-Env", "staging"))
		require.NoError(t, err)

		assert.Equal(t, http.MethodGet, got.Method)
		assert.Equal(t, "build-with-go-test", got.Header.Get("User-Agent"))
		assert.Equal(t, "core", got.Header.Get("X-Team"))
		assert.Equal(t, []string{"staging"}, got.Header.Values("X-Env"))
		assert.Equal(t, "application/json", got.Header.Get("Accept"))
	})

	t.Run("adds query parameters to those in the URL", func(t *testing.T) {
		t.Parallel()

		got, err := c.Get[echoed](t.Context(), "/search?q=go",
			WithQuery("page", "2"),
			WithQueryValues(url.Values{"tag": {"a", "b"}}),
		)
		require.NoError(t, err)

		assert.Equal(t, "/search?page=2&q=go&tag=a&tag=b", got.URL)
	})

	t.Run("sends a bearer token", func(t *testing.T) {
		t.Parallel()

		got, err := c.Get[echoed](t.Context(), "/", WithBearerToken(secret.New("tok")))
		require.NoError(t, err)

		assert.Equal(t, "Bearer tok", got.Header.Get("Authorization"))
	})

	t.Run("sends basic credentials", func(t *testing.T) {
		t.Parallel()

		got, err := c.Get[echoed](t.Context(), "/", WithBasicAuth("user", secret.New("pass")))
		require.NoError(t, err)

		user, pass, ok := (&http.Request{Header: got.Header}).BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "user", user)
		assert.Equal(t, "pass", pass)
	})

	t.Run("stops on a request func error", func(t *testing.T) {
		t.Parallel()

		boom := errors.New("boom")

		_, err := c.Get[struct{}](t.Context(), "/", WithRequestFunc(func(*http.Request) error { return boom }))
		assert.ErrorIs(t, err, boom)
	})
}

func TestClient_forwardRequestID(t *testing.T) {
	t.Parallel()

	srv := newEchoServer(t)
	ctx := ctxval.WithRequestID(t.Context(), "req-123")

	t.Run("is off by default", func(t *testing.T) {
		t.Parallel()

		got, err := newClient(t, WithBaseURL(srv.URL)).Get[echoed](ctx, "/")
		require.NoError(t, err)

		assert.Empty(t, got.Header.Get("X-Request-Id"))
	})

	t.Run("sends the context's ID when enabled", func(t *testing.T) {
		t.Parallel()

		got, err := newClient(t, WithBaseURL(srv.URL), WithForwardRequestID()).Get[echoed](ctx, "/")
		require.NoError(t, err)

		assert.Equal(t, "req-123", got.Header.Get("X-Request-Id"))
	})

	t.Run("leaves an explicit header alone", func(t *testing.T) {
		t.Parallel()

		c := newClient(t, WithBaseURL(srv.URL), WithForwardRequestID())

		got, err := c.Get[echoed](ctx, "/", WithHeader("X-Request-Id", "explicit"))
		require.NoError(t, err)

		assert.Equal(t, "explicit", got.Header.Get("X-Request-Id"))
	})
}

func TestClient_requestBody(t *testing.T) {
	t.Parallel()

	c := newClient(t, WithBaseURL(newEchoServer(t).URL))

	tests := []struct {
		name            string
		send            func() (echoed, error)
		wantContentType string
		wantBody        string
	}{
		{
			name: "a struct as JSON",
			send: func() (echoed, error) {
				return c.Post[echoed](t.Context(), "/", struct {
					N int `json:"n"`
				}{1})
			},
			wantContentType: "application/json",
			wantBody:        `{"n":1}`,
		},
		{
			name: "url.Values as a form",
			send: func() (echoed, error) {
				return c.Put[echoed](t.Context(), "/", url.Values{"a": {"1"}})
			},
			wantContentType: "application/x-www-form-urlencoded",
			wantBody:        "a=1",
		},
		{
			name: "a string as text",
			send: func() (echoed, error) {
				return c.Patch[echoed](t.Context(), "/", "hello")
			},
			wantContentType: "text/plain; charset=utf-8",
			wantBody:        "hello",
		},
		{
			name: "bytes as they are",
			send: func() (echoed, error) {
				return c.Post[echoed](t.Context(), "/", []byte{0x01, 0x02})
			},
			wantContentType: "application/octet-stream",
			wantBody:        "\x01\x02",
		},
		{
			name: "a reader, with the Content-Type from a header",
			send: func() (echoed, error) {
				return c.Post[echoed](t.Context(), "/", strings.NewReader("# hi"), WithHeader("Content-Type", "text/markdown"))
			},
			wantContentType: "text/markdown",
			wantBody:        "# hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.send()
			require.NoError(t, err)

			assert.Equal(t, tt.wantContentType, got.Header.Get("Content-Type"))
			assert.Equal(t, tt.wantBody, got.Body)
		})
	}

	t.Run("returns a JSON encoding failure", func(t *testing.T) {
		t.Parallel()

		_, err := c.Post[struct{}](t.Context(), "/", make(chan int))
		assert.ErrorContains(t, err, "encode request body")
	})
}

func TestClient_responseBody(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"id":1},{"id":2}]`)
	})
	mux.HandleFunc("GET /empty", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /bad", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `not json`)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := newClient(t, WithBaseURL(srv.URL))

	type item struct {
		ID int `json:"id"`
	}

	t.Run("decodes JSON into the requested type", func(t *testing.T) {
		t.Parallel()

		got, err := c.Get[[]item](t.Context(), "/json")
		require.NoError(t, err)
		assert.Equal(t, []item{{1}, {2}}, got)
	})

	t.Run("reads bytes and text as they are", func(t *testing.T) {
		t.Parallel()

		b, err := c.Get[[]byte](t.Context(), "/json")
		require.NoError(t, err)
		assert.Equal(t, `[{"id":1},{"id":2}]`, string(b))

		s, err := c.Get[string](t.Context(), "/json")
		require.NoError(t, err)
		assert.Equal(t, `[{"id":1},{"id":2}]`, s)
	})

	t.Run("hands over the raw response, open", func(t *testing.T) {
		t.Parallel()

		resp, err := c.Get[*http.Response](t.Context(), "/json")
		require.NoError(t, err)
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, `[{"id":1},{"id":2}]`, string(data))
	})

	t.Run("leaves the zero value for a 204", func(t *testing.T) {
		t.Parallel()

		got, err := c.Get[*item](t.Context(), "/empty")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("returns a decoding failure", func(t *testing.T) {
		t.Parallel()

		_, err := c.Get[item](t.Context(), "/bad")
		assert.ErrorContains(t, err, "decode response body")
	})
}

func TestClient_status(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /problem", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"type":"https://example.com/probs/out-of-stock","title":"Out of stock","status":409,"detail":"item is out of stock","sku":"ABC-123"}`)
	})
	mux.HandleFunc("GET /plain", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, strings.Repeat("x", 100))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Run("decodes a problem document", func(t *testing.T) {
		t.Parallel()

		_, err := newClient(t, WithBaseURL(srv.URL)).Get[struct{}](t.Context(), "/problem?api_key=hidden")

		se, ok := errors.AsType[*StatusError](err)
		require.True(t, ok, "got %v", err)

		assert.Equal(t, http.StatusConflict, se.StatusCode)
		require.NotNil(t, se.Problem)
		assert.Equal(t, "Out of stock", se.Problem.Title)
		assert.Equal(t, map[string]any{"sku": "ABC-123"}, se.Problem.Extensions)
		assert.Equal(t, srv.URL+"/problem", se.URL)
		assert.Equal(t, "client: GET "+srv.URL+"/problem: 409 Conflict: item is out of stock", se.Error())
	})

	t.Run("keeps the start of any other body", func(t *testing.T) {
		t.Parallel()

		_, err := newClient(t, WithBaseURL(srv.URL), WithMaxErrorBodySize(10)).Get[struct{}](t.Context(), "/plain")

		se, ok := errors.AsType[*StatusError](err)
		require.True(t, ok, "got %v", err)

		assert.Equal(t, http.StatusBadGateway, se.StatusCode)
		assert.Nil(t, se.Problem)
		assert.Equal(t, "xxxxxxxxxx", string(se.Body))
	})
}

// The dump must come from beneath the token transport, or it would not show
// the header actually sent.
func TestClient_debugAndToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "cookie-value"})
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)

	var out bytes.Buffer

	c := newClient(t,
		WithBaseURL(srv.URL),
		WithDebug(&out),
		WithTokenSource(transport.StaticToken(secret.New("token-value"))),
	)

	got, err := c.Post[map[string]bool](t.Context(), "/things", map[string]string{"name": "a"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"ok": true}, got)

	dump := out.String()

	assert.Contains(t, dump, "POST /things HTTP/1.1")
	assert.Contains(t, dump, `{"name":"a"}`, "request body")
	assert.Contains(t, dump, "HTTP/1.1 200 OK")
	assert.Contains(t, dump, `{"ok":true}`, "response body")
	assert.Contains(t, dump, "Authorization: [redacted]")
	assert.Contains(t, dump, "Set-Cookie: [redacted]")
	assert.NotContains(t, dump, "token-value")
	assert.NotContains(t, dump, "cookie-value")
}

// trackedBody records whether it was closed.
type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

// roundTripFunc adapts a function to an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestClient_closesBody(t *testing.T) {
	t.Parallel()

	respond := func(status int, body string) (*Client, *trackedBody) {
		tb := &trackedBody{Reader: strings.NewReader(body)}
		rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: tb, ContentLength: -1, Request: req}, nil
		})

		return newClient(t, WithBaseURL("https://api.example.com"), WithTransport(rt)), tb
	}

	t.Run("after decoding", func(t *testing.T) {
		t.Parallel()

		c, tb := respond(http.StatusOK, `{}`)

		_, err := c.Get[map[string]any](t.Context(), "/")
		require.NoError(t, err)
		assert.True(t, tb.closed)
	})

	t.Run("after a decoding failure", func(t *testing.T) {
		t.Parallel()

		c, tb := respond(http.StatusOK, `not json`)

		_, err := c.Get[map[string]any](t.Context(), "/")
		require.Error(t, err)
		assert.True(t, tb.closed)
	})

	t.Run("after a non-2xx status", func(t *testing.T) {
		t.Parallel()

		c, tb := respond(http.StatusInternalServerError, `oops`)

		_, err := c.Get[struct{}](t.Context(), "/")
		require.Error(t, err)
		assert.True(t, tb.closed)
	})

	t.Run("after a non-2xx status, even when the response was asked for", func(t *testing.T) {
		t.Parallel()

		c, tb := respond(http.StatusInternalServerError, `oops`)

		resp, err := c.Get[*http.Response](t.Context(), "/")
		_, ok := errors.AsType[*StatusError](err)
		assert.True(t, ok, "got %v", err)
		assert.Nil(t, resp)
		assert.True(t, tb.closed)
	})

	t.Run("not when the response is handed over", func(t *testing.T) {
		t.Parallel()

		c, tb := respond(http.StatusOK, `{}`)

		resp, err := c.Get[*http.Response](t.Context(), "/")
		require.NoError(t, err)
		assert.False(t, tb.closed)
		require.NoError(t, resp.Body.Close())
	})
}
