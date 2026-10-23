// Package secret wraps sensitive values so they do not leak through logs,
// formatted output, or encoded responses by accident.
//
// A Secret prints, logs, and encodes as "[redacted]". The only way to read the
// value back is Reveal, which keeps every use of it easy to find.
package secret

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
)

// Redacted is what a Secret shows in place of its value.
const Redacted = "[redacted]"

// Secret holds a value of type T that must not be printed or encoded.
//
// The zero value holds the zero value of T. A Secret is built only with New;
// nothing decodes into one.
//
// Every method uses a value receiver: fmt, slog, and encoding/json skip
// pointer methods on values they cannot address, such as a Secret passed by
// value or held in a map, and would then print or encode the struct itself.
type Secret[T any] struct {
	// value is held behind a pointer so a Secret nested in an unexported
	// struct field, which fmt cannot call methods on, prints as an address
	// rather than as the value itself.
	value *T

	// Comparing secrets would compare the pointers, not the values.
	_ [0]func()
}

// New wraps v in a Secret.
func New[T any](v T) Secret[T] {
	return Secret[T]{value: &v}
}

// Reveal returns the wrapped value.
func (s Secret[T]) Reveal() T {
	if s.value == nil {
		var zero T
		return zero
	}

	return *s.value
}

// String returns Redacted.
func (Secret[T]) String() string { return Redacted }

// Format writes Redacted for every verb, including %#v, %d, and %x.
func (Secret[T]) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, Redacted) }

// LogValue returns Redacted as a slog value.
func (Secret[T]) LogValue() slog.Value { return slog.StringValue(Redacted) }

// MarshalJSON encodes Redacted as a JSON string.
func (Secret[T]) MarshalJSON() ([]byte, error) { return json.Marshal(Redacted) }

// MarshalText returns Redacted. It covers text-based encoders such as XML and
// YAML.
func (Secret[T]) MarshalText() ([]byte, error) { return []byte(Redacted), nil }

var (
	_ fmt.Stringer   = Secret[string]{}
	_ fmt.Formatter  = Secret[string]{}
	_ slog.LogValuer = Secret[string]{}
	_ json.Marshaler = Secret[string]{}
)
