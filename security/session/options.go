package session

import (
	"log/slog"
	"net/http"
	"time"
)

type (
	// Option configures a Session.
	Option interface {
		apply(*Session)
	}

	optionFunc func(*Session)
)

func (fn optionFunc) apply(s *Session) {
	fn(s)
}

// WithLogger sets the logger used by the session. 0 means default.
func WithLogger(log *slog.Logger) Option {
	return optionFunc(func(s *Session) {
		if log == nil {
			log = slog.Default()
		}

		s.log = log
	})
}

// WithLifetime sets the maximum length of time a session is valid for.
func WithLifetime(d time.Duration) Option {
	return optionFunc(func(s *Session) {
		s.sm.Lifetime = d
	})
}

// WithIdleTimeout sets the maximum length of time a session can be inactive
// before it expires.
func WithIdleTimeout(d time.Duration) Option {
	return optionFunc(func(s *Session) {
		s.sm.IdleTimeout = d
	})
}

// WithCookieName sets the name of the session cookie.
func WithCookieName(name string) Option {
	return optionFunc(func(s *Session) {
		s.sm.Cookie.Name = name
	})
}

// WithCookiePath sets the Path attribute of the session cookie.
func WithCookiePath(path string) Option {
	return optionFunc(func(s *Session) {
		s.sm.Cookie.Path = path
	})
}

// WithCookieHttpOnly sets the HttpOnly attribute of the session cookie.
func WithCookieHttpOnly(httponly bool) Option {
	return optionFunc(func(s *Session) {
		s.sm.Cookie.HttpOnly = httponly
	})
}

// WithCookieSameSite sets the SameSite attribute of the session cookie.
func WithCookieSameSite(sameSite http.SameSite) Option {
	return optionFunc(func(s *Session) {
		s.sm.Cookie.SameSite = sameSite
	})
}

// WithCookiePersist sets whether the session cookie is persisted across
// browser restarts.
func WithCookiePersist(persist bool) Option {
	return optionFunc(func(s *Session) {
		s.sm.Cookie.Persist = persist
	})
}
