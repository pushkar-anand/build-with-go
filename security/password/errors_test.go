package password_test

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pushkar-anand/build-with-go/http/response"
	"github.com/pushkar-anand/build-with-go/security/password"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ response.Problem = password.ErrInvalidHashFormat

func TestPasswordErrorResponses(t *testing.T) {
	tests := []struct {
		err    error
		status int
		detail string
	}{
		{password.ErrPasswordTooLong, 422, "Password is too long"},
		{password.ErrMismatchedHashAndPassword, 401, "Invalid credentials"},
		{password.ErrInvalidHashFormat, 500, "Internal Server Error"},
		{password.ErrInvalidHashVersion, 500, "Internal Server Error"},
		{password.ErrHashParamsOutOfRange, 500, "Internal Server Error"},
	}
	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			wrapped := fmt.Errorf("checking password: %w", tt.err)
			require.True(t, errors.Is(wrapped, tt.err))
			for _, err := range []error{tt.err, wrapped} {
				r := httptest.NewRequest(http.MethodPost, "/login", nil)
				w := httptest.NewRecorder()
				response.NewJSONWriter(nil).Handle(func(http.ResponseWriter, *http.Request) error { return err })(w, r)
				assert.Equal(t, tt.status, w.Code)
				assert.Equal(t, "application/problem+json; charset=utf-8", w.Header().Get("Content-Type"))
				var body map[string]any
				require.NoError(t, json.UnmarshalRead(w.Body, &body))
				assert.Equal(t, map[string]any{"type": "about:blank", "title": http.StatusText(tt.status), "status": float64(tt.status), "detail": tt.detail, "instance": "/login"}, body)
			}
		})
	}
}

func TestPasswordErrorMapperOverrides(t *testing.T) {
	err := fmt.Errorf("checking password: %w", password.ErrMismatchedHashAndPassword)
	r := httptest.NewRequest(http.MethodPost, "/password", nil)
	for _, override := range []bool{false, true} {
		status := http.StatusUnauthorized
		if override {
			status = http.StatusUnprocessableEntity
		}
		w := httptest.NewRecorder()
		response.NewJSONWriter(nil, response.WithErrorProblemMapper(func(got error) response.Problem {
			require.ErrorIs(t, got, password.ErrMismatchedHashAndPassword)
			if override {
				return response.NewProblem().WithStatus(status).WithDetail("Current password is incorrect").Build()
			}
			return nil
		})).WriteError(w, r, err)
		assert.Equal(t, status, w.Code)
		w = httptest.NewRecorder()
		response.NewHTMLWriter(nil, nil, response.WithErrorStatusMapper(func(got error) int {
			require.ErrorIs(t, got, password.ErrMismatchedHashAndPassword)
			if override {
				return status
			}
			return 0
		})).WriteError(w, r, err)
		assert.Equal(t, status, w.Code)
	}
}
