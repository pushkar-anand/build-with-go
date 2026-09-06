// Package session wraps an scs.SessionManager with sensible defaults that
// options can override.
package session

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
)

const (
	defaultLifetime    = 7 * 24 * time.Hour
	defaultIdleTimeout = 24 * time.Hour

	defaultCookieName     = "_session"
	defaultCookiePath     = "/"
	defaultCookieHttpOnly = true
	defaultCookieSameSite = http.SameSiteStrictMode
	defaultCookiePersist  = false
)

// Session wraps an scs.SessionManager.
type Session struct {
	log *slog.Logger
	sm  *scs.SessionManager
}

// New returns a Session configured with sensible defaults, overridden by any
// options provided.
func New(opts ...Option) *Session {
	s := &Session{
		log: slog.Default(),
		sm:  scs.New(),
	}

	s.sm.Lifetime = defaultLifetime
	s.sm.IdleTimeout = defaultIdleTimeout

	s.sm.Cookie.Name = defaultCookieName
	s.sm.Cookie.Path = defaultCookiePath
	s.sm.Cookie.HttpOnly = defaultCookieHttpOnly
	s.sm.Cookie.SameSite = defaultCookieSameSite
	s.sm.Cookie.Persist = defaultCookiePersist

	for _, opt := range opts {
		opt.apply(s)
	}

	return s
}
