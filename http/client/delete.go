package client

import (
	"context"
	"net/http"
)

// Delete sends a DELETE request and reads the response body into a Resp. See
// Do.
func (c *Client) Delete[Resp any](ctx context.Context, rawURL string, opts ...RequestOption) (Resp, error) {
	return c.Do[Resp](ctx, http.MethodDelete, rawURL, http.NoBody, opts...)
}

// Delete sends a DELETE request using a default Client. See Client.Do.
func Delete[Resp any](ctx context.Context, rawURL string, opts ...RequestOption) (Resp, error) {
	return defaultClient().Delete[Resp](ctx, rawURL, opts...)
}
