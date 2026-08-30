package request_test

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/pushkar-anand/build-with-go/http/request"
	"github.com/pushkar-anand/build-with-go/validator"
)

type createPost struct {
	Title string `json:"title" validate:"required,min=3"`
	Body  string `json:"body"  validate:"required"`
}

// Struct tags cover most validation. The type parameter goes at the call site,
// so one Reader serves every request type.
func ExampleReader_ReadAndValidateJSON() {
	v, err := validator.New()
	if err != nil {
		return
	}

	reader := request.NewReader(slog.Default(), v)

	_ = func(w http.ResponseWriter, r *http.Request) error {
		post, err := reader.ReadAndValidateJSON[createPost](r)
		if err != nil {
			// A ReadError (400) or ValidationError (422). Both implement
			// response.Problem, so returning it is enough.
			return err
		}

		_ = post

		return nil
	}
}

// Query parameters and form bodies bind by schema tag, falling back to json.
func ExampleReader_ReadAndValidateQueryParams() {
	type listPosts struct {
		Page    int    `schema:"page"    validate:"min=1"`
		Author  string `schema:"author"`
		OrderBy string `schema:"order_by" validate:"omitempty,oneof=title created_at"`
	}

	v, _ := validator.New()
	reader := request.NewReader(slog.Default(), v)

	_ = func(w http.ResponseWriter, r *http.Request) error {
		params, err := reader.ReadAndValidateQueryParams[listPosts](r)
		if err != nil {
			return err
		}

		_ = params

		return nil
	}
}

type dateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Valid makes dateRange a request.SelfValidator. Rules spanning several fields
// are plain Go here, rather than something to encode in a struct tag.
func (d *dateRange) Valid(_ context.Context) map[string]string {
	problems := make(map[string]string)

	if d.Start.IsZero() {
		problems["start"] = "start is required"
	}

	if !d.End.IsZero() && d.End.Before(d.Start) {
		problems["end"] = "end must not be before start"
	}

	return problems
}

// A type that validates itself needs no Validator at all, so nil is fine.
func ExampleSelfValidator() {
	reader := request.NewReader(slog.Default(), nil)

	_ = func(w http.ResponseWriter, r *http.Request) error {
		window, err := reader.ReadAndValidateJSON[dateRange](r)
		if err != nil {
			return err
		}

		_ = window

		return nil
	}
}

// Unknown fields are ignored by default. Opt in to rejecting them when a
// misspelled field should be an error rather than silently dropped.
func ExampleWithRejectUnknownFields() {
	v, _ := validator.New()

	_ = request.NewReader(slog.Default(), v, request.WithRejectUnknownFields())
}
