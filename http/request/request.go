package request

import (
	"encoding/json"
	"github.com/pushkar-anand/build-with-go/logger"
	"github.com/pushkar-anand/build-with-go/validator"

	"io"
	"log/slog"
	"net/http"
)

type (
	// Reader provides functionality to read and validate HTTP request data
	// It contains a logger for error reporting and a validator for request validation
	Reader struct {
		logger    *slog.Logger
		validator *validator.Validator
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
	v *validator.Validator,
) *Reader {
	return &Reader{
		logger:    l,
		validator: v,
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

	result, err := t.validator.ValidateStruct(r.Context(), body)
	if err != nil {
		t.logger.Error("failed to validate JSON body", logger.Error(err))

		return nil, &ReadError{
			HTTPStatusCode: http.StatusInternalServerError,
			Message:        "Failed to read request due to an internal error, try again",
			UnderlyingErr:  err,
		}
	}

	if !result.Valid {
		return nil, &ValidationError{
			ReadError: ReadError{
				HTTPStatusCode: http.StatusUnprocessableEntity,
				Message:        "Request is not valid",
				UnderlyingErr:  nil,
			},
			Result: result,
		}
	}

	return body, nil
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
