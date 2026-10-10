package clientip

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/pushkar-anand/build-with-go/ctxval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAddr covers which address a request resolves to, for each header, from
// trusted and untrusted peers. 192.0.2.0/24 and 2001:db8:1::/48 are the
// trusted proxies throughout.
func TestAddr(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("2001:db8:1::/48"),
	}

	tests := []struct {
		name    string
		header  Header
		remote  string
		values  []string // the header's lines, in order
		want    string
		wantNot bool // RemoteAddr holds no IP address
	}{
		{
			name:   "no header",
			remote: "203.0.113.7:4711",
			want:   "203.0.113.7",
		},
		{
			name:   "header from an untrusted peer is ignored",
			remote: "198.51.100.9:4711",
			values: []string{"203.0.113.7"},
			want:   "198.51.100.9",
		},
		{
			name:   "trusted peer without the header is the client",
			remote: "192.0.2.1:4711",
			want:   "192.0.2.1",
		},
		{
			name:   "one hop",
			remote: "192.0.2.1:4711",
			values: []string{"203.0.113.7"},
			want:   "203.0.113.7",
		},
		{
			name:   "a client-written entry left of the real one is skipped",
			remote: "192.0.2.1:4711",
			values: []string{"198.51.100.66, 203.0.113.7"},
			want:   "203.0.113.7",
		},
		{
			name:   "trusted hops are skipped",
			remote: "192.0.2.1:4711",
			values: []string{"203.0.113.7, 192.0.2.2, 192.0.2.3"},
			want:   "203.0.113.7",
		},
		{
			name:   "every hop trusted gives the farthest",
			remote: "192.0.2.1:4711",
			values: []string{"192.0.2.5, 192.0.2.2"},
			want:   "192.0.2.5",
		},
		{
			name:   "lines join in order",
			remote: "192.0.2.1:4711",
			values: []string{"198.51.100.66", "203.0.113.7, 192.0.2.2"},
			want:   "203.0.113.7",
		},
		{
			name:   "unparseable entry gives the peer",
			remote: "192.0.2.1:4711",
			values: []string{"203.0.113.7, not-an-address"},
			want:   "192.0.2.1",
		},
		{
			name:   "empty header gives the peer",
			remote: "192.0.2.1:4711",
			values: []string{""},
			want:   "192.0.2.1",
		},
		{
			name:   "entries may carry a port",
			remote: "192.0.2.1:4711",
			values: []string{"[2001:db8:2::7]:4711"},
			want:   "2001:db8:2::7",
		},
		{
			name:   "mapped IPv4 peer matches an IPv4 prefix",
			remote: "[::ffff:192.0.2.1]:4711",
			values: []string{"::ffff:203.0.113.7"},
			want:   "203.0.113.7",
		},
		{
			name:   "IPv6 peer",
			remote: "[2001:db8:1::1]:4711",
			values: []string{"2001:db8:2::7"},
			want:   "2001:db8:2::7",
		},
		{
			name:    "no IP address on the connection",
			remote:  "@",
			wantNot: true,
		},
		{
			name:   "X-Real-IP from a trusted peer",
			header: XRealIP,
			remote: "192.0.2.1:4711",
			values: []string{"203.0.113.7"},
			want:   "203.0.113.7",
		},
		{
			name:   "X-Real-IP from an untrusted peer is ignored",
			header: XRealIP,
			remote: "198.51.100.9:4711",
			values: []string{"203.0.113.7"},
			want:   "198.51.100.9",
		},
		{
			name:   "two X-Real-IP lines give the peer",
			header: XRealIP,
			remote: "192.0.2.1:4711",
			values: []string{"203.0.113.7", "203.0.113.8"},
			want:   "192.0.2.1",
		},
		{
			name:   "Forwarded",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{"for=198.51.100.66, for=203.0.113.7;proto=https;by=192.0.2.1"},
			want:   "203.0.113.7",
		},
		{
			name:   "Forwarded with a quoted IPv6 address and port",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{`For="[2001:db8:2::7]:4711"`},
			want:   "2001:db8:2::7",
		},
		{
			name:   "Forwarded with a quoted IPv4 address and port",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{`for="203.0.113.7:4711"`},
			want:   "203.0.113.7",
		},
		{
			name:   "Forwarded separator inside quotes",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{`for=203.0.113.7;host="a,b;c.example"`},
			want:   "203.0.113.7",
		},
		{
			name:   "Forwarded trusted hops are skipped",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{"for=203.0.113.7", "for=192.0.2.2"},
			want:   "203.0.113.7",
		},
		{
			name:   "Forwarded unknown gives the peer",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{"for=unknown"},
			want:   "192.0.2.1",
		},
		{
			name:   "Forwarded obfuscated identifier gives the peer",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{"for=_hidden"},
			want:   "192.0.2.1",
		},
		{
			name:   "Forwarded element without for gives the peer",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{"for=198.51.100.66, proto=https"},
			want:   "192.0.2.1",
		},
		{
			name:   "Forwarded element with two for gives the peer",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			values: []string{"for=198.51.100.66;for=203.0.113.7"},
			want:   "192.0.2.1",
		},
		{
			name:   "X-Forwarded-For is not read when Forwarded is chosen",
			header: Forwarded,
			remote: "192.0.2.1:4711",
			want:   "192.0.2.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header := tc.header
			if header == "" {
				header = XForwardedFor
			}

			r := New(
				WithTrustedProxies(trusted...),
				WithHeader(header),
				WithLogger(slog.New(slog.DiscardHandler)),
			)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote

			for _, v := range tc.values {
				req.Header.Add(string(header), v)
			}

			// A header the resolver was not told to read must make no
			// difference.
			if header != XForwardedFor {
				req.Header.Set("X-Forwarded-For", "198.51.100.66")
			}

			got, ok := r.Addr(req)
			if tc.wantNot {
				assert.False(t, ok)
				return
			}

			require.True(t, ok)
			assert.Equal(t, tc.want, got.String())
		})
	}
}

// TestNoTrustedProxies covers the default: every forwarding header is ignored.
func TestNoTrustedProxies(t *testing.T) {
	r := New(WithLogger(slog.New(slog.DiscardHandler)))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:4711"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	got, ok := r.Addr(req)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", got.String())
}

// TestWarnsOnce covers the warning about a header from an untrusted address:
// it is logged for the first such request only.
func TestWarnsOnce(t *testing.T) {
	var buf bytes.Buffer

	r := New(WithLogger(slog.New(slog.NewTextHandler(&buf, nil))))

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "198.51.100.9:4711"
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		r.Addr(req)
	}

	assert.Equal(t, 1, strings.Count(buf.String(), "untrusted address"))
	assert.Contains(t, buf.String(), "from=198.51.100.9")
	assert.Contains(t, buf.String(), "header=X-Forwarded-For")
}

// TestMiddleware covers the address reaching the handler's context, and none
// arriving when the connection has no IP address.
func TestMiddleware(t *testing.T) {
	r := New(WithTrustedProxies(netip.MustParsePrefix("192.0.2.0/24")))

	var (
		got netip.Addr
		ok  bool
	)

	h := r.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		got, ok = ctxval.ClientAddrFromContext(req.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:4711"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	h.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, ok)
	assert.Equal(t, "203.0.113.7", got.String())

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "@"
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.False(t, ok)
}

// TestParseHeader covers matching a configured name without regard to case,
// and refusing a name that is not one of the three.
func TestParseHeader(t *testing.T) {
	for in, want := range map[string]Header{
		"x-forwarded-for": XForwardedFor,
		"X-Real-IP":       XRealIP,
		"FORWARDED":       Forwarded,
	} {
		got, err := ParseHeader(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	_, err := ParseHeader("x-client-ip")
	assert.ErrorContains(t, err, `"x-client-ip"`)
}

// TestParsePrefix covers a CIDR, a bare address standing for itself alone,
// and refusing anything else.
func TestParsePrefix(t *testing.T) {
	for in, want := range map[string]string{
		"192.0.2.0/24":     "192.0.2.0/24",
		"192.0.2.1":        "192.0.2.1/32",
		"2001:db8::/32":    "2001:db8::/32",
		"2001:db8::1":      "2001:db8::1/128",
		"::ffff:192.0.2.1": "::ffff:192.0.2.1/128",
	} {
		got, err := ParsePrefix(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got.String(), in)
	}

	for _, in := range []string{"", "proxy.example", "192.0.2.0/33"} {
		_, err := ParsePrefix(in)
		assert.Error(t, err, in)
	}
}
