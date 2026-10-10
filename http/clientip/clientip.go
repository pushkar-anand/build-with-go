// Package clientip finds the address of the client behind an HTTP request.
//
// Behind a reverse proxy every request arrives from the proxy, and the
// client's address travels in a header the proxy writes. A client can send
// that header too, so a [Resolver] reads it only on a request that arrives
// from a proxy the caller trusts, and reads only the one header that proxy
// writes. Reading a second header as a fallback would let a client supply the
// one its proxy leaves alone.
package clientip

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"

	"github.com/pushkar-anand/build-with-go/ctxval"
)

// Header names the forwarding header a trusted proxy writes.
type Header string

const (
	// XForwardedFor is a comma-separated list of addresses, to which each
	// proxy appends the address it received the request from.
	XForwardedFor Header = "X-Forwarded-For"

	// XRealIP holds the client's address alone, set by the proxy in front.
	XRealIP Header = "X-Real-IP"

	// Forwarded is the RFC 7239 header. The for= parameter of each element
	// is read as X-Forwarded-For's entries are.
	Forwarded Header = "Forwarded"
)

// ParseHeader returns the Header that s names, ignoring case, so a choice can
// be read from configuration.
func ParseHeader(s string) (Header, error) {
	for _, h := range []Header{XForwardedFor, XRealIP, Forwarded} {
		if strings.EqualFold(s, string(h)) {
			return h, nil
		}
	}

	return "", fmt.Errorf("unknown forwarding header %q: want %s, %s or %s", s, XForwardedFor, XRealIP, Forwarded)
}

// ParsePrefix parses s as a CIDR prefix, or as a single address standing for
// a prefix of that address alone, so a list of trusted proxies can mix both.
func ParsePrefix(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return netip.Prefix{}, err
		}

		return p, nil
	}

	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}

	return netip.PrefixFrom(a, a.BitLen()), nil
}

// Option configures New.
type Option func(*Resolver)

// WithTrustedProxies sets the proxies whose forwarding header is read. With
// none, every request's client is the address it arrived from.
func WithTrustedProxies(prefixes ...netip.Prefix) Option {
	return func(r *Resolver) {
		for _, p := range prefixes {
			r.trusted = append(r.trusted, unmapPrefix(p))
		}
	}
}

// WithHeader sets the header read from a trusted proxy. The default is
// [XForwardedFor].
func WithHeader(h Header) Option {
	return func(r *Resolver) {
		r.header = h
	}
}

// WithLogger sets where the Resolver reports a forwarding header that arrived
// from an address it does not trust. The default is slog.Default.
func WithLogger(log *slog.Logger) Option {
	return func(r *Resolver) {
		if log != nil {
			r.log = log
		}
	}
}

// Resolver finds the client address of a request. A Resolver is safe for
// concurrent use.
type Resolver struct {
	trusted []netip.Prefix
	header  Header
	log     *slog.Logger

	// warned is set after the first warning about a forwarding header from
	// an untrusted address. One warning is enough to point at a proxy left
	// out of the trusted list, and more would let any client fill the log.
	warned atomic.Bool
}

// New returns a Resolver configured by opts.
func New(opts ...Option) *Resolver {
	r := &Resolver{
		header: XForwardedFor,
		log:    slog.Default(),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Middleware places each request's client address in its context, where
// ctxval.ClientAddrFromContext finds it. A request whose connection has no IP
// address, such as one over a Unix socket, gets none.
func (r *Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if addr, ok := r.Addr(req); ok {
			req = req.WithContext(ctxval.WithClientAddr(req.Context(), addr))
		}

		next.ServeHTTP(w, req)
	})
}

// Addr returns the address of the client behind req. From a trusted proxy it
// is the nearest address in the forwarding header that is not itself a trusted
// proxy, or the farthest one when every address is trusted. From anywhere
// else, or when the header holds an entry that is not an address, it is the
// address req arrived from. The bool is false only when req.RemoteAddr holds
// no IP address.
func (r *Resolver) Addr(req *http.Request) (netip.Addr, bool) {
	peer, ok := parseAddr(req.RemoteAddr)
	if !ok {
		return netip.Addr{}, false
	}

	values := req.Header.Values(string(r.header))
	if len(values) == 0 {
		return peer, true
	}

	if !r.trusts(peer) {
		if r.warned.CompareAndSwap(false, true) {
			r.log.WarnContext(req.Context(), "forwarding header from an untrusted address ignored",
				slog.String("header", string(r.header)),
				slog.String("from", peer.String()),
			)
		}

		return peer, true
	}

	var hops []string

	switch r.header {
	case XRealIP:
		// One address, set by the proxy in front. A request carrying more
		// than one is ambiguous, so none of them is believed.
		if len(values) != 1 {
			return peer, true
		}

		hops = values
	case Forwarded:
		hops = forwardedFor(values)
	default:
		for _, v := range values {
			hops = append(hops, strings.Split(v, ",")...)
		}
	}

	return r.walk(peer, hops), true
}

// walk returns the client address in hops, the header's entries in the order
// proxies added them. Entries are read from the nearest proxy outwards, since
// only the entries trusted proxies added can be believed: anything to the left
// of the first untrusted address could have been written by that client.
func (r *Resolver) walk(peer netip.Addr, hops []string) netip.Addr {
	var addr netip.Addr

	for i := len(hops) - 1; i >= 0; i-- {
		var ok bool

		addr, ok = parseAddr(strings.TrimSpace(hops[i]))
		if !ok {
			return peer
		}

		if !r.trusts(addr) {
			return addr
		}
	}

	// Every entry was a trusted proxy. The farthest is the best account of
	// where the request began.
	if addr.IsValid() {
		return addr
	}

	return peer
}

// trusts reports whether addr is in one of the trusted prefixes.
func (r *Resolver) trusts(addr netip.Addr) bool {
	for _, p := range r.trusted {
		if p.Contains(addr) {
			return true
		}
	}

	return false
}

// forwardedFor returns the for= value of each element in a Forwarded header's
// values, in order. An element without exactly one for= parameter contributes
// an empty value, which no address parses from, so the walk stops there.
func forwardedFor(values []string) []string {
	var hops []string

	for _, v := range values {
		for _, element := range splitQuoted(v, ',') {
			var (
				hop   string
				found int
			)

			for _, pair := range splitQuoted(element, ';') {
				key, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
				if ok && strings.EqualFold(strings.TrimSpace(key), "for") {
					hop = unquote(strings.TrimSpace(value))
					found++
				}
			}

			if found != 1 {
				hop = ""
			}

			hops = append(hops, hop)
		}
	}

	return hops
}

// splitQuoted splits s at each sep outside a quoted string, as RFC 7239's
// grammar allows a quoted value to hold either separator.
func splitQuoted(s string, sep byte) []string {
	var (
		parts  []string
		start  int
		quoted bool
	)

	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && quoted:
			i++ // the escaped character is part of the value
		case c == '"':
			quoted = !quoted
		case c == sep && !quoted:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}

	return append(parts, s[start:])
}

// unquote returns the content of the quoted string s with its escapes
// removed, or s itself when it is not quoted.
func unquote(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}

	var b strings.Builder

	for i := 1; i < len(s)-1; i++ {
		if s[i] == '\\' && i+1 < len(s)-1 {
			i++
		}

		b.WriteByte(s[i])
	}

	return b.String()
}

// parseAddr returns the address in s, which may carry a port, bracketed or
// not, as RemoteAddr and the forwarding headers write them. An IPv4 address
// mapped into IPv6 comes back as plain IPv4, and a zone is dropped, so one
// client has one form however it arrived.
func parseAddr(s string) (netip.Addr, bool) {
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap().WithZone(""), true
	}

	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap().WithZone(""), true
	}

	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		if a, err := netip.ParseAddr(s[1 : len(s)-1]); err == nil {
			return a.Unmap().WithZone(""), true
		}
	}

	return netip.Addr{}, false
}

// unmapPrefix returns p over plain IPv4 when it covers IPv4 addresses mapped
// into IPv6, since parseAddr unmaps every address it is matched against.
func unmapPrefix(p netip.Prefix) netip.Prefix {
	if p.Addr().Is4In6() && p.Bits() >= 96 {
		return netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96).Masked()
	}

	return p.Masked()
}
