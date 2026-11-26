// Package migrate applies the pending files in a migrations directory, in order,
// each in its own transaction, recording what it applied.
//
// Deliberately not supported: down migrations, out-of-order detection, and a
// target version. This exists so that a second migration is a new file rather
// than an edit to a Make target.
//
// Two things are defended against, because both are silent corruption rather
// than a loud failure: two runners at once, which is a session advisory lock,
// and an applied file whose content has since changed, which is a SHA-256
// recorded next to the version when it ran.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration is one file: its version, taken from the numeric prefix of the name,
// and the checksum of its content.
type Migration struct {
	Version  int64
	Name     string
	SQL      string
	Checksum string
}

// Recorded is what schema_migrations says about one version. Checksum is empty
// for a row written before checksums existed.
type Recorded struct {
	Name     string
	Checksum string
}

// Report is what a run found and what it did. Status writes nothing, so its
// Backfilled slice is always empty.
type Report struct {
	Applied    []Migration // run by this process, and recorded
	Backfilled []Migration // applied before checksums existed; the checksum is now recorded
	Pending    []Migration // not recorded yet
	Changed    []Migration // recorded, but the file on disk is no longer the file that ran
}

// ErrLocked means another runner holds the migration lock. It is a distinct
// error because it is not a failure of the migration: waiting or retrying is the
// right response, and a Job with backoffLimit 0 should be re-run, not debugged.
var ErrLocked = errors.New("another migration is running")

const createTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    bigint PRIMARY KEY,
    name       text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
)`

// addChecksum is separate from createTable so that a database migrated by an
// older runner gains the column on its next run instead of needing a hand-ALTER.
const addChecksum = `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum text`

// lockKey is arbitrary but fixed. It is "godough" in hex, so it is unlikely to
// collide with another application on the same database by accident.
const lockKey = int64(0x676f646f756768)

// lockTimeout bounds a migration that is waiting behind another session's lock.
// Without it an ALTER TABLE waits forever, and because it queues at the head of
// the lock queue it stalls every reader behind it too.
const lockTimeout = 30 * time.Second

// Discover reads dir and returns its migrations ordered by version.
func Discover(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	var migrations []Migration

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		name := entry.Name()
		prefix, _, found := strings.Cut(name, "_")
		if !found {
			return nil, fmt.Errorf("%s: want a numeric prefix and an underscore, like 0001_create_snippets.sql", name)
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a number", name, prefix)
		}

		body, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			SQL:      string(body),
			Checksum: checksum(body),
		})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })

	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version == migrations[i-1].Version {
			return nil, fmt.Errorf("%s and %s share version %d", migrations[i-1].Name, migrations[i].Name, migrations[i].Version)
		}
	}

	return migrations, nil
}

// checksum is the hex SHA-256 of a file's bytes, so a reformatting counts as a
// change. That is the point: the file is the record of what ran.
func checksum(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// Pending returns the migrations whose version is not in applied, in order.
func Pending(all []Migration, applied map[int64]Recorded) []Migration {
	var pending []Migration
	for _, m := range all {
		if _, ok := applied[m.Version]; !ok {
			pending = append(pending, m)
		}
	}
	return pending
}

// changedSince returns the migrations whose recorded checksum no longer matches
// the file on disk. A row with no checksum was applied before checksums existed
// and cannot be judged, so it is not reported here.
func changedSince(all []Migration, applied map[int64]Recorded) []Migration {
	var changed []Migration
	for _, m := range all {
		r, ok := applied[m.Version]
		if !ok || r.Checksum == "" {
			continue
		}
		if r.Checksum != m.Checksum {
			changed = append(changed, m)
		}
	}
	return changed
}

// Apply runs every pending migration, in its own transaction together with the
// row that records it, so a failure leaves neither a half-applied schema nor a
// version claimed as done.
func Apply(ctx context.Context, pool *pgxpool.Pool, all []Migration) (Report, error) {
	release, err := lock(ctx, pool)
	if err != nil {
		return Report{}, err
	}
	defer release()

	if _, err := pool.Exec(ctx, createTable); err != nil {
		return Report{}, fmt.Errorf("create schema_migrations: %w", err)
	}
	if _, err := pool.Exec(ctx, addChecksum); err != nil {
		return Report{}, fmt.Errorf("add schema_migrations.checksum: %w", err)
	}

	applied, err := readApplied(ctx, pool, true)
	if err != nil {
		return Report{}, err
	}

	var report Report

	if drifted := changedSince(all, applied); len(drifted) > 0 {
		return Report{}, fmt.Errorf("%s has changed since it was applied (recorded %s, now %s): a migration that has run is immutable, so add a new file instead",
			drifted[0].Name, short(applied[drifted[0].Version].Checksum), short(drifted[0].Checksum))
	}

	// A row written before checksums existed adopts the checksum of the file on
	// disk. That trusts the file, which is the only option: the content that ran
	// was never recorded, and leaving the row NULL would keep it exempt from
	// drift detection forever.
	for _, m := range all {
		if r, ok := applied[m.Version]; ok && r.Checksum == "" {
			if _, err := pool.Exec(ctx, `UPDATE schema_migrations SET checksum = $1 WHERE version = $2 AND checksum IS NULL`, m.Checksum, m.Version); err != nil {
				return Report{}, fmt.Errorf("record checksum for %s: %w", m.Name, err)
			}
			report.Backfilled = append(report.Backfilled, m)
		}
	}

	report.Pending = Pending(all, applied)
	for _, m := range report.Pending {
		if err := applyOne(ctx, pool, m); err != nil {
			return Report{}, err
		}
		report.Applied = append(report.Applied, m)
	}

	return report, nil
}

// Status reports applied, pending and changed versions and writes nothing.
func Status(ctx context.Context, pool *pgxpool.Pool, all []Migration) (Report, error) {
	applied, err := readApplied(ctx, pool, false)
	if err != nil {
		return Report{}, err
	}

	// "Applied" here means recorded and still identical on disk; a file that has
	// changed is reported once, in Changed, rather than in both.
	drifted := changedSince(all, applied)
	isDrifted := make(map[int64]bool, len(drifted))
	for _, m := range drifted {
		isDrifted[m.Version] = true
	}
	unchanged := make(map[int64]Recorded, len(applied))
	for version, r := range applied {
		if !isDrifted[version] {
			unchanged[version] = r
		}
	}

	report := Report{Changed: drifted}
	for _, m := range all {
		if _, ok := unchanged[m.Version]; ok {
			report.Applied = append(report.Applied, m)
		}
	}
	report.Pending = Pending(all, applied)

	return report, nil
}

// lock takes the migration advisory lock for as long as the returned release
// function is not called, on a connection it holds rather than on one the pool
// may hand to something else.
//
// Session-level pg_try_advisory_lock rather than a lock on a table: there is no
// row to hold, and the lock disappears with the session if this process is
// killed. Nothing waits, so a second runner gets ErrLocked immediately instead
// of hanging behind the first.
func lock(ctx context.Context, pool *pgxpool.Pool) (func(), error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire a connection for the migration lock: %w", err)
	}

	var got bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&got); err != nil {
		conn.Release()
		return nil, fmt.Errorf("take the migration lock: %w", err)
	}
	if !got {
		conn.Release()
		return nil, fmt.Errorf("%w: the advisory lock is held by another session; wait for it to finish and run again", ErrLocked)
	}

	// Releasing twice must not panic or unlock someone else's lock: callers write
	// `defer release()` and also release explicitly when they want the lock gone
	// before the function returns.
	var once sync.Once

	return func() {
		once.Do(func() {
			// The signal context is already cancelled on SIGTERM, and a cancelled
			// context cannot unlock. The lock dies with the session anyway; a
			// clean unlock is what keeps the connection reusable.
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()

			if _, err := conn.Exec(releaseCtx, "SELECT pg_advisory_unlock($1)", lockKey); err != nil {
				// Nothing to do about it: the pool discards the connection and
				// the server drops the lock with the session.
				_ = conn.Conn().Close(releaseCtx)
			}
			conn.Release()
		})
	}, nil
}

// readApplied reads schema_migrations, or an empty map when the table is absent
// and creating it is not allowed. Status must not write, so it passes false.
func readApplied(ctx context.Context, pool *pgxpool.Pool, tableExists bool) (map[int64]Recorded, error) {
	if !tableExists {
		var name *string
		if err := pool.QueryRow(ctx, "SELECT to_regclass('schema_migrations')::text").Scan(&name); err != nil {
			return nil, fmt.Errorf("look for schema_migrations: %w", err)
		}
		if name == nil {
			return map[int64]Recorded{}, nil
		}
	}

	rows, err := pool.Query(ctx, "SELECT version, name, checksum FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]Recorded)
	for rows.Next() {
		var (
			version int64
			name    string
			stored  *string
		)
		if err := rows.Scan(&version, &name, &stored); err != nil {
			return nil, fmt.Errorf("read schema_migrations: %w", err)
		}

		r := Recorded{Name: name}
		if stored != nil {
			r.Checksum = *stored
		}
		applied[version] = r
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}

	return applied, nil
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, m Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", m.Name, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%s'", lockTimeout)); err != nil {
		return fmt.Errorf("set lock_timeout for %s: %w", m.Name, err)
	}
	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return fmt.Errorf("%s: %w", m.Name, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)", m.Version, m.Name, m.Checksum); err != nil {
		return fmt.Errorf("record %s: %w", m.Name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", m.Name, err)
	}

	return nil
}

// short is the first twelve hex characters of a checksum: enough to tell two
// apart in an error message, short enough to read.
func short(sum string) string {
	if len(sum) <= 12 {
		return sum
	}
	return sum[:12]
}
