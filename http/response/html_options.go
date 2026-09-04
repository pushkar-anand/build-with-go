package response

import "net/http"

type (
	// HTMLOption configures an HTMLWriter.
	HTMLOption interface {
		applyHTML(*HTMLWriter)
	}

	htmlOptionFunc func(*HTMLWriter)
)

func (fn htmlOptionFunc) applyHTML(hw *HTMLWriter) { fn(hw) }

// WithErrorTemplates sets the templates used to render error pages, keyed by
// HTTP status code. The zero key names the fallback template for any status
// without its own entry. Without this option the writer falls back to a plain
// http.Error.
func WithErrorTemplates(byStatus map[int]string) HTMLOption {
	return htmlOptionFunc(func(hw *HTMLWriter) {
		hw.errorTemplates = byStatus
	})
}

// WithErrorStatusMapper sets how errors that do not implement Problem are
// mapped to a status code, and so to an error page. Returning zero falls back
// to 500. Errors implementing Problem carry their own status and never reach
// the mapper.
func WithErrorStatusMapper(fn func(err error) int) HTMLOption {
	return htmlOptionFunc(func(hw *HTMLWriter) {
		hw.errStatusMapper = fn
	})
}

// WithErrorDataFunc sets extra fields to pass to an error-page template
// alongside Status and Title. fn receives the request, the originating error
// (nil when the page was rendered via ErrorPage rather than WriteError), and
// the status, and returns the fields to add, keyed by name:
//
//	WithErrorDataFunc(func(r *http.Request, err error, status int) map[string]any {
//		return map[string]any{"SupportEmail": "help@example.com"}
//	})
//
// The writer merges these into the template data alongside "Status" and
// "Title", so the template reads {{.Status}}, {{.Title}}, and
// {{.SupportEmail}} directly — the caller never builds or embeds
// ErrorPageData. A key of "Status" or "Title" overrides the default value.
// Without this option the template gets ErrorPageData with no extra fields.
func WithErrorDataFunc(fn func(r *http.Request, err error, status int) map[string]any) HTMLOption {
	return htmlOptionFunc(func(hw *HTMLWriter) {
		hw.errDataFunc = fn
	})
}
