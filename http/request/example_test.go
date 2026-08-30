package request_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/pushkar-anand/build-with-go/http/request"
	"github.com/pushkar-anand/build-with-go/validator"
)

type createPost struct {
	Title string `json:"title" validate:"required,min=3"`
	Body  string `json:"body"  validate:"required"`
}

func newReader(opts ...request.Option) *request.Reader {
	v, err := validator.New()
	if err != nil {
		log.Fatal(err)
	}

	return request.NewReader(slog.New(slog.DiscardHandler), v, opts...)
}

// The type parameter goes at the call site, so one Reader serves every request
// type.
func ExampleReader_ReadAndValidateJSON() {
	r := httptest.NewRequest(http.MethodPost, "/posts",
		strings.NewReader(`{"title":"Hello","body":"World"}`))

	post, err := newReader().ReadAndValidateJSON[createPost](r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(post.Title, "/", post.Body)

	// Output: Hello / World
}

// A body that parses but breaks a rule comes back as a ValidationError, which
// carries the reason per field.
func ExampleReader_ReadAndValidateJSON_invalid() {
	r := httptest.NewRequest(http.MethodPost, "/posts",
		strings.NewReader(`{"title":"no","body":"World"}`))

	_, err := newReader().ReadAndValidateJSON[createPost](r)

	var invalid *request.ValidationError
	if errors.As(err, &invalid) {
		reason := invalid.Problems["title"].(validator.Reason)
		fmt.Println("status:", invalid.Status())
		fmt.Println("rule:  ", reason.Rule)
	}

	// Output:
	// status: 422
	// rule:   min
}

// Query parameters bind by schema tag.
func ExampleReader_ReadAndValidateQueryParams() {
	type listPosts struct {
		Page    int    `schema:"page"     validate:"min=1"`
		OrderBy string `schema:"order_by" validate:"omitempty,oneof=title created_at"`
	}

	r := httptest.NewRequest(http.MethodGet, "/posts?page=2&order_by=title", nil)

	params, err := newReader().ReadAndValidateQueryParams[listPosts](r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(params.Page, params.OrderBy)

	// Output: 2 title
}

type dateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Valid makes dateRange a request.SelfValidator. A rule spanning two fields is
// plain Go here, rather than something to encode in a struct tag.
func (d *dateRange) Valid(_ context.Context) map[string]string {
	problems := make(map[string]string)

	if !d.End.IsZero() && d.End.Before(d.Start) {
		problems["end"] = "end must not be before start"
	}

	return problems
}

// A type that validates itself needs no Validator, so nil is fine.
func ExampleSelfValidator() {
	reader := request.NewReader(slog.New(slog.DiscardHandler), nil)

	r := httptest.NewRequest(http.MethodPost, "/ranges",
		strings.NewReader(`{"start":"2026-02-01T00:00:00Z","end":"2026-01-01T00:00:00Z"}`))

	_, err := reader.ReadAndValidateJSON[dateRange](r)

	var invalid *request.ValidationError
	if errors.As(err, &invalid) {
		fmt.Println(invalid.Problems["end"])
	}

	// Output: end must not be before start
}

// Unknown fields are ignored by default. Opt in when a misspelled field should
// be an error rather than silently dropped.
func ExampleWithRejectUnknownFields() {
	r := httptest.NewRequest(http.MethodPost, "/posts",
		strings.NewReader(`{"title":"Hello","body":"World","tpyo":true}`))

	_, err := newReader(request.WithRejectUnknownFields()).ReadAndValidateJSON[createPost](r)

	fmt.Println(err)

	// Output: Request body contains unknown field "tpyo"
}
