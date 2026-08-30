package response

import (
	"context"
	json "encoding/json/v2"
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
			require.NoError(t, json.UnmarshalRead(w.Body, &got))
			assert.Equal(t, tt.wantBody, got)
		})
	}
}

func teapot() Problem {
	return NewProblem().WithStatus(http.StatusTeapot).WithDetail("mapped").Build()
}

// encoding/json/v2 renders a nil slice as [] and a nil map as {}, where v1
// rendered both as null. This pins that wire format so the change is explicit.
func TestJSONWriter_NilSlicesAndMapsEncodeAsEmpty(t *testing.T) {
	t.Parallel()

	type payload struct {
		Posts []string          `json:"posts"`
		Meta  map[string]string `json:"meta"`
	}

	w := httptest.NewRecorder()

	NewJSONWriter(slog.New(slog.DiscardHandler)).
		Ok(context.Background(), w, payload{})

	assert.JSONEq(t, `{"posts":[],"meta":{}}`, w.Body.String())
}

func TestJSONWriter_ToStandardHandler(t *testing.T) {
	t.Parallel()

	writer := NewJSONWriter(slog.New(slog.DiscardHandler))

	t.Run("a handler returning nil writes nothing extra", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/orders", nil)

		writer.ToStandardHandler(func(w http.ResponseWriter, _ *http.Request) error {
			w.WriteHeader(http.StatusCreated)
			return nil
		})(w, r)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("a returned error becomes a problem document", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/orders", nil)

		writer.ToStandardHandler(func(http.ResponseWriter, *http.Request) error {
			return &domainProblem{status: http.StatusConflict, detail: "item is out of stock"}
		})(w, r)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "application/problem+json; charset=utf-8", w.Header().Get("Content-Type"))

		var got map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		assert.Equal(t, "item is out of stock", got["detail"])
	})

	// The context a handler derives reaches only what it attaches it to.
	t.Run("the handler reads its context from the request", func(t *testing.T) {
		t.Parallel()

		type key struct{}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/orders", nil)
		r = r.WithContext(context.WithValue(r.Context(), key{}, "carried"))

		var seen any

		writer.ToStandardHandler(func(_ http.ResponseWriter, r *http.Request) error {
			seen = r.Context().Value(key{})
			return nil
		})(w, r)

		assert.Equal(t, "carried", seen)
	})
}
