package response

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pushkar-anand/build-with-go/logger"
)

// NewHTMLWriter returns an HTMLWriter.
func NewHTMLWriter(
	l *slog.Logger,
	tmpl *template.Template,
) *HTMLWriter {
	if l == nil {
		l = slog.Default()
	}

	l = l.With(
		slog.String("writer", "HTMLWriter"),
	)

	return &HTMLWriter{
		logger:    l,
		templates: tmpl,
	}
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

// NotFound renders the given template with the given data with a 404 status.
func (hw *HTMLWriter) NotFound(w http.ResponseWriter, r *http.Request, templateName string, templateData any) {
	hw.render(w, r, http.StatusNotFound, templateName, templateData)
}

// InternalServerError renders the given template with the given data with a 500 status.
func (hw *HTMLWriter) InternalServerError(w http.ResponseWriter, r *http.Request, templateName string, templateData any) {
	hw.render(w, r, http.StatusInternalServerError, templateName, templateData)
}

// BadRequest renders the given template with the given data with a 400 status.
func (hw *HTMLWriter) BadRequest(w http.ResponseWriter, r *http.Request, templateName string, templateData any) {
	hw.render(w, r, http.StatusBadRequest, templateName, templateData)
}

// Unauthorized renders the given template with the given data with a 401 status.
func (hw *HTMLWriter) Unauthorized(w http.ResponseWriter, r *http.Request, templateName string, templateData any) {
	hw.render(w, r, http.StatusUnauthorized, templateName, templateData)
}

// Forbidden renders the given template with the given data with a 403 status.
func (hw *HTMLWriter) Forbidden(w http.ResponseWriter, r *http.Request, templateName string, templateData any) {
	hw.render(w, r, http.StatusForbidden, templateName, templateData)
}

// Error renders the given template with the given data with the given status.
func (hw *HTMLWriter) Error(w http.ResponseWriter, r *http.Request, status int, templateName string, templateData any) {
	hw.render(w, r, status, templateName, templateData)
}

// render renders the given template with the given data with the given status.
func (hw *HTMLWriter) render(w http.ResponseWriter, r *http.Request, status int, templateName string, templateData any) {
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
