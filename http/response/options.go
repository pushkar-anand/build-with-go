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

func WithErrorProblemMapper(fn func(err error) Problem) Option {
	return optionFunc(func(h *JSONWriter) {
		h.errProblemMapper = fn
	})
}
