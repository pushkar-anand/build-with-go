package request

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	validatorpkg "github.com/pushkar-anand/build-with-go/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type createPost struct {
	Title string `json:"title" schema:"title" validate:"required,min=3"`
	Draft bool   `json:"draft" schema:"draft"`
}

type listPosts struct {
	Page int `json:"page" schema:"page" validate:"min=1"`
}

func newTestReader(t *testing.T) *Reader {
	t.Helper()

	v, err := validatorpkg.New()
	require.NoError(t, err)

	return NewReader(slog.New(slog.DiscardHandler), v)
}

func TestReader_ReadAndValidateJSON(t *testing.T) {
	t.Parallel()

	r := newTestReader(t)

	t.Run("valid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts",
			strings.NewReader(`{"title":"hello","draft":true}`))

		got, err := r.ReadAndValidateJSON[createPost](req)

		require.NoError(t, err)
		assert.Equal(t, &createPost{Title: "hello", Draft: true}, got)
	})

	t.Run("body parses but does not validate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts",
			strings.NewReader(`{"title":"no"}`))

		_, err := r.ReadAndValidateJSON[createPost](req)

		var verr *ValidationError
		require.ErrorAs(t, err, &verr)

		assert.Equal(t, http.StatusUnprocessableEntity, verr.Status())
		require.Contains(t, verr.Problems, "title")

		reason, ok := verr.Problems["title"].(validatorpkg.Reason)
		require.True(t, ok, "problems should carry the validator's reason")
		assert.Equal(t, "min", reason.Rule)
	})

	t.Run("body does not parse", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{`))

		_, err := r.ReadAndValidateJSON[createPost](req)

		var rerr *ReadError
		require.ErrorAs(t, err, &rerr)
		assert.Equal(t, http.StatusBadRequest, rerr.Status())
	})
}

func TestReader_ReadAndValidateForm(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/posts",
		strings.NewReader("title=hello&draft=true"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got, err := newTestReader(t).ReadAndValidateForm[createPost](req)

	require.NoError(t, err)
	assert.Equal(t, &createPost{Title: "hello", Draft: true}, got)
}

func TestReader_ReadAndValidateQueryParams(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/posts?title=hello&draft=true", nil)

	got, err := newTestReader(t).ReadAndValidateQueryParams[createPost](req)

	require.NoError(t, err)
	assert.Equal(t, &createPost{Title: "hello", Draft: true}, got)
}

// One Reader now serves every request type. Previously each type needed its own
// TypedReader constructed at wiring time, because methods could not be generic.
func TestReader_ServesMultipleTypes(t *testing.T) {
	t.Parallel()

	r := newTestReader(t)

	post, err := r.ReadAndValidateJSON[createPost](
		httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{"title":"hello"}`)))
	require.NoError(t, err)
	assert.Equal(t, "hello", post.Title)

	list, err := r.ReadAndValidateQueryParams[listPosts](
		httptest.NewRequest(http.MethodGet, "/posts?page=2", nil))
	require.NoError(t, err)
	assert.Equal(t, 2, list.Page)
}

func TestReader_WithRejectUnknownFields(t *testing.T) {
	t.Parallel()

	v, err := validatorpkg.New()
	require.NoError(t, err)

	r := NewReader(slog.New(slog.DiscardHandler), v, WithRejectUnknownFields())

	req := httptest.NewRequest(http.MethodPost, "/posts",
		strings.NewReader(`{"title":"hello","nope":true}`))

	_, err = r.ReadAndValidateJSON[createPost](req)

	var readErr *ReadError
	require.ErrorAs(t, err, &readErr)

	assert.Equal(t, http.StatusBadRequest, readErr.HTTPStatusCode)
	assert.Equal(t, `Request body contains unknown field "nope"`, readErr.Message)
}

func TestReader_WithMaxBodyBytes(t *testing.T) {
	t.Parallel()

	v, err := validatorpkg.New()
	require.NoError(t, err)

	r := NewReader(slog.New(slog.DiscardHandler), v, WithMaxBodyBytes(16))

	t.Run("JSON body over the cap", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts",
			strings.NewReader(`{"title":"a title well past sixteen bytes"}`))

		_, err := r.ReadAndValidateJSON[createPost](req)

		var readErr *ReadError
		require.ErrorAs(t, err, &readErr)
		assert.Equal(t, http.StatusRequestEntityTooLarge, readErr.HTTPStatusCode)
	})

	t.Run("JSON body within the cap", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{"title":"abc"}`))

		got, err := r.ReadAndValidateJSON[createPost](req)

		require.NoError(t, err)
		assert.Equal(t, "abc", got.Title)
	})

	t.Run("form body over the cap", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts",
			strings.NewReader("title=a+title+well+past+sixteen+bytes"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		_, err := r.ReadAndValidateForm[createPost](req)

		var readErr *ReadError
		require.ErrorAs(t, err, &readErr)
		assert.Equal(t, http.StatusRequestEntityTooLarge, readErr.HTTPStatusCode)
	})

	t.Run("zero leaves the body unbounded", func(t *testing.T) {
		unbounded := NewReader(slog.New(slog.DiscardHandler), v)

		req := httptest.NewRequest(http.MethodPost, "/posts",
			strings.NewReader(`{"title":"a title well past sixteen bytes"}`))

		got, err := unbounded.ReadAndValidateJSON[createPost](req)

		require.NoError(t, err)
		assert.Equal(t, "a title well past sixteen bytes", got.Title)
	})
}

// A type carrying its own Valid method is validated by it, so rules spanning
// several fields need no struct tag.
type dateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

func (d *dateRange) Valid(_ context.Context) map[string]string {
	problems := make(map[string]string)

	if d.Start == "" {
		problems["start"] = "start is required"
	}

	if d.End != "" && d.End < d.Start {
		problems["end"] = "end must not be before start"
	}

	return problems
}

func TestReader_SelfValidatingType(t *testing.T) {
	t.Parallel()

	// No Validator at all: the type validates itself.
	r := NewReader(slog.New(slog.DiscardHandler), nil)

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ranges",
			strings.NewReader(`{"start":"2026-01-01","end":"2026-02-01"}`))

		got, err := r.ReadAndValidateJSON[dateRange](req)

		require.NoError(t, err)
		assert.Equal(t, "2026-02-01", got.End)
	})

	t.Run("cross-field rule fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ranges",
			strings.NewReader(`{"start":"2026-02-01","end":"2026-01-01"}`))

		_, err := r.ReadAndValidateJSON[dateRange](req)

		var verr *ValidationError
		require.ErrorAs(t, err, &verr)

		assert.Equal(t, http.StatusUnprocessableEntity, verr.Status())
		assert.Equal(t, "end must not be before start", verr.Problems["end"])
	})
}
