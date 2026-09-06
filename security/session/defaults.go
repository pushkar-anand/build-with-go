package session

import (
	"net/http"
	"time"
)

// Session defaults, overridable per option.
const (
	defaultLifetime    = 7 * 24 * time.Hour
	defaultIdleTimeout = 24 * time.Hour

	defaultCookieName     = "_session"
	defaultCookiePath     = "/"
	defaultCookieHttpOnly = true
	defaultCookieSameSite = http.SameSiteStrictMode
)
