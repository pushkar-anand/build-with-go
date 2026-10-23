package transport

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"os"
	"strings"
	"sync"

	"github.com/pushkar-anand/build-with-go/secret"
)

// Debug dumps every request it sends and every response it receives,
// headers and bodies, for seeing exactly what goes over the wire.
//
// Headers that carry credentials are shown as [redacted]. Headers are dumped
// as they are sent and received, and bodies as they are read, so a streamed
// response shows up as it arrives. The bodies of concurrent requests can
// interleave in the dump. This is for debugging, not for production traffic.
type Debug struct {
	// Next sends the request. It defaults to http.DefaultTransport.
	Next http.RoundTripper

	// Out receives the dumps. It defaults to os.Stderr.
	Out io.Writer

	// Serialises writes, so concurrent requests do not interleave mid-write.
	mu sync.Mutex
}

// RoundTrip implements http.RoundTripper.
func (t *Debug) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.Next
	if next == nil {
		next = http.DefaultTransport
	}

	t.write(redactHeaders(httputil.DumpRequestOut(req, false)))

	if req.Body == nil || req.Body == http.NoBody {
		t.write([]byte(dumpEnd))
	} else {
		// The request is the caller's; its body is swapped on a copy.
		req = req.Clone(req.Context())
		req.Body = &dumpBody{ReadCloser: req.Body, t: t}
	}

	resp, err := next.RoundTrip(req)
	if err != nil {
		t.write([]byte("error: " + err.Error() + dumpEnd))
		return nil, err
	}

	t.write(redactHeaders(httputil.DumpResponse(resp, false)))

	// A 101 response's body is the upgraded connection, written to as well as
	// read; it is left as it is.
	if _, ok := resp.Body.(io.Writer); ok {
		t.write([]byte(dumpEnd))
	} else {
		resp.Body = &dumpBody{ReadCloser: resp.Body, t: t}
	}

	return resp, nil
}

// dumpEnd follows each request and response in a dump.
const dumpEnd = "\n\n"

func (t *Debug) write(dump []byte) {
	out := t.Out
	if out == nil {
		out = os.Stderr
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_, _ = out.Write(dump)
}

// dumpBody copies a body to the dump as it is read, and ends the dump once the
// body has been read to the end or closed.
type dumpBody struct {
	io.ReadCloser
	t    *Debug
	once sync.Once
}

func (b *dumpBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.t.write(p[:n])
	}

	if err != nil {
		b.end()
	}

	return n, err
}

func (b *dumpBody) Close() error {
	b.end()
	return b.ReadCloser.Close()
}

// end is called by both Read and Close, which the transport may call from
// different goroutines.
func (b *dumpBody) end() {
	b.once.Do(func() { b.t.write([]byte(dumpEnd)) })
}

// sensitiveHeaders are shown as [redacted] in a dump.
var sensitiveHeaders = map[string]bool{
	headerAuthorization:      true,
	headerProxyAuthorization: true,
	headerCookie:             true,
	headerSetCookie:          true,
	headerAPIKey:             true,
}

// redactHeaders replaces the values of sensitive headers in the header block
// of an HTTP dump, leaving the start line and the body byte for byte. When the
// dump failed, it describes err instead.
func redactHeaders(dump []byte, err error) []byte {
	if err != nil {
		return []byte("dump failed: " + err.Error() + dumpEnd)
	}

	head, body, hasBody := bytes.Cut(dump, []byte("\r\n\r\n"))

	lines := strings.Split(string(head), "\r\n")
	for i, line := range lines[1:] {
		if name, _, ok := strings.Cut(line, ":"); ok && sensitiveHeaders[textproto.CanonicalMIMEHeaderKey(name)] {
			lines[i+1] = name + ": " + secret.Redacted
		}
	}

	out := []byte(strings.Join(lines, "\r\n"))
	if hasBody {
		out = append(out, "\r\n\r\n"...)
		out = append(out, body...)
	}

	return out
}

var _ http.RoundTripper = (*Debug)(nil)
