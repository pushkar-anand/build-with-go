package response

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/pushkar-anand/build-with-go/http/request"
	"github.com/pushkar-anand/build-with-go/logger"
	"log/slog"
	"net/http"
)

type (
	JSONWriter struct {
		logger           *slog.Logger
		errProblemMapper func(error) Problem
	}
)

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

func (h *JSONWriter) Ok(ctx context.Context, w http.ResponseWriter, v any) {
	h.writeJSON(ctx, w, http.StatusOK, v)
}

func (h *JSONWriter) Write(ctx context.Context, w http.ResponseWriter, statusCode int, v any) {
	h.writeJSON(ctx, w, statusCode, v)
}

func (h *JSONWriter) WriteError(ctx context.Context, r *http.Request, w http.ResponseWriter, err error) {
	problem := h.getMappedProblem(err)
	body := buildProblemJSON(r, problem)

	h.writeJSON(ctx, w, problem.Status(), body)
}

func (h *JSONWriter) WriteProblem(ctx context.Context, r *http.Request, w http.ResponseWriter, p Problem) {
	body := buildProblemJSON(r, p)
	h.writeJSON(ctx, w, p.Status(), body)
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

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to encode response", logger.Error(err))
	}
}

func (h *JSONWriter) getMappedProblem(err error) Problem {
	var (
		readErr       *request.ReadError
		validationErr *request.ValidationError
	)

	if errors.As(err, &readErr) {
		return readErr
	}

	if errors.As(err, &validationErr) {
		return validationErr
	}

	if h.errProblemMapper == nil {
		h.logger.ErrorContext(context.Background(), "failed to handle request", logger.Error(err))
		return &defaultProblem{}
	}

	p := h.errProblemMapper(err)
	if p == nil {
		return &defaultProblem{}
	}

	return p
}
