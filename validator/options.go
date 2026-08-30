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

// WithCustomTags registers validation rules, keyed by the tag name that
// selects them in a struct tag.
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

// WithCustomMessages overrides the message produced for several tags at once.
func WithCustomMessages(messages map[string]MessageFunc) Option {
	return optionFunc(func(s *Validator) {
		maps.Insert(s.messages, maps.All(messages))
	})
}
