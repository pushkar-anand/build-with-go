package session

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
)

type (
	// Option configures a Session.
	//
	// Options apply in order, so a later one wins over an earlier one touching
	// the same setting.
	Option interface {
		apply(*core)
	}

	optionFunc func(*core)
)

func (fn optionFunc) apply(c *core) {
	fn(c)
}

// WithLogger sets the logger the session middleware reports failures through.
// nil means the default (slog.Default).
func WithLogger(log *slog.Logger) Option {
	return optionFunc(func(c *core) {
		if log == nil {
			log = slog.Default()
		}

		c.log = log
	})
}

// WithStore sets where session data is persisted. New uses an in-memory store;
// bring a durable one (scs's postgres, redis, mysql stores, say) for anything
// that must survive a restart.
func WithStore(store scs.Store) Option {
	return optionFunc(func(c *core) {
		c.manager.Store = store
	})
}

// WithCodec sets how session data is encoded before it reaches the store. The
// default is gob, and Session registers its document type for it, so the
// default works without setup; a custom codec is the caller's own contract.
func WithCodec(codec scs.Codec) Option {
	return optionFunc(func(c *core) {
		c.manager.Codec = codec
	})
}

// WithHashTokenInStore stores a SHA-256 of the session token rather than the
// token itself, so a store leak does not hand out live sessions.
func WithHashTokenInStore(hash bool) Option {
	return optionFunc(func(c *core) {
		c.manager.HashTokenInStore = hash
	})
}

// WithErrorFunc switches how the middleware responds when a session cannot be
// loaded. The default logs the failure through the Session's logger and
// returns a 500.
func WithErrorFunc(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return optionFunc(func(c *core) {
		c.manager.ErrorFunc = fn
	})
}

// WithLifetime sets the maximum length of time a session is valid for.
func WithLifetime(d time.Duration) Option {
	return optionFunc(func(c *core) {
		c.manager.Lifetime = d
	})
}

// WithIdleTimeout sets the maximum length of time a session can be inactive
// before it expires.
func WithIdleTimeout(d time.Duration) Option {
	return optionFunc(func(c *core) {
		c.manager.IdleTimeout = d
	})
}

// WithCookieName sets the name of the session cookie.
func WithCookieName(name string) Option {
	return optionFunc(func(c *core) {
		c.manager.Cookie.Name = name
	})
}

// WithCookieDomain sets the Domain attribute of the session cookie.
func WithCookieDomain(domain string) Option {
	return optionFunc(func(c *core) {
		c.manager.Cookie.Domain = domain
	})
}

// WithCookiePath sets the Path attribute of the session cookie.
func WithCookiePath(path string) Option {
	return optionFunc(func(c *core) {
		c.manager.Cookie.Path = path
	})
}

// WithCookieHttpOnly sets the HttpOnly attribute of the session cookie.
func WithCookieHttpOnly(httponly bool) Option {
	return optionFunc(func(c *core) {
		c.manager.Cookie.HttpOnly = httponly
	})
}

// WithCookieSameSite sets the SameSite attribute of the session cookie.
func WithCookieSameSite(sameSite http.SameSite) Option {
	return optionFunc(func(c *core) {
		c.manager.Cookie.SameSite = sameSite
	})
}

// WithCookieSecure sets the Secure attribute of the session cookie, so the
// browser sends it only over HTTPS.
func WithCookieSecure(secure bool) Option {
	return optionFunc(func(c *core) {
		c.manager.Cookie.Secure = secure
	})
}

// WithCookiePartitioned sets the Partitioned attribute of the session cookie.
func WithCookiePartitioned(partitioned bool) Option {
	return optionFunc(func(c *core) {
		c.manager.Cookie.Partitioned = partitioned
	})
}

// WithCookiePersist controls whether every session cookie outlives the browser
// closing. New defaults to false and RememberMe then opts a specific session
// into persisting. A true here makes every session persistent instead.
func WithCookiePersist(persist bool) Option {
	return optionFunc(func(c *core) {
		c.manager.Cookie.Persist = persist
	})
}
