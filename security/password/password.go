package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// MaxRFC9106PasswdLen is the maximum length of a password that can be stored
const MaxRFC9106PasswdLen int = (2 << 31) - 1

// hashFormat is the format of the argon2id hash.
const hashFormat = "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s"

const (
	version    = argon2.Version
	saltLength = 16
	keyLength  = 64
)

const (
	defaultTime        = 4
	defaultMemory      = 64 * 1024
	defaultParallelism = 2
)

// Hasher is a password hasher
type Hasher struct {
	pepper                              string
	time, memory, saltLength, keyLength uint32
	parallelism                         uint8
}

// NewHasher returns a new Hasher
// with sensible defaults is none are provided
func NewHasher(
	memory, time uint32,
	parallelism uint8,
	pepper string,
) *Hasher {
	if memory == 0 {
		memory = defaultMemory
	}

	if time == 0 {
		time = defaultTime
	}

	if parallelism == 0 {
		parallelism = defaultParallelism
	}

	return &Hasher{
		pepper:      pepper,
		time:        time,
		memory:      memory,
		saltLength:  saltLength,
		parallelism: parallelism,
		keyLength:   keyLength,
	}
}

// Hash generates a secure hash of the given password
func (h *Hasher) Hash(password string) (string, error) {
	if len(password) > MaxRFC9106PasswdLen {
		return "", ErrPasswordTooLong
	}

	salt, err := generateRandomBytes(saltLength)
	if err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	defer eraseBuf(salt)

	fsalt := append(salt, h.pepper...)
	defer eraseBuf(fsalt)

	key := argon2.IDKey([]byte(password), fsalt, h.time, h.memory, h.parallelism, keyLength)
	defer eraseBuf(key)

	hash := fmt.Sprintf(
		hashFormat,
		version, h.memory, defaultTime, h.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)

	return hash, err
}

// Compare compares a plaintext password against a hash in constant time.
func (h *Hasher) Compare(
	password, hash string,
) error {
	split := strings.Split(hash, "$")
	if len(split) != 6 {
		return ErrInvalidHashFormat
	}

	if split[0] != "" || split[1] != "argon2id" {
		return ErrInvalidHashFormat
	}

	var (
		_version, _memory, _time uint32
		_parallelism             uint8
	)

	_, err := fmt.Sscanf(split[2], "v=%d", &_version)
	if err != nil {
		return ErrInvalidHashFormat
	}

	if _version != version {
		return ErrInvalidHashVersion
	}

	_, err = fmt.Sscanf(split[3], "m=%d,t=%d,p=%d", &_memory, &_time, &_parallelism)
	if err != nil {
		return ErrInvalidHashFormat
	}

	salt, err := base64.RawStdEncoding.DecodeString(split[4])
	if err != nil {
		return ErrInvalidHashFormat
	}
	defer eraseBuf(salt)

	fsalt := append(salt, h.pepper...)
	defer eraseBuf(fsalt)

	key, err := base64.RawStdEncoding.DecodeString(split[5])
	if err != nil {
		return ErrInvalidHashFormat
	}
	defer eraseBuf(key)

	_keyLen := uint32(len(key))

	derivedKey := argon2.IDKey([]byte(password), fsalt, _time, _memory, _parallelism, _keyLen)
	defer eraseBuf(derivedKey)

	match := subtle.ConstantTimeCompare(key, derivedKey)
	if match != 1 {
		return ErrMismatchedHashAndPassword
	}

	return nil
}

// generateRandomBytes generates n random bytes
func generateRandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)

	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("reading random bytes: %w", err)
	}

	return b, nil
}

// eraseBuf fills len(buf) with space characters.
// if buf is nil or zero length, eraseBuf takes no action.
func eraseBuf(buf []byte) {
	for i := range buf {
		buf[i] = 0x00
	}
}
