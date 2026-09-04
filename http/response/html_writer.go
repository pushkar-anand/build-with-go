package response

import "log/slog"

func NewHTMLWriter(
	l *slog.Logger,
) *HTMLWriter {
	return &HTMLWriter{
		logger: l,
	}
}
