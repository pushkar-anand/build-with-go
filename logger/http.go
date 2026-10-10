package logger

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/pushkar-anand/build-with-go/ctxval"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

// WriteHeader records the status code on its way through, so the middleware
// can log what was actually sent.
func (rw *responseWriter) WriteHeader(status int) {
	if rw.status == 0 {
		rw.status = status
	}
	rw.ResponseWriter.WriteHeader(status)
}

// Write records the response size, and the implicit 200 that writing without
// an explicit WriteHeader produces.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Unwrap exposes the underlying ResponseWriter so http.ResponseController can
// reach the flushing and hijacking it supports. Streaming needs this.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

type httpLogger struct {
	log  *slog.Logger
	next http.Handler
}

// ServeHTTP times the request, then logs it once the handler returns, at error
// level for a 5xx and info otherwise.
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
			slog.String("remote_ip", clientIP(r)),
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

// clientIP returns the address to log for r: the client address
// http/clientip resolved, or the connection's address when that middleware
// did not run. Forwarding headers are never read here, since any client can
// send one.
func clientIP(r *http.Request) string {
	if addr, ok := ctxval.ClientAddrFromContext(r.Context()); ok {
		return addr.String()
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
