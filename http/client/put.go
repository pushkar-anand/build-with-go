package client

import (
	"context"
	"net/http"
)

// Put sends body in a PUT request and reads the response body into a Resp.
// See Do.
func (c *Client) Put[Resp, Req any](ctx context.Context, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	return c.Do[Resp](ctx, http.MethodPut, rawURL, body, opts...)
}

// Put sends a PUT request using a default Client. See Client.Do.
func Put[Resp, Req any](ctx context.Context, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	return defaultClient().Put[Resp](ctx, rawURL, body, opts...)
}
