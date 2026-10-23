package client

import (
	"context"
	"net/http"
)

// Post sends body in a POST request and reads the response body into a Resp.
// See Do.
func (c *Client) Post[Resp, Req any](ctx context.Context, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	return c.Do[Resp](ctx, http.MethodPost, rawURL, body, opts...)
}

// Post sends a POST request using a default Client. See Client.Do.
func Post[Resp, Req any](ctx context.Context, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	return defaultClient().Post[Resp](ctx, rawURL, body, opts...)
}
