package logger

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(status int) {
	if rw.status == 0 {
		rw.status = status
	}
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

type httpLogger struct {
	log  *slog.Logger
	next http.Handler
}

func (l *httpLogger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	rw := &responseWriter{
		ResponseWriter: w,
		status:         0,
		size:           0,
	}

	defer func() {
		if rw.status == 0 {
			rw.status = http.StatusOK
		}

		duration := time.Since(start)

		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("protocol", r.Proto),
			slog.String("remote_ip", getClientIP(r)),
			slog.String("user_agent", r.UserAgent()),
			slog.Int("status", rw.status),
			slog.Int("bytes", rw.size),
			slog.Duration("duration", duration),
		}

		level := slog.LevelInfo
		if rw.status >= 500 {
			level = slog.LevelError
		}

		l.log.LogAttrs(r.Context(), level, "HTTP Request", attrs...)
	}()

	l.next.ServeHTTP(rw, r)
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain a comma-separated list of IPs.
		// The first one is the original client.
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	return r.RemoteAddr
}

// NewHTTPLogger returns a middleware that logs HTTP requests using slog.
func NewHTTPLogger(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		l := &httpLogger{
			log:  log,
			next: next,
		}
		return l
	}
}
