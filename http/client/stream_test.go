package client

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []rawEvent
	}{
		{
			name:  "one event",
			input: "event: delta\ndata: hi\n\n",
			want:  []rawEvent{{typ: "delta", data: "hi"}},
		},
		{
			name:  "an event without a type is a message",
			input: "data: hi\n\n",
			want:  []rawEvent{{typ: "message", data: "hi"}},
		},
		{
			name:  "data lines are joined",
			input: "data: a\ndata: b\n\n",
			want:  []rawEvent{{typ: "message", data: "a\nb"}},
		},
		{
			name:  "only one space is stripped, and none need be there",
			input: "data:a\n\ndata:  b\n\n",
			want:  []rawEvent{{typ: "message", data: "a"}, {typ: "message", data: " b"}},
		},
		{
			name:  "\\r\\n and \\r end lines",
			input: "data: a\r\n\r\ndata: b\r\rdata: c\n\n",
			want:  []rawEvent{{typ: "message", data: "a"}, {typ: "message", data: "b"}, {typ: "message", data: "c"}},
		},
		{
			name:  "comments and unknown fields are skipped",
			input: ": keep-alive\nretry: 1000\nfoo: bar\ndata: hi\n\n",
			want:  []rawEvent{{typ: "message", data: "hi"}},
		},
		{
			name:  "an event without data is not dispatched, but one with empty data is",
			input: "event: ping\n\ndata:\n\n",
			want:  []rawEvent{{typ: "message", data: ""}},
		},
		{
			name:  "the type resets after each event, the id carries over",
			input: "event: a\nid: 1\ndata: x\n\ndata: y\n\nid: 2\ndata: z\n\n",
			want:  []rawEvent{{typ: "a", id: "1", data: "x"}, {typ: "message", id: "1", data: "y"}, {typ: "message", id: "2", data: "z"}},
		},
		{
			name:  "an id holding NUL is ignored",
			input: "id: 1\ndata: x\n\nid: a\x00b\ndata: y\n\n",
			want:  []rawEvent{{typ: "message", id: "1", data: "x"}, {typ: "message", id: "1", data: "y"}},
		},
		{
			name:  "a leading byte-order mark is dropped",
			input: "\uFEFFdata: hi\n\n",
			want:  []rawEvent{{typ: "message", data: "hi"}},
		},
		{
			name:  "an event unfinished at the end is dropped",
			input: "data: a\n\ndata: b\n",
			want:  []rawEvent{{typ: "message", data: "a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []rawEvent

			for ev, err := range readEvents(strings.NewReader(tt.input)) {
				require.NoError(t, err)

				got = append(got, ev)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadEvents_lineTooLong(t *testing.T) {
	t.Parallel()

	input := "data: " + strings.Repeat("x", maxEventLineSize) + "\n\n"

	var errs []error

	for _, err := range readEvents(strings.NewReader(input)) {
		errs = append(errs, err)
	}

	require.Len(t, errs, 1)
	assert.ErrorIs(t, errs[0], bufio.ErrTooLong)
}

// newEventServer answers every request with the events send writes, flushing
// after each one.
func newEventServer(t *testing.T, send func(event func(string))) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		send(func(ev string) {
			_, _ = io.WriteString(w, ev)
			w.(http.Flusher).Flush()
		})
	}))
	t.Cleanup(srv.Close)

	return srv
}

type delta struct {
	Text string `json:"text"`
}

func TestClient_Stream(t *testing.T) {
	t.Parallel()

	t.Run("yields each event, decoded, until [DONE]", func(t *testing.T) {
		t.Parallel()

		var gotAccept, gotBody string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAccept = r.Header.Get("Accept")
			data, _ := io.ReadAll(r.Body)
			gotBody = string(data)

			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			_, _ = io.WriteString(w, "event: delta\nid: 1\ndata: {\"text\":\"Hel\"}\n\n")
			_, _ = io.WriteString(w, "event: delta\ndata: {\"text\":\"lo\"}\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\ndata: {\"text\":\"after\"}\n\n")
		}))
		t.Cleanup(srv.Close)

		c := newClient(t, WithBaseURL(srv.URL))

		var got []Event[delta]

		for ev, err := range c.Stream[delta](t.Context(), http.MethodPost, "/chat", map[string]string{"prompt": "hi"}) {
			require.NoError(t, err)

			got = append(got, ev)
		}

		assert.Equal(t, "text/event-stream", gotAccept)
		assert.JSONEq(t, `{"prompt":"hi"}`, gotBody)
		assert.Equal(t, []Event[delta]{
			{Type: "delta", ID: "1", Data: delta{Text: "Hel"}},
			{Type: "delta", ID: "1", Data: delta{Text: "lo"}},
		}, got)
	})

	t.Run("keeps data as sent when asked for a string", func(t *testing.T) {
		t.Parallel()

		srv := newEventServer(t, func(event func(string)) {
			event("data: hi\n\n")
			event("data: [DONE]\n\n")
		})

		var got []string

		for ev, err := range Stream[string](t.Context(), http.MethodGet, srv.URL, http.NoBody) {
			require.NoError(t, err)

			got = append(got, ev.Data)
		}

		assert.Equal(t, []string{"hi", "[DONE]"}, got)
	})

	// collect returns the error a stream ends with.
	collect := func(t *testing.T, c *Client) error {
		t.Helper()

		for _, err := range c.Stream[delta](t.Context(), http.MethodGet, "/", http.NoBody) {
			if err != nil {
				return err
			}
		}

		return nil
	}

	t.Run("a non-2xx status is a StatusError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "slow down", http.StatusTooManyRequests)
		}))
		t.Cleanup(srv.Close)

		err := collect(t, newClient(t, WithBaseURL(srv.URL)))

		se, ok := errors.AsType[*StatusError](err)
		require.True(t, ok, "got %v", err)
		assert.Equal(t, http.StatusTooManyRequests, se.StatusCode)
	})

	t.Run("a response that is not an event stream is an error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{}`)
		}))
		t.Cleanup(srv.Close)

		err := collect(t, newClient(t, WithBaseURL(srv.URL)))
		assert.ErrorContains(t, err, "not text/event-stream")
	})

	t.Run("an event that does not decode is an error", func(t *testing.T) {
		t.Parallel()

		srv := newEventServer(t, func(event func(string)) {
			event("event: delta\ndata: not json\n\n")
		})

		err := collect(t, newClient(t, WithBaseURL(srv.URL)))
		assert.ErrorContains(t, err, "decode delta event")
	})

	t.Run("breaking out of the loop closes the body", func(t *testing.T) {
		t.Parallel()

		tb := &trackedBody{Reader: strings.NewReader("data: {}\n\ndata: {}\n\n")}
		rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			header := http.Header{"Content-Type": {"text/event-stream"}}
			return &http.Response{StatusCode: http.StatusOK, Header: header, Body: tb, ContentLength: -1, Request: req}, nil
		})

		c := newClient(t, WithBaseURL("https://api.example.com"), WithTransport(rt))

		for _, err := range c.Stream[delta](t.Context(), http.MethodGet, "/", http.NoBody) {
			require.NoError(t, err)
			assert.False(t, tb.closed, "closed mid-stream")

			break
		}

		assert.True(t, tb.closed)
	})
}

// slowEvents answers every request with events sent gap apart, as an
// upstream streaming its output does. Like a real transport, it ends the body
// once the request's context is done.
func slowEvents(gap time.Duration, events ...string) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		pr, pw := io.Pipe()
		stop := context.AfterFunc(req.Context(), func() { pw.CloseWithError(context.Cause(req.Context())) })

		go func() {
			defer stop()

			for i, ev := range events {
				if i > 0 {
					select {
					case <-time.After(gap):
					case <-req.Context().Done():
						return
					}
				}

				if _, err := io.WriteString(pw, ev); err != nil {
					return
				}
			}

			_ = pw.Close()
		}()

		header := http.Header{"Content-Type": {"text/event-stream"}}

		return &http.Response{StatusCode: http.StatusOK, Header: header, Body: pr, ContentLength: -1, Request: req}, nil
	})
}

// The tests run in a synctest bubble, so their minutes pass at once.
func TestClient_streamTimeouts(t *testing.T) {
	t.Parallel()

	// Two events, further apart than the client's timeout.
	slow := slowEvents(time.Minute, "data: {\"text\":\"a\"}\n\n", "data: {\"text\":\"b\"}\n\n")

	t.Run("a stream outlives the client's timeout", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			c := newClient(t, WithBaseURL("https://api.example.com"), WithTransport(slow), WithTimeout(time.Second))

			var got []string

			for ev, err := range c.Stream[delta](t.Context(), http.MethodGet, "/", http.NoBody) {
				require.NoError(t, err)

				got = append(got, ev.Data.Text)
			}

			assert.Equal(t, []string{"a", "b"}, got)
		})
	})

	t.Run("so does a response handed over, but not one the client reads", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			c := newClient(t, WithBaseURL("https://api.example.com"), WithTransport(slow), WithTimeout(time.Second))

			resp, err := c.Get[*http.Response](t.Context(), "/")
			require.NoError(t, err)

			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Contains(t, string(data), `"b"`)

			_, err = c.Get[string](t.Context(), "/")
			assert.ErrorIs(t, err, context.DeadlineExceeded)
		})
	})

	t.Run("the wait for headers is bounded", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			// An upstream that never answers.
			rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, context.Cause(req.Context())
			})

			c := newClient(t, WithBaseURL("https://api.example.com"), WithTransport(rt), WithResponseHeaderTimeout(time.Minute))

			start := time.Now()

			var err error
			for _, err = range c.Stream[delta](t.Context(), http.MethodGet, "/", http.NoBody) {
			}

			assert.ErrorIs(t, err, ErrResponseHeaderTimeout)
			assert.Equal(t, time.Minute, time.Since(start))
		})
	})
}
