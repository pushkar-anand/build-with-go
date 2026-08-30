package request

import (
	json "encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummy struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func Test_parseReadError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		opts        []json.Options
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "empty body",
			input:       ``,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Request body must not be empty",
		},
		{
			name:        "truncated object",
			input:       `{"name": "John", "age": 30`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Request body contains badly-formed JSON at offset 26",
		},
		{
			name:        "wrong type for a field",
			input:       `{"name": "John", "age": "thirty"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: `Request body contains an invalid value for the "age" field, expecting: int`,
		},
		{
			name:        "duplicate field",
			input:       `{"name": "John", "name": "Jane"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: `Request body contains the duplicate field "name"`,
		},
		{
			name:        "trailing data after the value",
			input:       `{"name": "John"}{"name": "Jane"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Request body contains badly-formed JSON at offset 16",
		},
		{
			name:        "invalid utf-8",
			input:       "{\"name\": \"\xff\"}",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Request body contains badly-formed JSON at offset 10",
		},
		{
			name:        "unknown field rejected when configured",
			input:       `{"name": "John", "nope": true}`,
			opts:        []json.Options{json.RejectUnknownMembers(true)},
			wantStatus:  http.StatusBadRequest,
			wantMessage: `Request body contains unknown field "nope"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ReadJSONBody[dummy](strings.NewReader(tt.input), tt.opts...)

			var readErr *ReadError
			require.ErrorAs(t, err, &readErr)

			assert.Equal(t, tt.wantStatus, readErr.HTTPStatusCode)
			assert.Equal(t, tt.wantMessage, readErr.Message)
			assert.ErrorIs(t, err, readErr.UnderlyingErr)
		})
	}
}

// Unknown fields are ignored unless the caller opts in.
func Test_ReadJSONBody_IgnoresUnknownFieldsByDefault(t *testing.T) {
	t.Parallel()

	got, err := ReadJSONBody[dummy](strings.NewReader(`{"name":"John","nope":true}`))

	require.NoError(t, err)
	assert.Equal(t, &dummy{Name: "John"}, got)
}

func Test_parseReadError_BodyTooLarge(t *testing.T) {
	t.Parallel()

	body := http.MaxBytesReader(
		httptest.NewRecorder(),
		io.NopCloser(strings.NewReader(`{"name":"a considerably longer value"}`)),
		8,
	)

	_, err := ReadJSONBody[dummy](body)

	var readErr *ReadError
	require.ErrorAs(t, err, &readErr)

	assert.Equal(t, http.StatusRequestEntityTooLarge, readErr.HTTPStatusCode)
	assert.Equal(t, "Request body must not be larger than 8 bytes", readErr.Message)
}

// The client-facing wording and the Go error string are different audiences:
// Detail reads as a sentence, Error follows Go's lower-case, prefixed form.
func TestReadError_ErrorStringFollowsGoConvention(t *testing.T) {
	t.Parallel()

	err := &ReadError{
		HTTPStatusCode: http.StatusBadRequest,
		Message:        "Request body must not be empty",
	}

	assert.Equal(t, "request: request body must not be empty", err.Error())
	assert.Equal(t, "Request body must not be empty", err.Detail())
}

func TestValidationError_ErrorStringNamesTheFields(t *testing.T) {
	t.Parallel()

	err := &ValidationError{
		HTTPStatusCode: http.StatusUnprocessableEntity,
		Message:        "Request is not valid",
		Problems: map[string]any{
			"title": "required",
			"body":  "required",
		},
	}

	// Sorted, so a log line is stable regardless of map iteration order.
	assert.Equal(t, "request: validation failed: body, title", err.Error())
	assert.Equal(t, "Request is not valid", err.Detail())
}
