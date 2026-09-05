package password

import (
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
