package session

import (
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/pushkar-anand/build-with-go/logger"
)

// core holds what Session needs from its manager and logger.
type core struct {
	log     *slog.Logger
	manager *scs.SessionManager
}

// newCore builds the shared manager and logger with the package defaults.
func newCore() core {
	c := core{
		log:     slog.Default(),
		manager: scs.New(),
	}

	c.manager.Lifetime = defaultLifetime
	c.manager.IdleTimeout = defaultIdleTimeout

	c.manager.Cookie.Name = defaultCookieName
	c.manager.Cookie.Path = defaultCookiePath
	c.manager.Cookie.HttpOnly = defaultCookieHttpOnly
	c.manager.Cookie.SameSite = defaultCookieSameSite
	c.manager.Cookie.Persist = false

	// add a custom error handler so we can log the error and end the
	// session gracefully.
	c.manager.ErrorFunc = func(w http.ResponseWriter, r *http.Request, err error) {
		c.log.ErrorContext(r.Context(), "session error", logger.Err(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

	return c
}
