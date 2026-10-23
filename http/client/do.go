package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Do sends body to rawURL with the given method, and reads the response body
// into a Resp. A relative rawURL is appended to the base URL. Pass
// http.NoBody to send no body.
//
// See the package documentation for how body and Resp are encoded and
// decoded. A non-2xx response returns a *StatusError.
func (c *Client) Do[Resp, Req any](ctx context.Context, method, rawURL string, body Req, opts ...RequestOption) (Resp, error) {
	var zero Resp

	var accept string
	if wantsJSON[Resp]() {
		accept = mediaTypeJSON
	}

	// A caller asking for the response itself is handed it open, reads the
	// body at its own pace, and closes it. Every other path closes it here.
	p, handOver := any(&zero).(**http.Response)

	resp, err := c.send(ctx, method, rawURL, body, newRequestOptions(opts), accept, handOver)
	if err != nil {
		return zero, err
	}

	if handOver {
		*p = resp
		return zero, nil
	}

	defer func(b io.ReadCloser) { _ = b.Close() }(resp.Body)

	return decode[Resp](resp)
}

// send sends a request and returns its response when the status is 2xx. Any
// other status is returned as a *StatusError, with the body closed.
//
// A streamed request is one whose body the caller reads at its own pace. It is
// sent without the client's timeout, which would cut a long body off, and is
// bounded instead by ctx, and by the response header timeout until the
// response arrives.
func (c *Client) send[Req any](ctx context.Context, method, rawURL string, body Req, r *request, accept string, streamed bool) (*http.Response, error) {
	req, err := c.newRequest(ctx, method, rawURL, body, r)
	if err != nil {
		return nil, err
	}

	if accept != "" && req.Header.Get(headerAccept) == "" {
		req.Header.Set(headerAccept, accept)
	}

	var resp *http.Response
	if streamed {
		resp, err = c.doStreamed(req)
	} else {
		resp, err = c.httpClient.Do(req)
	}

	if err != nil {
		return nil, err
	}

	if !isSuccess(resp.StatusCode) {
		defer func(b io.ReadCloser) { _ = b.Close() }(resp.Body)
		return nil, newStatusError(req, resp, c.maxErrBodySize)
	}

	return resp, nil
}

// doStreamed sends req through streamClient, which has no overall timeout,
// cancelling it if the response headers take longer than headerTimeout.
func (c *Client) doStreamed(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(req.Context())

	stop := func() bool { return true }
	if c.headerTimeout > 0 {
		stop = time.AfterFunc(c.headerTimeout, func() { cancel(ErrResponseHeaderTimeout) }).Stop
	}

	resp, err := c.streamClient.Do(req.WithContext(ctx))

	// The timer cannot be stopped once it has fired, and by then the request
	// is cancelled: even a response that just made it can no longer be read.
	if !stop() {
		if err == nil {
			_ = resp.Body.Close()
		}

		err = fmt.Errorf("%s %s: %w", req.Method, displayURL(req.URL), ErrResponseHeaderTimeout)
	}

	if err != nil {
		cancel(nil)
		return nil, err
	}

	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}

	return resp, nil
}

// cancelOnClose releases a streamed request's context once its body is
// closed.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelCauseFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel(nil)

	return err
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
