// Package transport holds http.RoundTripper wrappers, composable with any
// other: each takes the RoundTripper it wraps as Next, and defaults it to
// http.DefaultTransport.
//
// Set one as an http.Client's Transport, or pass it to http/client's
// WithTransport.
package transport
