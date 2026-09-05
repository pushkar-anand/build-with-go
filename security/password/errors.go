package password

import "net/http"

var (
	ErrPasswordTooLong           = &passwordError{message: "password length exceeds RFC 9106 limits", status: http.StatusUnprocessableEntity, detail: "Password is too long"}
	ErrInvalidHashFormat         = &passwordError{message: "invalid hash format", status: http.StatusInternalServerError, detail: "Internal Server Error"}
	ErrInvalidHashVersion        = &passwordError{message: "invalid hash version", status: http.StatusInternalServerError, detail: "Internal Server Error"}
	ErrMismatchedHashAndPassword = &passwordError{message: "mismatched hash and password", status: http.StatusUnauthorized, detail: "Invalid credentials"}

	// ErrHashParamsOutOfRange guards against a tampered or malicious hash
	// forcing Compare to spend excessive memory or CPU deriving a key.
	ErrHashParamsOutOfRange = &passwordError{message: "hash cost parameters exceed allowed limits", status: http.StatusInternalServerError, detail: "Internal Server Error"}
)

// passwordError preserves diagnostic messages while providing default HTTP
// problem responses. Applications can override them with a response mapper.
type passwordError struct {
	message string
	status  int
	detail  string
}

func (e *passwordError) Error() string                 { return e.message }
func (e *passwordError) Type() string                  { return "about:blank" }
func (e *passwordError) Title() string                 { return http.StatusText(e.status) }
func (e *passwordError) Status() int                   { return e.status }
func (e *passwordError) Detail() string                { return e.detail }
func (e *passwordError) CustomMembers() map[string]any { return nil }
