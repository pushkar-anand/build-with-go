package ctxval

import (
	"context"
	"net/netip"
	"testing"
)

func TestContextClientAddr(t *testing.T) {
	ctx := context.Background()

	if _, ok := ClientAddrFromContext(ctx); ok {
		t.Error("expected no client address in empty context")
	}

	want := netip.MustParseAddr("203.0.113.7")
	ctx = WithClientAddr(ctx, want)

	got, ok := ClientAddrFromContext(ctx)
	if !ok {
		t.Error("expected to find client address in context")
	}

	if got != want {
		t.Errorf("expected %v, got %v", want, got)
	}
}
