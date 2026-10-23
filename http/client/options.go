package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pushkar-anand/build-with-go/http/transport"
)

type (
	// Option configures a Client.
	Option interface {
		apply(*Client)
	}

	optionFunc func(*Client)
)

func (fn optionFunc) apply(c *Client) {
	fn(c)
}

// WithBaseURL sets the URL that relative request URLs are appended to. Its
// path is kept, so a base of https://api.example.com/v1 and a request for
// /posts reach https://api.example.com/v1/posts.
func WithBaseURL(rawURL string) Option {
	return optionFunc(func(c *Client) {
		u, err := url.Parse(rawURL)
		if err != nil {
			c.err = fmt.Errorf("client: base URL: %w", err)
			return
		}

		if !u.IsAbs() {
			c.err = fmt.Errorf("client: base URL %q is not absolute", rawURL)
			return
		}

		c.baseURL = u
	})
}

// WithHTTPClient sends requests through a copy of hc, so its transport,
// redirect policy and cookie jar apply. It is copied, never modified, and its
// timeout is kept as it is, even when that is none.
func WithHTTPClient(hc *http.Client) Option {
	return optionFunc(func(c *Client) {
		c.httpClient = hc
	})
}

// WithTransport sets the RoundTripper requests are sent through. Wrap one here
// to add tracing, metrics or retries.
func WithTransport(rt http.RoundTripper) Option {
	return optionFunc(func(c *Client) {
		c.transport = rt
	})
}

// WithTimeout caps the time a request may take, including reading the
// response body. It defaults to 30 seconds; pass 0 to disable it.
func WithTimeout(d time.Duration) Option {
	return optionFunc(func(c *Client) {
		c.timeout = new(d)
	})
}

// WithDefaultHeader adds a header to every request the client sends. A request
// that sets the same header with WithHeader replaces it.
func WithDefaultHeader(key, value string) Option {
	return optionFunc(func(c *Client) {
		c.header.Add(key, value)
	})
}

// WithUserAgent sets the User-Agent header on every request.
func WithUserAgent(ua string) Option {
	return optionFunc(func(c *Client) {
		c.header.Set(headerUserAgent, ua)
	})
}

// WithForwardRequestID sends the request ID that http/middleware placed in the
// context as X-Request-Id, so a call made while serving a request can be
// traced across both services. A request that sets the header itself keeps
// its own value.
//
// It is off by default: an ID meant for your own services is not something to
// hand to a third party.
func WithForwardRequestID() Option {
	return optionFunc(func(c *Client) {
		c.forwardRequestID = true
	})
}

// WithTokenSource sends a bearer token from src with every request, through a
// transport.Bearer wrapped around the client's transport. A request given
// WithBearerToken sends that token instead.
func WithTokenSource(src transport.TokenSource) Option {
	return optionFunc(func(c *Client) {
		c.tokenSource = src
	})
}

// WithDebug dumps every request and response to w, through a transport.Debug
// wrapped around the client's transport. Pass os.Stderr to see them in the
// terminal.
func WithDebug(w io.Writer) Option {
	return optionFunc(func(c *Client) {
		c.debugOut = w
	})
}

// WithMaxErrorBodySize sets how much of a non-2xx response body is kept on the
// StatusError. It defaults to 64 KiB.
func WithMaxErrorBodySize(n int64) Option {
	return optionFunc(func(c *Client) {
		c.maxErrBodySize = n
	})
}
