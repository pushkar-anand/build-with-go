// Package client makes outbound HTTP calls, typed at the call site.
//
// Every call names the type it wants back, and the body is read into it:
//
//	post, err := c.Get[Post](ctx, "/posts/1")
//	created, err := c.Post[Post](ctx, "/posts", NewPost{Title: "Hello"})
//
// The request body is encoded according to its type, and the response body is
// decoded the same way: []byte and string are sent and read as they are,
// url.Values is sent as a form, an io.Reader is streamed, and anything else is
// JSON. Asking for struct{} discards the response body, and asking for
// *http.Response hands over the response unread, for streaming or for its
// headers; the caller must then close its body.
//
// A response outside the 2xx range comes back as a *StatusError, so a caller
// that checks err has handled every failure. When the upstream sends an
// RFC 9457 problem document, the error carries it decoded.
//
// Tracing, metrics, and retries belong in the transport: pass a wrapped
// http.RoundTripper to WithTransport, such as one from otelhttp.NewTransport.
// WithTokenSource and WithDebug wrap it in the bearer-token and debugging
// transports from http/transport.
package client

import (
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pushkar-anand/build-with-go/http/transport"
)

// Client sends HTTP requests. It is safe for concurrent use and is meant to
// be built once and shared, so connections are reused.
type Client struct {
	httpClient       *http.Client
	baseURL          *url.URL
	header           http.Header
	forwardRequestID bool
	maxErrBodySize   int64

	// Collected by options and applied to httpClient once they have all run,
	// so the result does not depend on their order.
	transport   http.RoundTripper
	timeout     *time.Duration
	tokenSource transport.TokenSource
	debugOut    io.Writer

	// The first option failure, reported by New.
	err error
}

// New returns a Client configured by Option. It fails only when an option is
// given an invalid value, such as a base URL that does not parse.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		header:         make(http.Header),
		maxErrBodySize: defaultMaxErrorBodySize,
	}

	for _, opt := range opts {
		opt.apply(c)
	}

	if c.err != nil {
		return nil, c.err
	}

	// Copy the caller's client rather than modifying it, since they may be
	// using it elsewhere. A client they supply keeps its own timeout, even if
	// that is none; only one built here gets the default.
	hc := &http.Client{Timeout: defaultTimeout}
	if c.httpClient != nil {
		hc = new(*c.httpClient)
	}

	if c.transport != nil {
		hc.Transport = c.transport
	}

	if c.timeout != nil {
		hc.Timeout = *c.timeout
	}

	// The debug transport goes beneath the token one, so its dump shows the
	// Authorization header that is actually sent (redacted).
	if c.debugOut != nil {
		hc.Transport = &transport.Debug{Next: hc.Transport, Out: c.debugOut}
	}

	if c.tokenSource != nil {
		hc.Transport = &transport.Bearer{Source: c.tokenSource, Next: hc.Transport}
	}

	c.httpClient = hc

	return c, nil
}

// defaultClient backs the package-level functions. It is built on first use.
var defaultClient = sync.OnceValue(func() *Client {
	c, err := New()
	if err != nil {
		panic(err) // New without options cannot fail
	}

	return c
})
