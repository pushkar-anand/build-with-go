package clientip_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"

	"github.com/pushkar-anand/build-with-go/ctxval"
	"github.com/pushkar-anand/build-with-go/http/clientip"
)

// A service behind a proxy at 192.0.2.1 that appends to X-Forwarded-For. The
// first entry was written by the client and is not believed; the last was
// written by the proxy.
func Example() {
	resolver := clientip.New(
		clientip.WithTrustedProxies(netip.MustParsePrefix("192.0.2.1/32")),
		clientip.WithHeader(clientip.XForwardedFor),
	)

	handler := resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		addr, _ := ctxval.ClientAddrFromContext(r.Context())
		fmt.Println(addr)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:4711"
	req.Header.Set("X-Forwarded-For", "198.51.100.66, 203.0.113.7")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	// Output: 203.0.113.7
}
