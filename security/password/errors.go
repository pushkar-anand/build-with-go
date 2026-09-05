package password

import "errors"

var (
	ErrPasswordTooLong           = errors.New("password length exceeds RFC 9106 limits")
	ErrInvalidHashFormat         = errors.New("invalid hash format")
	ErrInvalidHashVersion        = errors.New("invalid hash version")
	ErrMismatchedHashAndPassword = errors.New("mismatched hash and password")
)
