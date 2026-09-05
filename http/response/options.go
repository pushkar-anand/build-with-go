package response

type (
	Option interface {
		apply(*JSONWriter)
	}

	optionFunc func(*JSONWriter)
)

func (fn optionFunc) apply(h *JSONWriter) {
	fn(h)
}

// WithErrorProblemMapper overrides the default response for any error, including
// errors implementing Problem. Returning nil uses the error's Problem if present,
// otherwise a generic 500.
func WithErrorProblemMapper(fn func(err error) Problem) Option {
	return optionFunc(func(h *JSONWriter) {
		h.errProblemMapper = fn
	})
}
