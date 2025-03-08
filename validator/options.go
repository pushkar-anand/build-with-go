package validator

import "maps"

type (
	Option interface {
		apply(*Validator)
	}

	optionFunc func(*Validator)
)

func (f optionFunc) apply(v *Validator) {
	f(v)
}

func WithCustomTags(rules map[string]ValidationFunc) Option {
	return optionFunc(func(s *Validator) {
		maps.Insert(s.rules, maps.All(rules))
	})
}

// WithCustomMessage allows setting a single custom error message for a specific validation tag
func WithCustomMessage(tag string, messageFn MessageFunc) Option {
	return optionFunc(func(s *Validator) {
		s.messages[tag] = messageFn
	})
}
