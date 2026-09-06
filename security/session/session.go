// Package session is a typed session store: one generic type, Session[T],
// holds the whole shape of a session as a single document rather than a bag of
// independently keyed values.
//
// The type parameter is the application's session schema -- the consistent
// data it wants to carry across requests: an identity, its role, a one-time
// flash message. With it, the session's shape is one Go type, discoverable in
// go doc, and a misspelled or mismatched field stops compiling instead of
// surfacing at runtime:
//
//	type Identity struct {
//		UserID   int64
//		Username string
//		Role     string
//		Flash    map[string]string
//	}
//
//	sm := session.New[Identity]()
//
//	sm.Update(ctx, func(d *Identity) {
//		d.UserID = user.ID
//		d.Role = string(user.Role)
//	})
//
//	who, ok := sm.Current(ctx)
package session

import (
	"context"
	"encoding/gob"
	"net/http"
	"reflect"
	"sync"

	"github.com/alexedwards/scs/v2"
)

// dataKey stores the session data<T>
const dataKey = "session.data"

// Session is a typed session document: the middleware that carts it around
// each request, the lifecycle operations that renew and end it, and the typed
// read and write that make its contents one struct, T, instead of a bag.
type Session[T any] struct {
	core
}

// New returns a Session whose document is a T
func New[T any](opts ...Option) *Session[T] {
	registerType[T]()

	s := &Session[T]{core: newCore()}

	for _, opt := range opts {
		opt.apply(&s.core)
	}

	return s
}

// Current returns the session's data T, or false if none exists
func (s *Session[T]) Current(ctx context.Context) (T, bool) {
	data, ok := s.manager.Get(ctx, dataKey).(T)
	return data, ok
}

// Set replaces the session's document with data.
func (s *Session[T]) Set(ctx context.Context, data T) {
	s.manager.Put(ctx, dataKey, data)
}

// Update applies fn to the session's data
func (s *Session[T]) Update(ctx context.Context, fn func(*T)) {
	data, _ := s.Current(ctx)
	fn(&data)
	s.Set(ctx, data)
}

// Clear drops the session's data but keeps the session itself alive.
func (s *Session[T]) Clear(ctx context.Context) {
	s.manager.Remove(ctx, dataKey)
}

// LoadAndSave wraps next with the middleware every request must pass through
// before any session data can be touched: it reads the session cookie, loads
// the matching session into the request context, and commits any changes made
// along the way back out.
func (s *Session[T]) LoadAndSave(next http.Handler) http.Handler {
	return s.manager.LoadAndSave(next)
}

// Renew updates the session data to have a new session token while
// retaining the current session data.
func (s *Session[T]) Renew(ctx context.Context) error {
	return s.manager.RenewToken(ctx)
}

// Destroy deletes the session data from the session store and sets
// the session status to Destroyed. Any further operations in the same
// request cycle will result in a new session being created.
func (s *Session[T]) Destroy(ctx context.Context) error {
	return s.manager.Destroy(ctx)
}

// RememberMe opts this one session into surviving a browser restart even when
// the session was configured not to persist by default, as New does.
func (s *Session[T]) RememberMe(ctx context.Context, persist bool) {
	s.manager.RememberMe(ctx, persist)
}

// Token returns the session's token, or "" before the session has ever been
// committed.
func (s *Session[T]) Token(ctx context.Context) string {
	return s.manager.Token(ctx)
}

// Status reports where the session is in the current request cycle.
func (s *Session[T]) Status(ctx context.Context) scs.Status {
	return s.manager.Status(ctx)
}

// Manager returns the session manager used by this session.
func (s *Session[T]) Manager() *scs.SessionManager {
	return s.manager
}

var (
	registerMu sync.Mutex
	registered = map[reflect.Type]struct{}{}
)

// registerType tells gob about T so the default codec can encode and decode a
// document crossing a session store. Safe to call multiple times concurrently.
func registerType[T any]() {
	t := reflect.TypeFor[T]()

	registerMu.Lock()
	_, ok := registered[t]
	registerMu.Unlock()
	if ok {
		return
	}

	var v any
	if t.Kind() == reflect.Pointer {
		v = reflect.New(t.Elem()).Interface()
	} else {
		v = reflect.Zero(t).Interface()
	}

	defer func() { _ = recover() }()
	gob.Register(v)

	registerMu.Lock()
	registered[t] = struct{}{}
	registerMu.Unlock()
}
