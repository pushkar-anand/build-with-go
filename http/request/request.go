package request

import (
	"context"
	json "encoding/json/v2"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gorilla/schema"
	"github.com/pushkar-anand/build-with-go/logger"
	validatorpkg "github.com/pushkar-anand/build-with-go/validator"
)

type (
	validator interface {
		ValidateStruct(context.Context, any) (*validatorpkg.Result, error)
	}

	// Reader provides functionality to read and validate HTTP request data
	// It contains a logger for error reporting and a validator for request validation
	Reader struct {
		logger      *slog.Logger
		validator   validator
		decoder     *schema.Decoder
		jsonOptions []json.Options
	}

	Option interface {
		apply(*Reader)
	}

	optionFunc func(*Reader)
)

func (fn optionFunc) apply(r *Reader) {
	fn(r)
}

// WithRejectUnknownFields makes a JSON body carrying fields that do not map to
// the target struct fail with a 400, instead of those fields being ignored.
func WithRejectUnknownFields() Option {
	return optionFunc(func(r *Reader) {
		r.jsonOptions = append(r.jsonOptions, json.RejectUnknownMembers(true))
	})
}

// NewReader creates a new Reader instance with the provided logger and validator
func NewReader(
	l *slog.Logger,
	v validator,
	opts ...Option,
) *Reader {
	r := &Reader{
		logger:    l,
		validator: v,
		decoder:   schema.NewDecoder(),
	}

	for _, opt := range opts {
		opt.apply(r)
	}

	return r
}

// ReadAndValidateJSON reads a JSON request body into a T and validates it
// against the struct tags.
//
// It returns a ValidationError carrying the per-field failures when the body
// parses but does not validate, and a ReadError when it cannot be parsed.
func (r *Reader) ReadAndValidateJSON[T any](req *http.Request) (*T, error) {
	body, err := ReadJSONBody[T](req.Body, r.jsonOptions...)
	if err != nil {
		return nil, err
	}

	err = r.validate(req.Context(), body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// ReadAndValidateForm reads form data into a T and validates it against the
// struct tags. Fields are matched by their schema tag, falling back to json.
func (r *Reader) ReadAndValidateForm[T any](req *http.Request) (*T, error) {
	data, err := ReadFormData[T](req, r.decoder)
	if err != nil {
		return nil, err
	}

	err = r.validate(req.Context(), data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// ReadAndValidateQueryParams reads the query string into a T and validates it
// against the struct tags. Fields are matched by their schema tag, falling back
// to json.
func (r *Reader) ReadAndValidateQueryParams[T any](req *http.Request) (*T, error) {
	params, err := ReadQueryParams[T](req.URL.Query(), r.decoder)
	if err != nil {
		return nil, err
	}

	err = r.validate(req.Context(), params)
	if err != nil {
		return nil, err
	}

	return params, nil
}

// ReadJSONBody decodes a JSON input stream into a struct of type T.
//
// The stream must hold exactly one JSON value: anything trailing it is an
// error, rather than being silently ignored. Parsing failures are returned as
// a ReadError.
func ReadJSONBody[T any](r io.Reader, opts ...json.Options) (*T, error) {
	v := new(T)

	err := json.UnmarshalRead(r, v, opts...)
	if err != nil {
		return nil, parseReadError(err)
	}

	return v, nil
}

// ReadQueryParams decodes the query params into a struct of type T.
// It handles any parsing errors and returns them as ReadError.
// Returns a pointer to the populated struct and any error encountered
func ReadQueryParams[T any](q url.Values, d *schema.Decoder) (*T, error) {
	v := new(T)

	err := d.Decode(v, q)
	if err != nil {
		return nil, parseReadError(err)
	}

	return v, nil
}

// ReadFormData decodes the form data into a struct of type T.
// It handles any parsing errors and returns them as ReadError.
// Returns a pointer to the populated struct and any error encountered
func ReadFormData[T any](r *http.Request, d *schema.Decoder) (*T, error) {
	v := new(T)

	err := r.ParseForm()
	if err != nil {
		return nil, parseReadError(err)
	}

	err = d.Decode(v, r.PostForm)
	if err != nil {
		return nil, parseReadError(err)
	}

	return v, nil
}

func (r *Reader) validate(ctx context.Context, v any) error {
	result, err := r.validator.ValidateStruct(ctx, v)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to validate body", logger.Error(err))

		return &ReadError{
			HTTPStatusCode: http.StatusInternalServerError,
			Message:        "Failed to read request due to an internal error, try again",
			UnderlyingErr:  err,
		}
	}

	if !result.Valid {
		return &ValidationError{
			HTTPStatusCode: http.StatusUnprocessableEntity,
			Message:        "Request is not valid",
			UnderlyingErr:  nil,
			Result:         result,
		}
	}

	return nil
}
