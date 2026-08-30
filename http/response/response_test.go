package response

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// domainProblem is a caller-defined error that describes its own response.
type domainProblem struct {
	status int
	detail string
}

func (e *domainProblem) Error() string  { return e.detail }
func (e *domainProblem) Type() string   { return "https://example.com/probs/out-of-stock" }
func (e *domainProblem) Title() string  { return "Out of stock" }
func (e *domainProblem) Status() int    { return e.status }
func (e *domainProblem) Detail() string { return e.detail }

func (e *domainProblem) CustomMembers() map[string]any {
	return map[string]any{"sku": "ABC-123"}
}

var _ Problem = (*domainProblem)(nil)

func TestJSONWriter_WriteError(t *testing.T) {
	t.Parallel()

	outOfStock := &domainProblem{status: http.StatusConflict, detail: "item is out of stock"}

	outOfStockBody := map[string]any{
		"type":     "https://example.com/probs/out-of-stock",
		"title":    "Out of stock",
		"status":   float64(http.StatusConflict),
		"detail":   "item is out of stock",
		"instance": "/orders",
		"sku":      "ABC-123",
	}

	genericServerError := map[string]any{
		"type":     "about:blank",
		"title":    http.StatusText(http.StatusInternalServerError),
		"status":   float64(http.StatusInternalServerError),
		"detail":   http.StatusText(http.StatusInternalServerError),
		"instance": "/orders",
	}

	tests := []struct {
		name       string
		err        error
		opts       []Option
		wantStatus int
		wantBody   map[string]any
	}{
		{
			name:       "an error implementing Problem describes itself, with no mapper configured",
			err:        outOfStock,
			wantStatus: http.StatusConflict,
			wantBody:   outOfStockBody,
		},
		{
			name:       "a wrapped Problem is still found",
			err:        fmt.Errorf("placing order: %w", outOfStock),
			wantStatus: http.StatusConflict,
			wantBody:   outOfStockBody,
		},
		{
			name:       "a Problem takes precedence over the mapper",
			err:        outOfStock,
			opts:       []Option{WithErrorProblemMapper(func(error) Problem { return teapot() })},
			wantStatus: http.StatusConflict,
			wantBody:   outOfStockBody,
		},
		{
			name:       "the mapper handles errors that do not implement Problem",
			err:        errors.New("boom"),
			opts:       []Option{WithErrorProblemMapper(func(error) Problem { return teapot() })},
			wantStatus: http.StatusTeapot,
			wantBody: map[string]any{
				"type":     "about:blank",
				"title":    http.StatusText(http.StatusTeapot),
				"status":   float64(http.StatusTeapot),
				"detail":   "mapped",
				"instance": "/orders",
			},
		},
		{
			name:       "an unmapped error falls back to a generic 500",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   genericServerError,
		},
		{
			name:       "a mapper returning nil falls back to a generic 500",
			err:        errors.New("boom"),
			opts:       []Option{WithErrorProblemMapper(func(error) Problem { return nil })},
			wantStatus: http.StatusInternalServerError,
			wantBody:   genericServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/orders", nil)

			NewJSONWriter(slog.New(slog.DiscardHandler), tt.opts...).
				WriteError(context.Background(), r, w, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, "application/problem+json; charset=utf-8", w.Header().Get("Content-Type"))
			assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))

			var got map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
			assert.Equal(t, tt.wantBody, got)
		})
	}
}

func teapot() Problem {
	return NewProblem().WithStatus(http.StatusTeapot).WithDetail("mapped").Build()
}
