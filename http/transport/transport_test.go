package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pushkar-anand/build-with-go/secret"
)

// roundTripFunc adapts a function to an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// recordAuth answers every request with a 204, recording the Authorization
// header it arrived with.
func recordAuth(got *string) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		*got = req.Header.Get("Authorization")
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: req}, nil
	})
}

func TestBearer(t *testing.T) {
	t.Parallel()

	t.Run("adds the token, leaving the caller's request alone", func(t *testing.T) {
		t.Parallel()

		var got string
		rt := &Bearer{Source: StaticToken(secret.New("tok")), Next: recordAuth(&got)}

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/", nil)

		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		_ = resp.Body.Close()

		assert.Equal(t, "Bearer tok", got)
		assert.Empty(t, req.Header.Get("Authorization"), "the caller's request was modified")
	})

	t.Run("keeps an Authorization header already set", func(t *testing.T) {
		t.Parallel()

		var got string
		rt := &Bearer{Source: StaticToken(secret.New("tok")), Next: recordAuth(&got)}

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/", nil)
		req.Header.Set("Authorization", "Basic abc")

		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		_ = resp.Body.Close()

		assert.Equal(t, "Basic abc", got)
	})

	t.Run("asks the source for every request", func(t *testing.T) {
		t.Parallel()

		calls := 0
		src := TokenSourceFunc(func(context.Context) (secret.Secret[string], error) {
			calls++
			return secret.New("tok"), nil
		})

		var got string
		rt := &Bearer{Source: src, Next: recordAuth(&got)}

		for range 2 {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/", nil)

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			_ = resp.Body.Close()
		}

		assert.Equal(t, 2, calls)
	})

	t.Run("fails without sending when the source fails", func(t *testing.T) {
		t.Parallel()

		boom := errors.New("boom")
		src := TokenSourceFunc(func(context.Context) (secret.Secret[string], error) {
			return secret.Secret[string]{}, boom
		})

		sent := false
		next := roundTripFunc(func(*http.Request) (*http.Response, error) {
			sent = true
			return nil, nil
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://api.example.com/", strings.NewReader("x"))

		_, err := (&Bearer{Source: src, Next: next}).RoundTrip(req)
		assert.ErrorIs(t, err, boom)
		assert.False(t, sent)
	})
}

func TestDebug(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		http.SetCookie(w, &http.Cookie{Name: "session", Value: "cookie-value"})
		_, _ = w.Write(append([]byte("echo: "), body...))
	}))
	t.Cleanup(srv.Close)

	var out bytes.Buffer
	hc := &http.Client{Transport: &Debug{Out: &out}}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/things", strings.NewReader("ping"))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer token-value")

	resp, err := hc.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, "echo: ping", string(body), "the dump consumed a body")

	dump := out.String()

	assert.Contains(t, dump, "POST /things HTTP/1.1")
	assert.Contains(t, dump, "\r\n\r\nping")
	assert.Contains(t, dump, "HTTP/1.1 200 OK")
	assert.Contains(t, dump, "echo: ping")
	assert.Contains(t, dump, "Authorization: [redacted]")
	assert.Contains(t, dump, "Set-Cookie: [redacted]")
	assert.NotContains(t, dump, "token-value")
	assert.NotContains(t, dump, "cookie-value")
}

// It runs in a synctest bubble: were the body buffered for the dump, the
// round trip would wait on the upstream, which waits on the release below,
// and synctest would report the deadlock.
func TestDebug_streams(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		pr, pw := io.Pipe()

		go func() {
			_, _ = io.WriteString(pw, "first")

			<-release

			_, _ = io.WriteString(pw, "second")
			_ = pw.Close()
		}()

		var out bytes.Buffer
		rt := &Debug{
			Out: &out,
			Next: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, ProtoMajor: 1, ProtoMinor: 1, Body: pr, ContentLength: -1, Request: req}, nil
			}),
		}

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/", nil)

		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)

		buf := make([]byte, len("first"))
		_, err = io.ReadFull(resp.Body, buf)
		require.NoError(t, err)

		assert.Equal(t, "first", string(buf))
		assert.True(t, strings.HasSuffix(out.String(), "\r\n\r\nfirst"), "the dump lags the body: %q", out.String())

		close(release)

		_, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		assert.True(t, strings.HasSuffix(out.String(), "firstsecond\n\n"), "got %q", out.String())
	})
}

func TestDebug_endsDumpOnClose(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	rt := &Debug{
		Out: &out,
		Next: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				ProtoMajor: 1,
				ProtoMinor: 1,
				Body:       io.NopCloser(strings.NewReader("unread")),
				Request:    req,
			}, nil
		}),
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/", nil)

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, resp.Body.Close())

	assert.True(t, strings.HasSuffix(out.String(), "\r\n\r\n\n\n"), "got %q", out.String())
	assert.NotContains(t, out.String(), "unread")
}

func TestDebug_error(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")

	var out bytes.Buffer
	rt := &Debug{
		Out:  &out,
		Next: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, boom }),
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/", nil)

	_, err := rt.RoundTrip(req)
	assert.ErrorIs(t, err, boom)
	assert.Contains(t, out.String(), "error: boom")
}

func TestRedactHeaders(t *testing.T) {
	t.Parallel()

	dump := "GET / HTTP/1.1\r\nHost: example.com\r\nX-Api-Key: k\r\n\r\nAuthorization: in the body"

	assert.Equal(t,
		"GET / HTTP/1.1\r\nHost: example.com\r\nX-Api-Key: [redacted]\r\n\r\nAuthorization: in the body",
		string(redactHeaders([]byte(dump), nil)),
	)
}
