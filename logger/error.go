package logger

import "log/slog"

// ErrorKey is the key Err logs under, exported so that callers can filter and
// query on the same name the library writes.
const ErrorKey = "error"

// Err returns err as a log attribute.
//
// A nil error yields the empty Attr, which handlers drop, so logging an error
// that may be nil does not leave "error=<nil>" behind.
//
// It is named Err rather than Error so that it does not read as a call that
// logs something, and does not stutter beside slog's own Error methods.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}

	return slog.Any(ErrorKey, err)
}
