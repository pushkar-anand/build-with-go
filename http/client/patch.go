package client

import (
	"context"
	"net/http"
)

// Patch sends body in a PATCH request and reads the response body into a
// Resp. See Do.
func (c *Client) Patch[Resp, Req any](ctx context.Context, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	return c.Do[Resp](ctx, http.MethodPatch, rawURL, body, opts...)
}

// Patch sends a PATCH request using a default Client. See Client.Do.
func Patch[Resp, Req any](ctx context.Context, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	return defaultClient().Patch[Resp](ctx, rawURL, body, opts...)
}
