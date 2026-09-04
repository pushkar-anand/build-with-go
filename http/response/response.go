// Package response writes JSON responses, and renders errors as RFC 9457
// problem documents.
//
// Handlers report failure by returning an error rather than writing one. Any
// error implementing Problem describes its own response; anything else goes
// through the mapper given to WithErrorProblemMapper, and then to a generic 500.
package response

import (
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"

	"github.com/pushkar-anand/build-with-go/logger"
)

type (
	JSONWriter struct {
		logger           *slog.Logger
		errProblemMapper func(error) Problem
	}
)

// NewJSONWriter returns a JSONWriter that logs encoding failures to l.
func NewJSONWriter(
	l *slog.Logger,
	opts ...Option,
) *JSONWriter {
	jw := &JSONWriter{
		logger: l,
	}

	for _, opt := range opts {
		opt.apply(jw)
	}

	return jw
}

// Ok writes v as a 200 response.
func (h *JSONWriter) Ok(w http.ResponseWriter, r *http.Request, v any) {
	h.writeJSON(r.Context(), w, http.StatusOK, v)
}

// Write writes v with the given status code.
func (h *JSONWriter) Write(w http.ResponseWriter, r *http.Request, statusCode int, v any) {
	h.writeJSON(r.Context(), w, statusCode, v)
}

// WriteError resolves err to a Problem and writes it as a problem document.
func (h *JSONWriter) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()

	problem := h.getMappedProblem(ctx, err)
	body := buildProblemJSON(r, problem)

	h.writeJSON(ctx, w, problem.Status(), body)
}

// WriteProblem writes p as a problem document.
func (h *JSONWriter) WriteProblem(w http.ResponseWriter, r *http.Request, p Problem) {
	body := buildProblemJSON(r, p)
	h.writeJSON(r.Context(), w, p.Status(), body)
}

func (h *JSONWriter) writeJSON(
	ctx context.Context,
	w http.ResponseWriter,
	statusCode int,
	v any,
) {
	contentType := "application/json; charset=utf-8"
	if statusCode >= http.StatusBadRequest {
		contentType = "application/problem+json; charset=utf-8"
		w.Header().Set("Cache-Control", "no-store")
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)

	if v == nil {
		return
	}

	// Deterministic sorts map keys. Problem documents are built as maps, and
	// json/v2 does not order them otherwise, so without this the same error
	// renders with a different key order each time.
	err := json.MarshalWrite(w, v, json.Deterministic(true))
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to encode response", logger.Err(err))
	}
}

// HandlerFunc is a handler that reports failure by returning an error, leaving
// the response to ToStandardHandler.
//
// It takes no context parameter on purpose. The request carries the context, so
// a handler that derives one must attach it with r.WithContext for anything it
// passes r to — a Reader, say — to observe it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Handle converts a HandlerFunc to a standard http.HandlerFunc, turning a
// returned error into a problem document.
func (h *JSONWriter) Handle(handler HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			h.WriteError(w, r, err)
		}
	}
}

func (h *JSONWriter) HandleJSON[T any](fn func(r *http.Request) (T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)

		if err != nil {
			h.WriteError(w, r, err)
			return
		}

		h.Ok(w, r, v)
	}
}

// getMappedProblem resolves err to the Problem describing the response.
//
// An error that implements Problem already describes itself, so it is used as
// it is. That covers request.ReadError and request.ValidationError without this
// package having to know those types, and lets callers give their own domain
// errors a response without configuring a mapper at all.
//
// Anything else falls to the configured mapper, then to a generic 500.
func (h *JSONWriter) getMappedProblem(ctx context.Context, err error) Problem {
	var problem Problem
	if errors.As(err, &problem) {
		return problem
	}

	if h.errProblemMapper == nil {
		h.logger.ErrorContext(ctx, "failed to handle request", logger.Err(err))
		return defaultProblem
	}

	p := h.errProblemMapper(err)
	if p == nil {
		return defaultProblem
	}

	return p
}
