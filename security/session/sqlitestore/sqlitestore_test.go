package sqlitestore_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/pushkar-anand/build-with-go/security/session/sqlitestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// newDB returns a test-scoped database holding the default "sessions" table.
func newDB(t *testing.T) *sql.DB {
	t.Helper()

	return newNamedDB(t, "sessions")
}

// newNamedDB is newDB with the store's schema under an arbitrary table name.
func newNamedDB(t *testing.T, table string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "sessions.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	for _, stmt := range []string{
		fmt.Sprintf(`CREATE TABLE %s (
			token  TEXT    PRIMARY KEY,
			data   BLOB    NOT NULL,
			expiry INTEGER NOT NULL
		)`, table),
		fmt.Sprintf(`CREATE INDEX %s_expiry_idx ON %s (expiry)`, table, table),
	} {
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}

	return db
}

// rowCount reports how many rows the sessions table holds, expired or not.
func rowCount(t *testing.T, db *sql.DB) int {
	t.Helper()

	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&n))

	return n
}

// scs only threads the request context into a store that is a CtxStore.
var _ scs.CtxStore = (*sqlitestore.SQLiteStore)(nil)

func TestCommitThenFind(t *testing.T) {
	t.Parallel()

	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	require.NoError(t, s.Commit("token-a", []byte("payload"), time.Now().Add(time.Hour)))

	b, found, err := s.Find("token-a")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, []byte("payload"), b)
}

func TestFindMissingToken(t *testing.T) {
	t.Parallel()

	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	b, found, err := s.Find("absent")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, b)
}

func TestFindExpiredToken(t *testing.T) {
	t.Parallel()

	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	require.NoError(t, s.Commit("token-a", []byte("payload"), time.Now().Add(-time.Second)))

	_, found, err := s.Find("token-a")
	require.NoError(t, err)
	assert.False(t, found, "an expired row is not served")
}

func TestCommitOverwrites(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	s := sqlitestore.NewWithCleanupInterval(db, 0)

	require.NoError(t, s.Commit("token-a", []byte("first"), time.Now().Add(time.Hour)))
	require.NoError(t, s.Commit("token-a", []byte("second"), time.Now().Add(time.Hour)))

	b, found, err := s.Find("token-a")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, []byte("second"), b)
	assert.Equal(t, 1, rowCount(t, db), "the token keeps one row")
}

func TestDelete(t *testing.T) {
	t.Parallel()

	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	require.NoError(t, s.Commit("token-a", []byte("payload"), time.Now().Add(time.Hour)))
	require.NoError(t, s.Delete("token-a"))

	_, found, err := s.Find("token-a")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestDeleteMissingTokenIsNoError(t *testing.T) {
	t.Parallel()

	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	assert.NoError(t, s.Delete("absent"))
}

func TestContextVariantsRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	require.NoError(t, s.CommitCtx(ctx, "token-a", []byte("payload"), time.Now().Add(time.Hour)))

	b, found, err := s.FindCtx(ctx, "token-a")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, []byte("payload"), b)

	require.NoError(t, s.DeleteCtx(ctx, "token-a"))

	_, found, err = s.FindCtx(ctx, "token-a")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestAllReturnsOnlyUnexpiredSessions(t *testing.T) {
	t.Parallel()

	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	require.NoError(t, s.Commit("live-a", []byte("a"), time.Now().Add(time.Hour)))
	require.NoError(t, s.Commit("live-b", []byte("b"), time.Now().Add(time.Hour)))
	require.NoError(t, s.Commit("dead", []byte("c"), time.Now().Add(-time.Second)))

	all, err := s.All()
	require.NoError(t, err)
	assert.Equal(t, map[string][]byte{"live-a": []byte("a"), "live-b": []byte("b")}, all)
}

func TestAllOnEmptyTable(t *testing.T) {
	t.Parallel()

	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	all, err := s.AllCtx(context.Background())
	require.NoError(t, err)
	assert.Empty(t, all)
	assert.NotNil(t, all)
}

func TestNewWithConfigUsesTheNamedTable(t *testing.T) {
	t.Parallel()

	db := newNamedDB(t, "app_sessions")

	s, err := sqlitestore.NewWithConfig(db, sqlitestore.Config{
		TableName:       "app_sessions",
		CleanupInterval: -1,
	})
	require.NoError(t, err)

	require.NoError(t, s.Commit("token-a", []byte("payload"), time.Now().Add(time.Hour)))

	b, found, err := s.Find("token-a")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, []byte("payload"), b)
}

func TestNewWithConfigRejectsAnUnsafeTableName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"a b", "a;drop", "a-b", "1abc", `"quoted"`, "s)", "sessions;--"} {
		_, err := sqlitestore.NewWithConfig(nil, sqlitestore.Config{TableName: name})
		assert.Errorf(t, err, "table name %q should be refused", name)
	}
}

func TestNewWithConfigZeroValueMatchesNew(t *testing.T) {
	t.Parallel()

	s, err := sqlitestore.NewWithConfig(newDB(t), sqlitestore.Config{})
	require.NoError(t, err)
	t.Cleanup(s.StopCleanup)

	require.NoError(t, s.Commit("token-a", []byte("payload"), time.Now().Add(time.Hour)))

	_, found, err := s.Find("token-a")
	require.NoError(t, err)
	assert.True(t, found)
}

func TestBackgroundCleanupRemovesExpiredRows(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	s := sqlitestore.NewWithCleanupInterval(db, 20*time.Millisecond)
	t.Cleanup(s.StopCleanup)

	require.NoError(t, s.Commit("expired", []byte("x"), time.Now().Add(-time.Second)))
	require.NoError(t, s.Commit("live", []byte("y"), time.Now().Add(time.Hour)))

	assert.Eventually(t, func() bool {
		return rowCount(t, db) == 1
	}, time.Second, 10*time.Millisecond, "the sweep drops the expired row")

	_, found, err := s.Find("live")
	require.NoError(t, err)
	assert.True(t, found, "the sweep leaves a live row alone")
}

func TestStopCleanupWithoutGoroutine(t *testing.T) {
	t.Parallel()

	s := sqlitestore.NewWithCleanupInterval(newDB(t), 0)

	assert.NotPanics(t, s.StopCleanup)
}
