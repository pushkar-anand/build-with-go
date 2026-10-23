package client

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"

	"github.com/pushkar-anand/build-with-go/ctxval"
)

// newRequest builds the http.Request described by the client's defaults and
// opts, applied in that order so a request option overrides a default.
func (c *Client) newRequest[Req any](ctx context.Context, method, rawURL string, body Req, opts []RequestOption) (*http.Request, error) {
	r := &request{
		header: make(http.Header),
		query:  make(url.Values),
	}

	for _, opt := range opts {
		opt.apply(r)
	}

	u, err := c.resolve(rawURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	for key, values := range r.query {
		q[key] = append(q[key], values...)
	}

	u.RawQuery = q.Encode()

	rd, contentType, err := encode(body)
	if err != nil {
		return nil, fmt.Errorf("client: encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}

	// Cloned, since the client's defaults are shared by concurrent requests.
	for key, values := range c.header {
		req.Header[key] = slices.Clone(values)
	}

	if contentType != "" {
		req.Header.Set(headerContentType, contentType)
	}

	maps.Copy(req.Header, r.header)

	if c.forwardRequestID && req.Header.Get(headerRequestID) == "" {
		if id, ok := ctxval.RequestIDFromContext(ctx); ok {
			req.Header.Set(headerRequestID, id)
		}
	}

	for _, edit := range r.edits {
		if err := edit(req); err != nil {
			return nil, err
		}
	}

	return req, nil
}

// resolve turns rawURL into an absolute URL. A relative one is appended to the
// base URL's path rather than replacing it, so a base of
// https://api.example.com/v1 and a path of /posts reach /v1/posts.
func (c *Client) resolve(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}

	if u.IsAbs() {
		return u, nil
	}

	if c.baseURL == nil {
		return nil, fmt.Errorf("client: relative URL %q needs a base URL", rawURL)
	}

	resolved := c.baseURL.JoinPath(u.EscapedPath())

	// The base URL's own query, if any, is kept alongside the request's.
	q := resolved.Query()
	for key, values := range u.Query() {
		q[key] = append(q[key], values...)
	}

	resolved.RawQuery = q.Encode()

	return resolved, nil
}
