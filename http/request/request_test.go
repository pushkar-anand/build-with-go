package request

import (
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
		require.Contains(t, verr.Result.Failed, "title")
		assert.Equal(t, "min", verr.Result.Failed["title"].Rule)
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
