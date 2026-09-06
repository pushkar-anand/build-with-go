package session

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2/memstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identity is the session document this package's tests use: an identity, its
// role, and a one-time flash message -- the consistent data a session carries.
type identity struct {
	UserID   int64
	Username string
	Role     string
	Flash    map[string]string
}

// loadSession starts a fresh session in a test context, the way the middleware
// does for a first visit.
func loadSession(t *testing.T, s *Session[identity]) context.Context {
	t.Helper()

	ctx, err := s.Load(t.Context(), "")
	require.NoError(t, err)

	return ctx
}

// twoSessions is a test-only type, so constructing two Sessions over it proves
// gob registration of a repeated document type is safe.
type twoSessions struct {
	N int
}

func TestNewRegistersTypeOnce(t *testing.T) {
	t.Parallel()

	first := New[identity]()
	second := New[identity]()
	_ = New[twoSessions]()

	assert.NotNil(t, first)
	assert.NotNil(t, second)
}

func TestSetCurrent(t *testing.T) {
	t.Parallel()

	s := New[identity]()
	ctx := loadSession(t, s)

	_, ok := s.Current(ctx)
	assert.False(t, ok, "a never-written session holds no document")

	s.Set(ctx, identity{UserID: 42, Username: "ada", Role: "admin"})

	user, ok := s.Current(ctx)
	require.True(t, ok)
	assert.Equal(t, int64(42), user.UserID)
	assert.Equal(t, "ada", user.Username)
	assert.Equal(t, "admin", user.Role)
	assert.Equal(t, StatusModified, s.Status(ctx))
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	s := New[identity]()
	ctx := loadSession(t, s)

	t.Run("builds a document from nothing", func(t *testing.T) {
		s.Update(ctx, func(d *identity) {
			d.UserID = 42
			d.Role = "admin"
			d.Flash = map[string]string{"notice": "saved"}
		})

		user, ok := s.Current(ctx)
		require.True(t, ok)
		assert.Equal(t, "admin", user.Role)
		assert.Equal(t, "saved", user.Flash["notice"])
	})

	t.Run("changes one field, keeps the rest", func(t *testing.T) {
		s.Update(ctx, func(d *identity) {
			d.Username = "grace"
		})

		user, ok := s.Current(ctx)
		require.True(t, ok)
		assert.Equal(t, int64(42), user.UserID, "an untouched field survives an Update")
		assert.Equal(t, "grace", user.Username)
	})

	t.Run("a flash is just a field to delete", func(t *testing.T) {
		s.Update(ctx, func(d *identity) {
			delete(d.Flash, "notice")
		})

		user, ok := s.Current(ctx)
		require.True(t, ok)
		_, still := user.Flash["notice"]
		assert.False(t, still, "the message is read exactly once")
	})
}

func TestClear(t *testing.T) {
	t.Parallel()

	s := New[identity]()
	ctx := loadSession(t, s)

	s.Set(ctx, identity{UserID: 42})
	s.Clear(ctx)

	_, ok := s.Current(ctx)
	assert.False(t, ok, "the document is gone")

	_, _, err := s.Commit(ctx)
	require.NoError(t, err, "the session itself survives a Clear")
	assert.Equal(t, StatusModified, s.Status(ctx))
}

func TestLoadAndSavePersistsAcrossRequests(t *testing.T) {
	t.Parallel()

	s := New[identity]()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		s.Renew(r.Context())
		s.Set(r.Context(), identity{UserID: 42, Username: "ada", Role: "admin"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /who", func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.Current(r.Context())
		if !ok {
			http.Error(w, "anon", http.StatusUnauthorized)
			return
		}

		fmt.Fprintf(w, "%s@%d(%s)", user.Username, user.UserID, user.Role)
	})

	ts := httptest.NewServer(s.LoadAndSave(mux))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/login", "", nil)
	require.NoError(t, err)
	res.Body.Close()
	cookie := res.Cookies()[0]

	body := get(t, ts.URL, cookie, "/who", http.StatusOK)
	assert.Equal(t, "ada@42(admin)", body)

	get(t, ts.URL, nil, "/who", http.StatusUnauthorized)
}

func TestFlashViaDocument(t *testing.T) {
	t.Parallel()

	s := New[identity]()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /submit", func(w http.ResponseWriter, r *http.Request) {
		s.Update(r.Context(), func(d *identity) {
			d.UserID = 42
			if d.Flash == nil {
				d.Flash = map[string]string{}
			}
			d.Flash["notice"] = "saved"
		})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /next", func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.Current(r.Context())
		if !ok {
			http.Error(w, "no session", http.StatusNotFound)
			return
		}

		notice, ok := user.Flash["notice"]
		if !ok {
			http.Error(w, "no flash", http.StatusNotFound)
			return
		}

		s.Update(r.Context(), func(d *identity) {
			delete(d.Flash, "notice")
		})
		fmt.Fprint(w, notice)
	})

	ts := httptest.NewServer(s.LoadAndSave(mux))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/submit", "", nil)
	require.NoError(t, err)
	res.Body.Close()
	cookie := res.Cookies()[0]

	body := get(t, ts.URL, cookie, "/next", http.StatusOK)
	assert.Equal(t, "saved", body, "the flash survives the redirect")

	get(t, ts.URL, cookie, "/next", http.StatusNotFound)
}

func TestValueSurvivesRoundTrip(t *testing.T) {
	t.Parallel()

	s := New[identity]()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /set", func(w http.ResponseWriter, r *http.Request) {
		s.Set(r.Context(), identity{
			UserID:   7,
			Username: "grace",
			Role:     "reader",
			Flash:    map[string]string{"once": "x"},
		})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /get", func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.Current(r.Context())
		if !ok {
			http.Error(w, "no doc", http.StatusNotFound)
			return
		}

		fmt.Fprintf(w, "%s@%d %s %s", user.Username, user.UserID, user.Role, user.Flash["once"])
	})

	ts := httptest.NewServer(s.LoadAndSave(mux))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/set", "", nil)
	require.NoError(t, err)
	res.Body.Close()
	cookie := res.Cookies()[0]

	body := get(t, ts.URL, cookie, "/get", http.StatusOK)
	assert.Equal(t, "grace@7 reader x", body)
}

func TestRenewKeepsDocument(t *testing.T) {
	t.Parallel()

	s := New[identity]()
	ctx := loadSession(t, s)

	s.Set(ctx, identity{UserID: 42, Role: "admin"})
	old, _, err := s.Commit(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, old)

	require.NoError(t, s.Renew(ctx))
	assert.NotEqual(t, old, s.Token(ctx), "a new token replaces the old one")

	user, ok := s.Current(ctx)
	require.True(t, ok)
	assert.Equal(t, int64(42), user.UserID, "renewal keeps the document")
}

func TestDestroy(t *testing.T) {
	t.Parallel()

	s := New[identity]()
	ctx := loadSession(t, s)

	s.Set(ctx, identity{UserID: 42})

	require.NoError(t, s.Destroy(ctx))
	assert.Equal(t, StatusDestroyed, s.Status(ctx))

	_, ok := s.Current(ctx)
	assert.False(t, ok, "a destroyed session holds no document")
}

func TestLogoutViaMiddleware(t *testing.T) {
	t.Parallel()

	s := New[identity]()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, s.Destroy(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	})

	ts := httptest.NewServer(s.LoadAndSave(mux))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/logout", "", nil)
	require.NoError(t, err)
	res.Body.Close()

	for _, c := range res.Cookies() {
		if c.Name == s.manager.Cookie.Name {
			assert.Equal(t, -1, c.MaxAge, "the session cookie asks the browser to drop it")
		}
	}
}

func TestRememberMeAgainstACookie(t *testing.T) {
	t.Parallel()

	s := New[identity]()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		s.RememberMe(r.Context(), true)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /login-brief", func(w http.ResponseWriter, r *http.Request) {
		s.Set(r.Context(), identity{UserID: 1})
		w.WriteHeader(http.StatusNoContent)
	})

	ts := httptest.NewServer(s.LoadAndSave(mux))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/login", "", nil)
	require.NoError(t, err)
	res.Body.Close()
	remembered := res.Cookies()[0]
	assert.NotEmpty(t, remembered.Expires, "a remembered session cookie gets an expiry")
	assert.Greater(t, remembered.MaxAge, 0, "that expiry lies in the future")

	res, err = http.Post(ts.URL+"/login-brief", "", nil)
	require.NoError(t, err)
	res.Body.Close()
	brief := res.Cookies()[0]
	assert.True(t, brief.Expires.IsZero(), "an unremembered session cookie dies with the browser")
	assert.Zero(t, brief.MaxAge)
}

func TestHashTokenInStore(t *testing.T) {
	t.Parallel()

	store := memstore.New()
	s := New[identity](
		WithStore(store),
		WithHashTokenInStore(true),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /sentinel", func(w http.ResponseWriter, r *http.Request) {
		s.Set(r.Context(), identity{UserID: 42})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /check", func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.Current(r.Context())
		if !ok {
			http.Error(w, "no sentinel", http.StatusNotFound)
			return
		}

		fmt.Fprint(w, user.UserID)
	})

	ts := httptest.NewServer(s.LoadAndSave(mux))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/sentinel", "", nil)
	require.NoError(t, err)
	res.Body.Close()
	cookie := res.Cookies()[0]

	all, err := store.All()
	require.NoError(t, err)
	require.Len(t, all, 1)

	for stored := range all {
		assert.NotEqual(t, cookie.Value, stored, "the store holds a hash of the token, not the token")
	}

	body := get(t, ts.URL, cookie, "/check", http.StatusOK)
	assert.Equal(t, "42", body, "the original cookie still reads its session back")
}

func TestCommitAndLoad(t *testing.T) {
	t.Parallel()

	s := New[identity]()
	ctx := loadSession(t, s)

	s.Set(ctx, identity{UserID: 42, Username: "ada"})
	token, _, err := s.Commit(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	ctx2, err := s.Load(t.Context(), token)
	require.NoError(t, err)

	user, ok := s.Current(ctx2)
	require.True(t, ok)
	assert.Equal(t, "ada", user.Username)
}

func TestOptions(t *testing.T) {
	t.Parallel()

	s := New[identity](
		WithStore(memstore.New()),
		WithCookieName("app_session"),
		WithCookieDomain("example.com"),
		WithCookiePath("/app"),
		WithCookieHttpOnly(false),
		WithCookieSameSite(http.SameSiteLaxMode),
		WithCookieSecure(true),
		WithCookiePartitioned(true),
		WithCookiePersist(true),
		WithHashTokenInStore(true),
		WithLifetime(2),
		WithIdleTimeout(1),
	)

	c := s.manager.Cookie
	assert.Equal(t, "app_session", c.Name)
	assert.Equal(t, "example.com", c.Domain)
	assert.Equal(t, "/app", c.Path)
	assert.False(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.True(t, c.Secure)
	assert.True(t, c.Partitioned)
	assert.True(t, c.Persist)
	assert.True(t, s.manager.HashTokenInStore)
	assert.Equal(t, time.Duration(2), s.manager.Lifetime)
	assert.Equal(t, time.Duration(1), s.manager.IdleTimeout)
}

func TestTokenEmptyBeforeCommit(t *testing.T) {
	t.Parallel()

	s := New[identity]()
	ctx := loadSession(t, s)

	assert.Empty(t, s.Token(ctx))
}

// get performs a GET with the given cookie and returns the response body,
// asserting the status code along the way.
func get(t *testing.T, base string, cookie *http.Cookie, path string, wantStatus int) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	require.NoError(t, err)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	assert.Equal(t, wantStatus, res.StatusCode)

	return string(body)
}

func Example() {
	sm := New[identity]()

	ctx, err := sm.Load(context.Background(), "")
	if err != nil {
		panic(err)
	}

	sm.Set(ctx, identity{UserID: 42, Username: "ada", Role: "admin"})

	sm.Update(ctx, func(d *identity) {
		d.Flash = map[string]string{"notice": "saved"}
	})

	user, ok := sm.Current(ctx)
	fmt.Printf("user=%s role=%s notice=%s ok=%v\n", user.Username, user.Role, user.Flash["notice"], ok)

	// Output: user=ada role=admin notice=saved ok=true
}
