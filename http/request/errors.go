package request

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
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

	// ValidationError extends ReadError with the reason each field was rejected.
	// It is used when request data parses but fails validation.
	ValidationError struct {
		ReadError
		Problems map[string]any
	}
)

// Error returns the error message for ReadError
func (e *ReadError) Error() string { return e.Message }

// Unwrap returns the underlying error for ReadError
func (e *ReadError) Unwrap() error { return e.UnderlyingErr }

func (e *ReadError) Type() string {
	return "about:blank"
}

func (e *ReadError) Title() string {
	return http.StatusText(e.HTTPStatusCode)
}

func (e *ReadError) Status() int {
	return e.HTTPStatusCode
}

func (e *ReadError) Detail() string {
	return e.Message
}

func (e *ReadError) CustomMembers() map[string]any {
	return nil
}

// newValidationError builds the error for a failed validation, or returns nil
// when there is nothing to report.
func newValidationError(problems map[string]any) error {
	if len(problems) == 0 {
		return nil
	}

	return &ValidationError{
		HTTPStatusCode: http.StatusUnprocessableEntity,
		Message:        "Request is not valid",
		Problems:       problems,
	}
}

// Error returns the error message for ValidationError
func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) CustomMembers() map[string]any {
	return map[string]any{
		"context": e.Problems,
	}
}

// parseReadError turns a decoding failure into a ReadError carrying a status
// code and a message that is safe, and useful, to show the client.
func parseReadError(err error) *ReadError {
	var (
		maxBytesErr  *http.MaxBytesError
		semanticErr  *json.SemanticError
		syntacticErr *jsontext.SyntacticError
	)

	switch {
	// The body exceeded the limit set by http.MaxBytesReader.
	case errors.As(err, &maxBytesErr):
		return &ReadError{
			HTTPStatusCode: http.StatusRequestEntityTooLarge,
			Message:        fmt.Sprintf("Request body must not be larger than %d bytes", maxBytesErr.Limit),
			UnderlyingErr:  err,
		}

	// The JSON is well formed but does not fit the target type.
	case errors.As(err, &semanticErr):
		if errors.Is(err, json.ErrUnknownName) {
			return &ReadError{
				HTTPStatusCode: http.StatusBadRequest,
				Message:        fmt.Sprintf("Request body contains unknown field %q", fieldName(semanticErr.JSONPointer)),
				UnderlyingErr:  err,
			}
		}

		return &ReadError{
			HTTPStatusCode: http.StatusBadRequest,
			Message: fmt.Sprintf("Request body contains an invalid value for the %q field, expecting: %s",
				fieldName(semanticErr.JSONPointer), goTypeName(semanticErr.GoType)),
			UnderlyingErr: err,
		}

	// The bytes are not valid JSON.
	case errors.As(err, &syntacticErr):
		switch {
		case errors.Is(err, jsontext.ErrDuplicateName):
			return &ReadError{
				HTTPStatusCode: http.StatusBadRequest,
				Message:        fmt.Sprintf("Request body contains the duplicate field %q", fieldName(syntacticErr.JSONPointer)),
				UnderlyingErr:  err,
			}

		// Nothing was read at all, as opposed to truncation part way through.
		case errors.Is(err, io.ErrUnexpectedEOF) && syntacticErr.ByteOffset == 0:
			return &ReadError{
				HTTPStatusCode: http.StatusBadRequest,
				Message:        "Request body must not be empty",
				UnderlyingErr:  err,
			}
		}

		return &ReadError{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        fmt.Sprintf("Request body contains badly-formed JSON at offset %d", syntacticErr.ByteOffset),
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

// fieldName renders a JSON pointer as a field name for a client-facing message.
func fieldName(p jsontext.Pointer) string {
	return strings.TrimPrefix(string(p), "/")
}

// goTypeName names the type a value could not be decoded into.
func goTypeName(t reflect.Type) string {
	if t == nil {
		return "a different type"
	}

	return t.String()
}
