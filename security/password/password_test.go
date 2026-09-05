package password

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashAndCompare(t *testing.T) {
	t.Parallel()

	t.Run("same password", func(t *testing.T) {
		h := NewHasher(WithPepper("jsjfij390xm9"))
		p := "sjfyew8fAdw8e9ww"

		hash, err := h.Hash(p)
		require.NoError(t, err)

		err = h.Compare(p, hash)
		require.NoError(t, err)
	})
}

func TestCompare_RejectsUnsafeHashParams(t *testing.T) {
	t.Parallel()

	h := NewHasher()

	tests := map[string]string{
		"memory over absolute ceiling":      fmt.Sprintf(hashFormat, version, maxHashMemory+1, 1, 1, "AAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		"time over absolute ceiling":        fmt.Sprintf(hashFormat, version, 1024, maxHashTime+1, 1, "AAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		"parallelism over absolute ceiling": fmt.Sprintf(hashFormat, version, 1024, 1, maxHashParallelism+1, "AAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		// Well under the absolute ceilings, but more than
		// maxHashCostMultiplier times what this Hasher is configured with --
		// the case that protects a resource-constrained deployment.
		"memory over configured multiplier": fmt.Sprintf(hashFormat, version, h.memory*maxHashCostMultiplier+1, h.time, h.parallelism, "AAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		"time over configured multiplier":   fmt.Sprintf(hashFormat, version, h.memory, h.time*maxHashCostMultiplier+1, h.parallelism, "AAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
	}

	for name, hash := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := h.Compare("whatever", hash)
			require.ErrorIs(t, err, ErrHashParamsOutOfRange)
		})
	}
}

func TestCostCeiling(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		configured, absoluteMax, want uint64
	}{
		"zero configured falls back to absolute max": {configured: 0, absoluteMax: 100, want: 100},
		"relative bound tighter than absolute max":   {configured: 10, absoluteMax: 100, want: 20},
		"absolute max tighter than relative bound":   {configured: 1000, absoluteMax: 100, want: 100},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, costCeiling(tt.configured, tt.absoluteMax))
		})
	}
}

func TestCompare_RejectsOversizedHash(t *testing.T) {
	t.Parallel()

	h := NewHasher()

	oversized := strings.Repeat("A", maxEncodedHashLength+1)

	err := h.Compare("whatever", oversized)
	require.ErrorIs(t, err, ErrInvalidHashFormat)
}
