package client

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"iter"
	"mime"
	"strings"
)

// Event is one server-sent event, with its data decoded into a T.
type Event[T any] struct {
	// Type is the event's name, from its event field. An event without one
	// is a "message".
	Type string

	// ID is the last event ID the stream set. It carries over to later
	// events that do not set their own.
	ID string

	// Data is the event's data, decoded as a response body is: a []byte or
	// string holds it as sent, and anything else is decoded from JSON.
	Data T
}

// Stream sends body to rawURL with the given method, and reads the response
// as a stream of server-sent events, yielding each one as it arrives:
//
//	for ev, err := range c.Stream[Delta](ctx, http.MethodPost, "/chat", req) {
//		if err != nil {
//			return err
//		}
//
//		fmt.Print(ev.Data.Text)
//	}
//
// Each loop over the result sends the request, and the response body is
// closed when the loop ends, early or not. A failure is yielded as the error,
// and ends the loop: sending the request, a non-2xx status (a *StatusError), a
// response that is not text/event-stream, reading the stream, or decoding an
// event. A stream ends when the upstream closes it, or at the event that
// WithStreamEnd names.
//
// The client's timeout does not apply, since a stream lasts as long as the
// upstream keeps sending. Bound it with ctx; WithResponseHeaderTimeout bounds
// the wait for it to start.
func (c *Client) Stream[T, Req any](ctx context.Context, method, rawURL string, body Req, opts ...RequestOption) iter.Seq2[Event[T], error] {
	return func(yield func(Event[T], error) bool) {
		r := newRequestOptions(opts)

		resp, err := c.send(ctx, method, rawURL, body, r, mediaTypeEventStream, true)
		if err != nil {
			yield(Event[T]{}, err)
			return
		}

		defer func(b io.ReadCloser) { _ = b.Close() }(resp.Body)

		if ct := resp.Header.Get(headerContentType); ct != "" {
			if mt, _, _ := mime.ParseMediaType(ct); mt != mediaTypeEventStream {
				yield(Event[T]{}, fmt.Errorf("client: stream response is %s, not %s", ct, mediaTypeEventStream))
				return
			}
		}

		for raw, err := range readEvents(resp.Body) {
			if err != nil {
				yield(Event[T]{}, fmt.Errorf("client: read stream: %w", err))
				return
			}

			if r.streamEnd != "" && raw.data == r.streamEnd {
				return
			}

			ev := Event[T]{Type: raw.typ, ID: raw.id}

			if err := decodeData(raw.data, &ev.Data); err != nil {
				yield(Event[T]{}, fmt.Errorf("client: decode %s event: %w", raw.typ, err))
				return
			}

			if !yield(ev, nil) {
				return
			}
		}
	}
}

// Stream sends a request using a default Client, and reads its response as
// server-sent events. See Client.Stream.
func Stream[T, Req any](ctx context.Context, method, rawURL string, body Req, opts ...RequestOption) iter.Seq2[Event[T], error] {
	return defaultClient().Stream[T](ctx, method, rawURL, body, opts...)
}

// decodeData decodes an event's data into v, chosen by its type as decode
// chooses for a response body.
func decodeData[T any](data string, v *T) error {
	switch p := any(v).(type) {
	case *string:
		*p = data
	case *[]byte:
		*p = []byte(data)
	case *struct{}:
	default:
		return json.Unmarshal([]byte(data), v)
	}

	return nil
}

// rawEvent is a server-sent event as sent, before its data is decoded.
type rawEvent struct {
	typ, id, data string
}

// readEvents parses a stream of server-sent events from r, following the
// WHATWG HTML specification: events end at a blank line, data lines are
// joined with \n, lines starting with a colon are comments, and an event left
// unfinished at the end of the stream is dropped. The retry field, which
// tells a browser how long to wait before reconnecting, is ignored.
func readEvents(r io.Reader) iter.Seq2[rawEvent, error] {
	return func(yield func(rawEvent, error) bool) {
		sc := bufio.NewScanner(r)
		sc.Buffer(nil, maxEventLineSize)
		sc.Split(scanLines)

		var (
			typ, id string
			data    strings.Builder
			hasData bool
			first   = true
		)

		for sc.Scan() {
			line := sc.Text()
			if first {
				line, first = strings.TrimPrefix(line, "\uFEFF"), false
			}

			if line == "" {
				// An event without data is not dispatched; one with an empty
				// data line is.
				if hasData && !yield(rawEvent{typ: cmp.Or(typ, "message"), id: id, data: data.String()}, nil) {
					return
				}

				typ, hasData = "", false
				data.Reset()

				continue
			}

			field, value, _ := strings.Cut(line, ":")
			if field == "" {
				continue // a comment, often sent to keep the connection open
			}

			value = strings.TrimPrefix(value, " ")

			switch field {
			case "event":
				typ = value
			case "data":
				if hasData {
					data.WriteByte('\n')
				}

				data.WriteString(value)
				hasData = true
			case "id":
				if !strings.ContainsRune(value, 0) {
					id = value
				}
			}
		}

		if err := sc.Err(); err != nil {
			yield(rawEvent{}, err)
		}
	}
}

// scanLines is a bufio.SplitFunc for the line endings server-sent events
// allow: \n, \r\n, or a lone \r.
func scanLines(data []byte, atEOF bool) (int, []byte, error) {
	i := bytes.IndexAny(data, "\r\n")

	switch {
	case i >= 0 && data[i] == '\n':
		return i + 1, data[:i], nil
	case i >= 0 && i+1 < len(data):
		if data[i+1] == '\n' {
			return i + 2, data[:i], nil
		}

		return i + 1, data[:i], nil
	case i >= 0 && atEOF:
		return i + 1, data[:i], nil
	case i >= 0:
		// A \r at the end of what has arrived may be the start of a \r\n.
		return 0, nil, nil
	case atEOF && len(data) > 0:
		return len(data), data, nil
	default:
		return 0, nil, nil
	}
}
