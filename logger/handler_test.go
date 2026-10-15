package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/pushkar-anand/build-with-go/ctxval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextHandler(t *testing.T) {
	t.Parallel()

	addr := netip.MustParseAddr("203.0.113.7")

	t.Run("adds the request ID and client address", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		log := New(WithWriter(&buf), WithFormat(FormatJSON))

		ctx := ctxval.WithRequestID(context.Background(), "req-1")
		ctx = ctxval.WithClientAddr(ctx, addr)
		log.InfoContext(ctx, "hello")

		var entry map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
		assert.Equal(t, "req-1", entry["request_id"])
		assert.Equal(t, "203.0.113.7", entry["remote_ip"])
	})

	t.Run("omits values the context lacks", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		log := New(WithWriter(&buf), WithFormat(FormatJSON))

		log.InfoContext(context.Background(), "hello")

		var entry map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
		assert.NotContains(t, entry, "request_id")
		assert.NotContains(t, entry, "remote_ip")
	})

	t.Run("keeps a remote_ip the record already carries", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		log := New(WithWriter(&buf), WithFormat(FormatJSON))

		ctx := ctxval.WithClientAddr(context.Background(), addr)
		log.InfoContext(ctx, "hello", slog.String("remote_ip", "198.51.100.1"))

		assert.Equal(t, 1, strings.Count(buf.String(), `"remote_ip"`))
		assert.Contains(t, buf.String(), `"remote_ip":"198.51.100.1"`)
	})
}
