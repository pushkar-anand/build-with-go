package password

import "errors"

var (
	ErrPasswordTooLong           = errors.New("password length exceeds RFC 9106 limits")
	ErrInvalidHashFormat         = errors.New("invalid hash format")
	ErrInvalidHashVersion        = errors.New("invalid hash version")
	ErrMismatchedHashAndPassword = errors.New("mismatched hash and password")

	// ErrHashParamsOutOfRange guards against a tampered or malicious hash
	// forcing Compare to spend excessive memory or CPU deriving a key.
	ErrHashParamsOutOfRange = errors.New("hash cost parameters exceed allowed limits")
)
