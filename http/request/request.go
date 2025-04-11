package request

import (
	"context"
	"encoding/json"
	"github.com/gorilla/schema"
	"github.com/pushkar-anand/build-with-go/logger"
	validatorpkg "github.com/pushkar-anand/build-with-go/validator"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

type (
	validator interface {
		ValidateStruct(context.Context, any) (*validatorpkg.Result, error)
	}

	// Reader provides functionality to read and validate HTTP request data
	// It contains a logger for error reporting and a validator for request validation
	Reader struct {
		logger    *slog.Logger
		validator validator
		decoder   *schema.Decoder
	}

	// TypedReader is a generic wrapper around Reader that provides type-safe request parsing
	// The type parameter T represents the expected request body structure
	TypedReader[T any] struct {
		*Reader
	}
)

// NewReader creates a new Reader instance with the provided logger and validator
func NewReader(
	l *slog.Logger,
	v validator,
) *Reader {
	return &Reader{
		logger:    l,
		validator: v,
		decoder:   schema.NewDecoder(),
	}
}

// NewTypedReader creates a new TypedReader for a specific type T
// It wraps an existing Reader to provide type-safe request handling
func NewTypedReader[T any](r *Reader) TypedReader[T] {
	return TypedReader[T]{Reader: r}
}

// ReadAndValidateJSON reads a JSON request body and validates it against the struct tags.
// It returns a pointer to the parsed struct of type T and any error that occurred.
// If validation fails, it returns a ValidationError with details about the failure
func (t *TypedReader[T]) ReadAndValidateJSON(r *http.Request) (*T, error) {
	body, err := ReadJSONBody[T](r.Body)
	if err != nil {
		return nil, err
	}

	err = t.validate(r.Context(), body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (t *TypedReader[T]) ReadAndValidateForm(r *http.Request) (*T, error) {
	data, err := ReadFormData[T](r, t.decoder)
	if err != nil {
		return nil, err
	}

	err = t.validate(r.Context(), data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (t *TypedReader[T]) ReadAndValidateQueryParams(r *http.Request) (*T, error) {
	params, err := ReadQueryParams[T](r.URL.Query(), t.decoder)
	if err != nil {
		return nil, err
	}

	err = t.validate(r.Context(), params)
	if err != nil {
		return nil, err
	}

	return params, nil
}

// ReadJSONBody decodes a JSON input stream into a struct of type T.
// It handles any parsing errors and returns them as ReadError.
// Returns a pointer to the populated struct and any error encountered
func ReadJSONBody[T any](r io.Reader) (*T, error) {
	v := new(T)

	err := json.NewDecoder(r).Decode(v)
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
		r.logger.Error("failed to validate body", logger.Error(err))

		return &ReadError{
			HTTPStatusCode: http.StatusInternalServerError,
			Message:        "Failed to read request due to an internal error, try again",
			UnderlyingErr:  err,
		}
	}

	if !result.Valid {
		return &ValidationError{
			ReadError: ReadError{
				HTTPStatusCode: http.StatusUnprocessableEntity,
				Message:        "Request is not valid",
				UnderlyingErr:  nil,
			},
			Result: result,
		}
	}

	return nil
}
