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
// Headers that carry credentials are shown as [redacted]. Bodies are read in
// full to be dumped, so a streamed body is buffered; this is for debugging,
// not for production traffic.
type Debug struct {
	// Next sends the request. It defaults to http.DefaultTransport.
	Next http.RoundTripper

	// Out receives the dumps. It defaults to os.Stderr.
	Out io.Writer

	// Serialises writes, so concurrent requests do not interleave.
	mu sync.Mutex
}

// RoundTrip implements http.RoundTripper.
func (t *Debug) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.Next
	if next == nil {
		next = http.DefaultTransport
	}

	// DumpRequestOut leaves the body in place, readable again.
	t.write(httputil.DumpRequestOut(req, true))

	resp, err := next.RoundTrip(req)
	if err != nil {
		t.write([]byte("error: "+err.Error()), nil)
		return nil, err
	}

	t.write(httputil.DumpResponse(resp, true))

	return resp, nil
}

func (t *Debug) write(dump []byte, err error) {
	out := t.Out
	if out == nil {
		out = os.Stderr
	}

	if err != nil {
		dump = []byte("dump failed: " + err.Error())
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_, _ = out.Write(redactHeaders(dump))
	_, _ = io.WriteString(out, "\n\n")
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
// of an HTTP dump, leaving the start line and the body byte for byte.
func redactHeaders(dump []byte) []byte {
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
