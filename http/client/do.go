package client

import (
	"context"
	"io"
	"net/http"
)

// Do sends body to rawURL with the given method, and reads the response body
// into a Resp. A relative rawURL is appended to the base URL. Pass
// http.NoBody to send no body.
//
// See the package documentation for how body and Resp are encoded and
// decoded. A non-2xx response returns a *StatusError.
func (c *Client) Do[Resp, Req any](ctx context.Context, method, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	var zero Resp

	req, err := c.newRequest(ctx, method, rawURL, body, opts)
	if err != nil {
		return zero, err
	}

	if wantsJSON[Resp]() && req.Header.Get(headerAccept) == "" {
		req.Header.Set(headerAccept, mediaTypeJSON)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, err
	}

	// A caller asking for the response itself is handed it open, and closes
	// the body. Every other path closes it here.
	if p, ok := any(&zero).(**http.Response); ok && isSuccess(resp.StatusCode) {
		*p = resp
		return zero, nil
	}

	defer func(b io.ReadCloser) { _ = b.Close() }(resp.Body)

	if !isSuccess(resp.StatusCode) {
		return zero, newStatusError(req, resp, c.maxErrBodySize)
	}

	return decode[Resp](resp)
}

// isSuccess reports whether status is in the 2xx range.
func isSuccess(status int) bool {
	return status >= 200 && status <= 299
}

// Do sends a request using a default Client, which has no base URL and a
// 30-second timeout. See Client.Do.
func Do[Resp, Req any](ctx context.Context, method, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	return defaultClient().Do[Resp](ctx, method, rawURL, body, opts...)
}
