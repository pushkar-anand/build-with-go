package request

import (
	"encoding/json"
	"errors"
	"fmt"
	validatorpkg "github.com/pushkar-anand/build-with-go/validator"
	"io"
	"net/http"
	"strings"
)

type (
	// ReadError represents an error that occurs when reading or parsing a request
	// It provides HTTP status code, user-friendly message, and the underlying error
	ReadError struct {
		HTTPStatusCode int
		Message        string
		UnderlyingErr  error
	}

	// ValidationError extends ReadError to include validation results
	// It is used when request data fails validation checks
	ValidationError struct {
		ReadError
		Result *validatorpkg.Result
	}
)

// Error returns the error message for ReadError
func (e *ReadError) Error() string { return e.Message }

// Unwrap returns the underlying error for ReadError
func (e *ReadError) Unwrap() error { return e.UnderlyingErr }

// Error returns the error message for ValidationError
func (e *ValidationError) Error() string { return e.Message }

// parseReadError analyzes JSON parsing errors and returns appropriate ReadError
// with user-friendly messages and suitable HTTP status codes
func parseReadError(err error) *ReadError {
	var (
		syntaxError        *json.SyntaxError
		unmarshalTypeError *json.UnmarshalTypeError
	)

	switch {
	// Catch any syntax errors in the JSON and send an error Message
	// which interpolates the location of the problem to make it
	// easier for the client to fix.
	// In some circumstances Decode() may also return an
	// io.ErrUnexpectedEOF error for syntax errors in the JSON. There
	// is an open issue regarding this at
	// https://github.com/golang/go/issues/25956.
	case errors.As(err, &syntaxError):
		return &ReadError{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        fmt.Sprintf("Request body contains badly-formed JSON at offset %d", syntaxError.Offset),
			UnderlyingErr:  err,
		}
	case errors.Is(err, io.ErrUnexpectedEOF):
		return &ReadError{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        fmt.Sprintf("Request body contains badly-formed JSON"),
			UnderlyingErr:  err,
		}

	// Catch any type errors, like trying to assign a string in the
	// JSON request body to an int field in our Person struct. We can
	// interpolate the relevant field name and position into the error
	// Message to make it easier for the client to fix.
	case errors.As(err, &unmarshalTypeError):
		return &ReadError{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        fmt.Sprintf("Request body contains an invalid value for the %q field, expecting: %q", unmarshalTypeError.Field, unmarshalTypeError.Type.Name()),
			UnderlyingErr:  err,
		}

	// Catch the error caused by extra unexpected fields in the request
	// body. We extract the field name from the error Message and
	// interpolate it in our custom error Message. There is an open
	// issue at https://github.com/golang/go/issues/29035 regarding
	// turning this into a sentinel error.
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")

		return &ReadError{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        fmt.Sprintf("Request body contains unknown field %s", fieldName),
			UnderlyingErr:  err,
		}

	// An io.EOF error is returned by Decode() if the request body is
	// empty.
	case errors.Is(err, io.EOF):
		return &ReadError{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        "Request body must not be empty",
			UnderlyingErr:  err,
		}

	// Catch the error caused by the request body being too large. Again,
	// there is an open issue regarding turning this into a sentinel
	// error at https://github.com/golang/go/issues/30715.
	case err.Error() == "http: request body too large":
		return &ReadError{
			HTTPStatusCode: http.StatusRequestEntityTooLarge,
			Message:        "Request body must not be larger than 1MB",
			UnderlyingErr:  err,
		}

	default:
		return &ReadError{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        "Failed to parse request body",
			UnderlyingErr:  err,
		}
	}
}
