package ctxval

import (
	"context"
	"net/netip"
)

const clientAddrKey contextKey = "client_addr"

// WithClientAddr adds the address of the client behind a request to ctx.
func WithClientAddr(ctx context.Context, addr netip.Addr) context.Context {
	return context.WithValue(ctx, clientAddrKey, addr)
}

// ClientAddrFromContext returns the client address that http/clientip placed
// in ctx. The bool is false when no address was placed, as when the
// middleware did not run or the connection had no IP address.
func ClientAddrFromContext(ctx context.Context) (netip.Addr, bool) {
	addr, ok := ctx.Value(clientAddrKey).(netip.Addr)
	return addr, ok
}
