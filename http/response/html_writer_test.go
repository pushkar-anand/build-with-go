package response

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// errorPages is a template set with a page for 404 and one for 500.
func errorPages(t *testing.T) *template.Template {
	t.Helper()

	root := template.New("root")
	template.Must(root.New("errors/not-found").Parse(`<h1>{{.Status}}</h1><p>{{.Title}}</p>`))
	template.Must(root.New("errors/server-error").Parse(`<h1>Error {{.Status}}</h1>`))

	return root
}

func newHTMLWriter(t *testing.T, opts ...HTMLOption) *HTMLWriter {
	t.Helper()

	return NewHTMLWriter(slog.New(slog.DiscardHandler), errorPages(t), opts...)
}

func TestHTMLWriter_WriteError(t *testing.T) {
	t.Parallel()

	pages := WithErrorTemplates(map[int]string{
		http.StatusNotFound:            "errors/not-found",
		http.StatusInternalServerError: "errors/server-error",
	})

	tests := []struct {
		name       string
		err        error
		opts       []HTMLOption
		wantStatus int
		wantBody   string
	}{
		{
			name:       "a Problem error renders the page for its status",
			err:        &domainProblem{status: http.StatusNotFound, detail: "gone"},
			opts:       []HTMLOption{pages},
			wantStatus: http.StatusNotFound,
			wantBody:   "<h1>404</h1><p>Not Found</p>",
		},
		{
			name:       "a wrapped Problem is still found",
			err:        fmt.Errorf("saving: %w", &domainProblem{status: http.StatusInternalServerError, detail: "boom"}),
			opts:       []HTMLOption{pages},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "<h1>Error 500</h1>",
		},
		{
			name:       "a plain error renders the 500 page",
			err:        errors.New("boom"),
			opts:       []HTMLOption{pages},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "<h1>Error 500</h1>",
		},
		{
			name: "the status mapper maps errors that are not a Problem",
			err:  errors.New("no rows"),
			opts: []HTMLOption{
				pages,
				WithErrorStatusMapper(func(error) int { return http.StatusNotFound }),
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "<h1>404</h1><p>Not Found</p>",
		},
		{
			name: "a mapper returning zero falls back to 500",
			err:  errors.New("boom"),
			opts: []HTMLOption{
				pages,
				WithErrorStatusMapper(func(error) int { return 0 }),
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "<h1>Error 500</h1>",
		},
		{
			name:       "a status with no configured template falls back to a plain response",
			err:        &domainProblem{status: http.StatusConflict, detail: "nope"},
			opts:       []HTMLOption{pages},
			wantStatus: http.StatusConflict,
			wantBody:   "Conflict\n",
		},
		{
			name:       "with no templates configured it falls back to a plain response",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/x", nil)

			newHTMLWriter(t, tt.opts...).WriteError(w, r, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}

func TestHTMLWriter_ErrorPage(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)

	newHTMLWriter(t, WithErrorTemplates(map[int]string{
		http.StatusNotFound: "errors/not-found",
	})).ErrorPage(w, r, http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, "<h1>404</h1><p>Not Found</p>", w.Body.String())
}

func TestHTMLWriter_Handle(t *testing.T) {
	t.Parallel()

	writer := newHTMLWriter(t, WithErrorTemplates(map[int]string{
		http.StatusNotFound: "errors/not-found",
	}))

	t.Run("a returned error renders the error page", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/x", nil)

		writer.Handle(func(http.ResponseWriter, *http.Request) error {
			return &domainProblem{status: http.StatusNotFound, detail: "gone"}
		})(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "<h1>404</h1><p>Not Found</p>", w.Body.String())
	})

	t.Run("a nil error leaves the handler's response alone", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/x", nil)

		writer.Handle(func(w http.ResponseWriter, _ *http.Request) error {
			w.WriteHeader(http.StatusTeapot)
			return nil
		})(w, r)

		assert.Equal(t, http.StatusTeapot, w.Code)
		assert.Empty(t, w.Body.String())
	})
}
