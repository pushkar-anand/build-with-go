package response

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
