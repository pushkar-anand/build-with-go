package logger

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErr(t *testing.T) {
	t.Parallel()

	t.Run("an error is logged under the error key", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer

		slog.New(slog.NewTextHandler(&buf, nil)).
			Info("failed", Err(errors.New("boom")))

		assert.Contains(t, buf.String(), `error=boom`)
	})

	// Logging a maybe-nil error should not leave "error=<nil>" in the line.
	t.Run("a nil error is dropped", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer

		slog.New(slog.NewTextHandler(&buf, nil)).
			Info("fine", Err(nil))

		assert.NotContains(t, buf.String(), "error")
		assert.True(t, strings.HasSuffix(strings.TrimSpace(buf.String()), `msg=fine`))
	})
}
