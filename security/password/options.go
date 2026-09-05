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

// WithTime sets the argon2id time (iteration) parameter. 0 means default.
func WithTime(time uint32) Option {
	return optionFunc(func(h *Hasher) {
		if time == 0 {
			time = defaultTime
		}

		h.time = time
	})
}

// WithMemory sets the argon2id memory parameter, in KiB. 0 means default.
func WithMemory(memory uint32) Option {
	return optionFunc(func(h *Hasher) {
		if memory == 0 {
			memory = defaultMemory
		}

		h.memory = memory
	})
}

// WithParallelism sets the argon2id parallelism (thread count) parameter. 0 means default.
func WithParallelism(parallelism uint8) Option {
	return optionFunc(func(h *Hasher) {
		if parallelism == 0 {
			parallelism = defaultParallelism
		}

		h.parallelism = parallelism
	})
}

// WithSaltLength sets the length, in bytes, of the random salt generated for
// each hash. 0 means default.
func WithSaltLength(length uint32) Option {
	return optionFunc(func(h *Hasher) {
		if length == 0 {
			length = defaultSaltLength
		}

		h.saltLength = length
	})
}

// WithKeyLength sets the length, in bytes, of the derived key. 0 means default.
func WithKeyLength(length uint32) Option {
	return optionFunc(func(h *Hasher) {
		if length == 0 {
			length = defaultKeyLength
		}

		h.keyLength = length
	})
}
