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

// WithErrorProblemMapper sets how errors that do not implement Problem are
// turned into one. Errors that do implement it describe themselves and never
// reach the mapper. Returning nil falls back to a generic 500.
func WithErrorProblemMapper(fn func(err error) Problem) Option {
	return optionFunc(func(h *JSONWriter) {
		h.errProblemMapper = fn
	})
}
