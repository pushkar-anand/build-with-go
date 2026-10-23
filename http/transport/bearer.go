package transport

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pushkar-anand/build-with-go/secret"
)

type (
	// TokenSource supplies the token sent with each request. It is called for
	// every request, so a source whose tokens expire should cache one and
	// refresh it only when needed. It must be safe for concurrent use.
	TokenSource interface {
		Token(ctx context.Context) (secret.Secret[string], error)
	}

	// TokenSourceFunc adapts a function to a TokenSource.
	TokenSourceFunc func(ctx context.Context) (secret.Secret[string], error)

	staticToken secret.Secret[string]
)

// Token calls fn.
func (fn TokenSourceFunc) Token(ctx context.Context) (secret.Secret[string], error) {
	return fn(ctx)
}

// StaticToken returns a TokenSource that always supplies token, for a key
// that does not expire.
func StaticToken(token secret.Secret[string]) TokenSource {
	return staticToken(token)
}

func (t staticToken) Token(context.Context) (secret.Secret[string], error) {
	return secret.Secret[string](t), nil
}

// Bearer sends a bearer token from Source in the Authorization header
// of every request. A request that already has an Authorization header, such
// as one given WithBearerToken, is sent as it is.
type Bearer struct {
	// Source supplies the token. It is required.
	Source TokenSource

	// Next sends the request. It defaults to http.DefaultTransport.
	Next http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *Bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.Next
	if next == nil {
		next = http.DefaultTransport
	}

	if req.Header.Get(headerAuthorization) != "" {
		return next.RoundTrip(req)
	}

	token, err := t.Source.Token(req.Context())
	if err != nil {
		closeBody(req)
		return nil, fmt.Errorf("transport: get token: %w", err)
	}

	// A RoundTripper must not modify the request it is given, so the header
	// goes on a copy. The copy shares the body, which is read only once.
	req = req.Clone(req.Context())
	req.Header.Set(headerAuthorization, bearerPrefix+token.Reveal())

	return next.RoundTrip(req)
}

// closeBody closes a request body that will not be sent, as a RoundTripper
// must even when it fails.
func closeBody(req *http.Request) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
}

var _ http.RoundTripper = (*Bearer)(nil)
