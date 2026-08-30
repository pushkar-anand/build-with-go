package http_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pushkar-anand/build-with-go/http/request"
	"github.com/pushkar-anand/build-with-go/http/response"
	"github.com/pushkar-anand/build-with-go/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// response no longer special-cases request's error types; they are handled
// because they implement response.Problem. The assertions in http.go guard that
// at compile time, and this guards the behaviour they stand for.
func TestRequestErrorsAreWrittenAsProblems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantDetail  string
		wantContext map[string]any
	}{
		{
			name: "read error",
			err: &request.ReadError{
				HTTPStatusCode: http.StatusBadRequest,
				Message:        "Request body must not be empty",
			},
			wantStatus: http.StatusBadRequest,
			wantDetail: "Request body must not be empty",
		},
		{
			name: "validation error carries its failures as a custom member",
			err: &request.ValidationError{
				HTTPStatusCode: http.StatusUnprocessableEntity,
				Message:        "Request is not valid",
				Problems: map[string]any{
					"title": validator.Reason{Rule: "required", Message: "title is required"},
				},
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantDetail: "Request is not valid",
			// The failures are the context member directly. Previously this was
			// the validator's Result, wrapping them in Valid/Failed keys.
			wantContext: map[string]any{
				"title": map[string]any{
					"value":   nil,
					"rule":    "required",
					"message": "title is required",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/posts", nil)

			response.NewJSONWriter(slog.New(slog.DiscardHandler)).
				WriteError(w, r, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)

			var got map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&got))

			assert.Equal(t, tt.wantDetail, got["detail"])
			assert.Equal(t, http.StatusText(tt.wantStatus), got["title"])
			assert.Equal(t, "about:blank", got["type"])
			assert.Equal(t, "/posts", got["instance"])

			if tt.wantContext != nil {
				assert.Equal(t, tt.wantContext, got["context"])
			} else {
				assert.NotContains(t, got, "context")
			}
		})
	}
}
