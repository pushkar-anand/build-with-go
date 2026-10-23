package transport_test

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pushkar-anand/build-with-go/http/transport"
	"github.com/pushkar-anand/build-with-go/secret"
)

// Transports stack: each wraps the next. Here the dump is taken beneath the
// token transport, so it shows the Authorization header that is sent.
func Example() {
	hc := &http.Client{
		Transport: &transport.Bearer{
			Source: transport.StaticToken(secret.New("token")),
			Next:   &transport.Debug{Out: os.Stderr},
		},
	}

	_, _ = hc.Get("https://api.example.com/healthz")
}

// A source for expiring tokens caches one and fetches another only once it
// has run out. fetchToken stands in for a call to the issuer.
func ExampleTokenSourceFunc() {
	var (
		mu      sync.Mutex
		token   secret.Secret[string]
		expires time.Time
	)

	src := transport.TokenSourceFunc(func(ctx context.Context) (secret.Secret[string], error) {
		mu.Lock()
		defer mu.Unlock()

		if time.Now().Before(expires) {
			return token, nil
		}

		t, ttl, err := fetchToken(ctx)
		if err != nil {
			return secret.Secret[string]{}, err
		}

		// Refresh a little early, so a token does not expire in flight.
		token, expires = t, time.Now().Add(ttl-30*time.Second)

		return token, nil
	})

	_ = &http.Client{Transport: &transport.Bearer{Source: src}}
}

func fetchToken(context.Context) (secret.Secret[string], time.Duration, error) {
	return secret.New("token"), time.Hour, nil
}
