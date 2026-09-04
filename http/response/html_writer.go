package response

import (
	"bytes"
	"errors"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pushkar-anand/build-with-go/logger"
)

// NewHTMLWriter returns an HTMLWriter.
func NewHTMLWriter(
	l *slog.Logger,
	tmpl *template.Template,
	opts ...HTMLOption,
) *HTMLWriter {
	if l == nil {
		l = slog.Default()
	}

	l = l.With(
		slog.String("writer", "HTMLWriter"),
	)

	hw := &HTMLWriter{
		logger:    l,
		templates: tmpl,
	}

	for _, opt := range opts {
		opt.applyHTML(hw)
	}

	return hw
}

// WithTemplates sets the templates to be used by the writer.
// It will overwrite any previously set templates.
func (hw *HTMLWriter) WithTemplates(tmpl *template.Template) *HTMLWriter {
	hw.templates = tmpl
	return hw
}

// HTML returns a [http.HandlerFunc] that renders the given template with the given data.
// It is a shortcut for Success.
func (hw *HTMLWriter) HTML(tmpl string, data any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hw.Success(w, r, tmpl, data)
	}
}

// Success renders the given template with the given data with a 200 status.
func (hw *HTMLWriter) Success(w http.ResponseWriter, r *http.Request, templateName string, templateData any) {
	hw.render(w, r, http.StatusOK, templateName, templateData)
}

// Error renders the given template with the given data with the given status.
func (hw *HTMLWriter) Error(w http.ResponseWriter, r *http.Request, status int, templateName string, templateData any) {
	hw.render(w, r, status, templateName, templateData)
}

// ErrorPageData is the data passed to an error-page template. It carries only
// what is safe to show a visitor: the status code and its text. Problem
// details, error messages, and instance URIs stay in the logs and in JSON
// responses; they are not part of an HTML error page.
type ErrorPageData struct {
	Status int
	Title  string
}

// Handle adapts a HandlerFunc to an http.HandlerFunc, rendering the error page
// for any error the handler returns.
func (hw *HTMLWriter) Handle(handler HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			hw.logger.ErrorContext(
				r.Context(), "handler failed",
				logger.Err(err),
			)

			hw.WriteError(w, r, err)
		}
	}
}

// WriteError renders the error page for err. An error implementing Problem
// selects the page by its Status; an error handled by WithErrorStatusMapper
// uses the status it returns; anything else renders the 500 page.
func (hw *HTMLWriter) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	hw.renderError(w, r, err, hw.statusForError(err))
}

// ErrorPage renders the configured error page for status.
func (hw *HTMLWriter) ErrorPage(w http.ResponseWriter, r *http.Request, status int) {
	hw.renderError(w, r, nil, status)
}

func (hw *HTMLWriter) statusForError(err error) int {
	if problem, ok := errors.AsType[Problem](err); ok {
		return problem.Status()
	}

	if hw.errStatusMapper != nil {
		if status := hw.errStatusMapper(err); status != 0 {
			return status
		}
	}

	return http.StatusInternalServerError
}

func (hw *HTMLWriter) renderError(w http.ResponseWriter, r *http.Request, err error, status int) {
	tmpl, present := hw.errorTemplates[status]
	if !present {
		hw.logger.ErrorContext(
			r.Context(),
			"no error template for status",
			slog.Int("status", status),
		)

		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	if hw.errDataFunc == nil {
		hw.render(w, r, status, tmpl, ErrorPageData{
			Status: status,
			Title:  http.StatusText(status),
		})
		return
	}

	data := map[string]any{
		"Status": status,
		"Title":  http.StatusText(status),
	}

	for k, v := range hw.errDataFunc(r, err, status) {
		data[k] = v
	}

	hw.render(w, r, status, tmpl, data)
}

// render renders the given template with the given data with the given status.
func (hw *HTMLWriter) render(w http.ResponseWriter, r *http.Request, status int, templateName string, templateData any) {
	if hw.templates == nil {
		hw.logger.ErrorContext(
			r.Context(),
			"no templates configured",
		)

		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Rendered to a buffer first, so a template that fails halfway through
	// cannot leave half a page followed by an error, and so the status is not
	// yet written when it does.
	var buf bytes.Buffer

	err := hw.templates.ExecuteTemplate(&buf, templateName, templateData)
	if err != nil {
		hw.logger.ErrorContext(
			r.Context(),
			"error rendering template",
			logger.Err(err),
			slog.String("template", templateName),
		)

		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	_, err = buf.WriteTo(w)
	if err != nil {
		hw.logger.ErrorContext(
			r.Context(),
			"error writing html to client",
			logger.Err(err),
			slog.String("template", templateName),
		)
	}
}
