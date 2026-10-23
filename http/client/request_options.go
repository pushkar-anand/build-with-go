package client

import (
	"net/http"
	"net/url"

	"github.com/pushkar-anand/build-with-go/secret"
)

type (
	// RequestOption configures a single request.
	RequestOption interface {
		apply(*request)
	}

	requestOptionFunc func(*request)

	// request collects what the options of one call ask for.
	request struct {
		header    http.Header
		query     url.Values
		edits     []func(*http.Request) error
		streamEnd string
	}
)

// newRequestOptions collects what opts ask for.
func newRequestOptions(opts []RequestOption) *request {
	r := &request{
		header: make(http.Header),
		query:  make(url.Values),
	}

	for _, opt := range opts {
		opt.apply(r)
	}

	return r
}

func (fn requestOptionFunc) apply(r *request) {
	fn(r)
}

// WithHeader adds a header to the request, replacing a client default of the
// same name. Given several times for one key, it sends every value.
func WithHeader(key, value string) RequestOption {
	return requestOptionFunc(func(r *request) {
		r.header.Add(key, value)
	})
}

// WithQuery adds a query parameter, alongside any already in the URL.
func WithQuery(key, value string) RequestOption {
	return requestOptionFunc(func(r *request) {
		r.query.Add(key, value)
	})
}

// WithQueryValues adds every parameter in v, alongside any already in the URL.
func WithQueryValues(v url.Values) RequestOption {
	return requestOptionFunc(func(r *request) {
		for key, values := range v {
			r.query[key] = append(r.query[key], values...)
		}
	})
}

// WithBearerToken sends token in an Authorization header. It takes a Secret so
// the token cannot reach a log through the code that passes it along.
//
// For a token every request needs, use the client's WithTokenSource instead.
func WithBearerToken(token secret.Secret[string]) RequestOption {
	return requestOptionFunc(func(r *request) {
		r.header.Set(headerAuthorization, bearerPrefix+token.Reveal())
	})
}

// WithBasicAuth sends HTTP Basic credentials in an Authorization header.
func WithBasicAuth(username string, password secret.Secret[string]) RequestOption {
	return WithRequestFunc(func(req *http.Request) error {
		req.SetBasicAuth(username, password.Reveal())
		return nil
	})
}

// WithRequestFunc calls fn with the built request just before it is sent, for
// anything the other options do not cover, such as signing it. An error from
// fn is returned from the call and the request is not sent.
func WithRequestFunc(fn func(*http.Request) error) RequestOption {
	return requestOptionFunc(func(r *request) {
		r.edits = append(r.edits, fn)
	})
}

// WithStreamEnd ends a Stream at the first event whose data is data, without
// yielding that event. Use it for an API that marks the end of its stream
// with a sentinel rather than by closing it, as OpenAI-style APIs send
// "[DONE]", which would otherwise fail to decode. Calls other than Stream
// ignore it.
func WithStreamEnd(data string) RequestOption {
	return requestOptionFunc(func(r *request) {
		r.streamEnd = data
	})
}
