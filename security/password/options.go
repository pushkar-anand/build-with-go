package password

type (
	// Option configures a Hasher.
	Option interface {
		apply(*Hasher)
	}

	optionFunc func(*Hasher)
)

func (fn optionFunc) apply(h *Hasher) {
	fn(h)
}

// WithPepper sets a secret value appended to the salt before hashing. Unlike
// the salt, the pepper is not stored alongside the hash, so it must be
// supplied again on every Hash and Compare call.
func WithPepper(pepper string) Option {
	return optionFunc(func(h *Hasher) {
		h.pepper = pepper
	})
}

// WithTime sets the argon2id time (iteration) parameter.
func WithTime(time uint32) Option {
	return optionFunc(func(h *Hasher) {
		h.time = time
	})
}

// WithMemory sets the argon2id memory parameter, in KiB.
func WithMemory(memory uint32) Option {
	return optionFunc(func(h *Hasher) {
		h.memory = memory
	})
}

// WithParallelism sets the argon2id parallelism (thread count) parameter.
func WithParallelism(parallelism uint8) Option {
	return optionFunc(func(h *Hasher) {
		h.parallelism = parallelism
	})
}

// WithSaltLength sets the length, in bytes, of the random salt generated for
// each hash.
func WithSaltLength(length uint32) Option {
	return optionFunc(func(h *Hasher) {
		h.saltLength = length
	})
}

// WithKeyLength sets the length, in bytes, of the derived key.
func WithKeyLength(length uint32) Option {
	return optionFunc(func(h *Hasher) {
		h.keyLength = length
	})
}
