package http_test

import (
	"context"
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
		name       string
		err        error
		wantStatus int
		wantDetail string
		wantCtx    bool
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
				ReadError: request.ReadError{
					HTTPStatusCode: http.StatusUnprocessableEntity,
					Message:        "Request is not valid",
				},
				Result: &validator.Result{
					Valid: false,
					Failed: map[string]validator.Reason{
						"title": {Rule: "required", Message: "title is required"},
					},
				},
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantDetail: "Request is not valid",
			wantCtx:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/posts", nil)

			response.NewJSONWriter(slog.New(slog.DiscardHandler)).
				WriteError(context.Background(), r, w, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)

			var got map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&got))

			assert.Equal(t, tt.wantDetail, got["detail"])
			assert.Equal(t, http.StatusText(tt.wantStatus), got["title"])
			assert.Equal(t, "about:blank", got["type"])
			assert.Equal(t, "/posts", got["instance"])

			if tt.wantCtx {
				assert.NotNil(t, got["context"], "validation failures should be exposed")
			}
		})
	}
}
