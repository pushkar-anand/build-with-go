// Package response writes JSON responses, and renders errors as RFC 9457
// problem documents.
//
// Handlers report failure by returning an error rather than writing one. Any
// error implementing Problem describes its own response; anything else goes
// through the mapper given to WithErrorProblemMapper, and then to a generic 500.
package response

import (
	"html/template"
	"log/slog"
	"net/http"
)

// HandlerFunc is a handler that reports failure by returning an error, leaving
// the response to ToStandardHandler.
//
// It takes no context parameter on purpose. The request carries the context, so
// a handler that derives one must attach it with r.WithContext for anything it
// passes r to — a Reader, say — to observe it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

type (
	JSONWriter struct {
		logger           *slog.Logger
		errProblemMapper func(error) Problem
	}

	HTMLWriter struct {
		logger          *slog.Logger
		templates       *template.Template
		errorTemplates  map[int]string
		errStatusMapper func(error) int
	}
)
