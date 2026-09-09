// Package sqlitestore keeps scs session data in a SQLite database, so sessions
// outlive the process that created them.
//
// It imports no driver, so any SQLite driver works -- including the CGo-free
// modernc.org/sqlite.
//
// The caller owns the schema. Create the table and its index before building a
// store; the columns are fixed, the name is Config.TableName (default
// "sessions"):
//
//	CREATE TABLE sessions (
//	    token  TEXT    PRIMARY KEY,
//	    data   BLOB    NOT NULL,
//	    expiry INTEGER NOT NULL
//	);
//
//	CREATE INDEX sessions_expiry_idx ON sessions (expiry);
//
// expiry is a Unix nanosecond count, not a timestamp string, so expiry checks
// never depend on how a driver formats or parses time.
package sqlitestore

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/pushkar-anand/build-with-go/logger"
)

const (
	// defaultCleanupInterval bounds how long an expired row sits in the table.
	// Find already refuses to serve one, so the sweep is only about disk space.
	defaultCleanupInterval = 5 * time.Minute

	defaultTableName = "sessions"
)

// tableName must be a bare identifier: the store interpolates it into every
// statement, so a value that could carry SQL is refused rather than escaped.
var tableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Config is the input to NewWithConfig. Its zero value is valid and matches
// what New does: the "sessions" table, swept every five minutes.
type Config struct {
	// TableName is the table the store reads and writes. Empty means "sessions".
	TableName string

	// CleanupInterval is how often the background goroutine deletes expired
	// rows. Zero selects the five-minute default; a negative value turns the
	// sweep off, leaving cleanup to the caller or to Find's own per-read check.
	CleanupInterval time.Duration
}

// SQLiteStore is an [scs.Store] backed by a SQLite table.
type SQLiteStore struct {
	db          *sql.DB
	stopCleanup chan bool

	findQuery   string
	commitQuery string
	deleteQuery string
	allQuery    string
	sweepQuery  string
}

// CtxStore, not just Store: scs then passes the request context down to each
// query rather than calling the context-free methods. IterableStore backs
// SessionManager.Iterate, e.g. to end every session for one account.
var (
	_ scs.CtxStore         = (*SQLiteStore)(nil)
	_ scs.IterableStore    = (*SQLiteStore)(nil)
	_ scs.IterableCtxStore = (*SQLiteStore)(nil)
)

// New returns a store over db whose background goroutine clears expired rows
// every five minutes. Call StopCleanup to end that goroutine once the store is
// done with; a process-lifetime server never has to.
func New(db *sql.DB) *SQLiteStore {
	return newStore(db, defaultTableName, defaultCleanupInterval)
}

// NewWithCleanupInterval returns a store over db, sweeping expired rows every
// interval. A zero or negative interval starts no goroutine.
func NewWithCleanupInterval(db *sql.DB, interval time.Duration) *SQLiteStore {
	return newStore(db, defaultTableName, interval)
}

// NewWithConfig returns a store configured by cfg. It errors only when
// cfg.TableName is set to something other than a bare SQL identifier.
func NewWithConfig(db *sql.DB, cfg Config) (*SQLiteStore, error) {
	table := cmp.Or(cfg.TableName, defaultTableName)
	if !tableName.MatchString(table) {
		return nil, fmt.Errorf("sqlitestore: table name %q is not a bare SQL identifier", cfg.TableName)
	}

	interval := cfg.CleanupInterval
	if interval == 0 {
		interval = defaultCleanupInterval
	}

	return newStore(db, table, interval), nil
}

func newStore(db *sql.DB, table string, cleanupInterval time.Duration) *SQLiteStore {
	s := &SQLiteStore{
		db:        db,
		findQuery: fmt.Sprintf(`SELECT data FROM %s WHERE token = ? AND expiry > ?`, table),
		commitQuery: fmt.Sprintf(`INSERT INTO %s (token, data, expiry) VALUES (?, ?, ?) `+
			`ON CONFLICT (token) DO UPDATE SET data = excluded.data, expiry = excluded.expiry`, table),
		deleteQuery: fmt.Sprintf(`DELETE FROM %s WHERE token = ?`, table),
		allQuery:    fmt.Sprintf(`SELECT token, data FROM %s WHERE expiry > ?`, table),
		sweepQuery:  fmt.Sprintf(`DELETE FROM %s WHERE expiry <= ?`, table),
	}

	if cleanupInterval > 0 {
		s.stopCleanup = make(chan bool)
		go s.startCleanup(cleanupInterval)
	}

	return s
}

// Find returns the data stored under token, or found == false when no unexpired
// row holds it. A missing, expired, or malformed token is reported as not found,
// never as an error.
func (s *SQLiteStore) Find(token string) (b []byte, found bool, err error) {
	return s.FindCtx(context.Background(), token)
}

// FindCtx is Find with a caller-supplied context.
func (s *SQLiteStore) FindCtx(ctx context.Context, token string) (b []byte, found bool, err error) {
	err = s.db.QueryRowContext(ctx, s.findQuery, token, time.Now().UnixNano()).Scan(&b)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}

	return b, true, nil
}

// Commit stores b under token until expiry, replacing whatever row token
// already had.
func (s *SQLiteStore) Commit(token string, b []byte, expiry time.Time) error {
	return s.CommitCtx(context.Background(), token, b, expiry)
}

// CommitCtx is Commit with a caller-supplied context.
func (s *SQLiteStore) CommitCtx(ctx context.Context, token string, b []byte, expiry time.Time) error {
	_, err := s.db.ExecContext(ctx, s.commitQuery, token, b, expiry.UnixNano())

	return err
}

// Delete removes the row for token, and is a no-op when there is none.
func (s *SQLiteStore) Delete(token string) error {
	return s.DeleteCtx(context.Background(), token)
}

// DeleteCtx is Delete with a caller-supplied context.
func (s *SQLiteStore) DeleteCtx(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, s.deleteQuery, token)

	return err
}

// All returns the data for every unexpired session, keyed by token, or an empty
// map when there are none.
func (s *SQLiteStore) All() (map[string][]byte, error) {
	return s.AllCtx(context.Background())
}

// AllCtx is All with a caller-supplied context.
func (s *SQLiteStore) AllCtx(ctx context.Context) (map[string][]byte, error) {
	rows, err := s.db.QueryContext(ctx, s.allQuery, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	sessions := make(map[string][]byte)

	for rows.Next() {
		var (
			token string
			data  []byte
		)

		if err := rows.Scan(&token, &data); err != nil {
			return nil, err
		}

		sessions[token] = data
	}

	return sessions, rows.Err()
}

// StopCleanup ends the background cleanup goroutine. Call it once, from a single
// goroutine; a store built with a non-positive interval has no goroutine and
// StopCleanup does nothing.
func (s *SQLiteStore) StopCleanup() {
	if s.stopCleanup != nil {
		s.stopCleanup <- true
	}
}

func (s *SQLiteStore) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.deleteExpired(); err != nil {
				slog.Error("sqlitestore: sweep expired sessions", logger.Err(err))
			}
		case <-s.stopCleanup:
			return
		}
	}
}

func (s *SQLiteStore) deleteExpired() error {
	_, err := s.db.Exec(s.sweepQuery, time.Now().UnixNano())

	return err
}
