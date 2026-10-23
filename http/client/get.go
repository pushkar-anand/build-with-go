package client

import (
	"context"
	"net/http"
)

// Get sends a GET request and reads the response body into a Resp. See Do.
func (c *Client) Get[Resp any](ctx context.Context, rawURL string, opts ...RequestOption) (Resp, error) {
	return c.Do[Resp](ctx, http.MethodGet, rawURL, http.NoBody, opts...)
}

// Get sends a GET request using a default Client. See Client.Do.
func Get[Resp any](ctx context.Context, rawURL string, opts ...RequestOption) (Resp, error) {
	return defaultClient().Get[Resp](ctx, rawURL, opts...)
}
