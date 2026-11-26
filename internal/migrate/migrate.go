// Package migrate applies the pending files in a migrations directory, in order,
// each in its own transaction, recording what it applied.
//
// Not supported, deliberately: a down migration, checksums of already-applied
// files, and out-of-order detection. Those are a migration framework's job. This
// exists so that a second migration is a new file rather than an edit to a Make
// target.
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration is one file: its version, taken from the numeric prefix of the name.
type Migration struct {
	Version int64
	Name    string
	SQL     string
}

const createTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    bigint PRIMARY KEY,
    name       text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
)`

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

		migrations = append(migrations, Migration{Version: version, Name: name, SQL: string(body)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })

	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version == migrations[i-1].Version {
			return nil, fmt.Errorf("%s and %s share version %d", migrations[i-1].Name, migrations[i].Name, migrations[i].Version)
		}
	}

	return migrations, nil
}

// Pending returns the migrations whose version is not in applied, in order.
func Pending(all []Migration, applied map[int64]bool) []Migration {
	var pending []Migration
	for _, m := range all {
		if !applied[m.Version] {
			pending = append(pending, m)
		}
	}
	return pending
}

// Apply runs every pending migration. Each file runs in its own transaction
// together with the row that records it, so a failure leaves neither a
// half-applied schema nor a version claimed as done.
func Apply(ctx context.Context, pool *pgxpool.Pool, all []Migration) ([]Migration, error) {
	if _, err := pool.Exec(ctx, createTable); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return nil, err
	}

	pending := Pending(all, applied)
	for _, m := range pending {
		if err := applyOne(ctx, pool, m); err != nil {
			return nil, err
		}
	}

	return pending, nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int64]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}

	applied := make(map[int64]bool)
	versions, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	for _, v := range versions {
		applied[v] = true
	}

	return applied, nil
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, m Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", m.Name, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return fmt.Errorf("%s: %w", m.Name, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name); err != nil {
		return fmt.Errorf("record %s: %w", m.Name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", m.Name, err)
	}

	return nil
}
