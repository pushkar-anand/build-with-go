package logger

import "log/slog"

// Error returns err as a log attribute under the conventional "error" key.
func Error(err error) slog.Attr {
	return slog.Any("error", err)
}
