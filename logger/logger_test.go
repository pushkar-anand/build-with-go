package logger

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormat_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format Format
		want   string
	}{
		{format: FormatJSON, want: "json"},
		{format: FormatText, want: "text"},
		// An out-of-range value should say so rather than print a bare number.
		{format: Format(7), want: "Format(7)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.format.String())
			assert.Equal(t, tt.want, fmt.Sprintf("%v", tt.format))
		})
	}
}
