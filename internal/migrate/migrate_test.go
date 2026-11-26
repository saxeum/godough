package migrate

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDiscoverOrdersByVersionNotByName(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_second.sql": {Data: []byte("select 2;")},
		"0010_tenth.sql":  {Data: []byte("select 10;")},
		"0001_first.sql":  {Data: []byte("select 1;")},
		"notes.txt":       {Data: []byte("ignored")},
	}

	got, err := Discover(fsys, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	var versions []int64
	for _, m := range got {
		versions = append(versions, m.Version)
	}
	if want := []int64{1, 2, 10}; !slices.Equal(versions, want) {
		t.Errorf("versions = %v, want %v — string order would put 10 before 2", versions, want)
	}
	if len(got) != 3 {
		t.Errorf("got %d migrations, want 3: a non-.sql file is not one", len(got))
	}
}

func TestDiscoverRejectsBadNames(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "no prefix", file: "create_snippets.sql"},
		{name: "not a number", file: "abc_create.sql"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Discover(fstest.MapFS{tt.file: {Data: []byte("select 1;")}}, "."); err == nil {
				t.Fatalf("Discover accepted %q, want an error", tt.file)
			}
		})
	}
}

func TestDiscoverRejectsDuplicateVersions(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_a.sql": {Data: []byte("select 1;")},
		"0001_b.sql": {Data: []byte("select 2;")},
	}

	if _, err := Discover(fsys, "."); err == nil {
		t.Fatal("two files with version 1 were accepted; which one ran first would be undefined")
	}
}

func TestPending(t *testing.T) {
	all := []Migration{{Version: 1}, {Version: 2}, {Version: 3}}

	got := Pending(all, map[int64]Recorded{1: {Name: "0001_first.sql"}})
	if want := []int64{2, 3}; !slices.Equal(versions(got), want) {
		t.Errorf("pending = %v, want %v", versions(got), want)
	}

	inOrder := map[int64]Recorded{
		1: {Name: "0001_first.sql"},
		2: {Name: "0002_second.sql"},
		3: {Name: "0003_third.sql"},
	}
	if got := Pending(all, inOrder); len(got) != 0 {
		t.Errorf("pending = %v, want none", versions(got))
	}

	// A version recorded in the database that no longer has a file is not an
	// error here: files are never removed, and detecting it is out of scope.
	if got := Pending(all, map[int64]Recorded{99: {Name: "0099_gone.sql"}}); len(got) != 3 {
		t.Errorf("pending = %v, want all three", versions(got))
	}
}

func TestChecksumsRecordedAndCompared(t *testing.T) {
	fsys := fstest.MapFS{"0001_one.sql": {Data: []byte("create table one (id int);")}}

	first, err := Discover(fsys, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	again, err := Discover(fsys, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if first[0].Checksum == "" || first[0].Checksum != again[0].Checksum {
		t.Fatalf("checksums = %q and %q: one file must hash to one value", first[0].Checksum, again[0].Checksum)
	}

	// One byte matters: the file is the record of what ran, so reformatting it
	// is a change like any other.
	fsys["0001_one.sql"] = &fstest.MapFile{Data: []byte("create table one (id int) ;")}
	edited, err := Discover(fsys, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if edited[0].Checksum == first[0].Checksum {
		t.Fatal("an edited file kept its checksum")
	}
}

func TestApplyRefusesAMigrationThatChanged(t *testing.T) {
	pool := scratchDatabase(t)
	fsys := fstest.MapFS{"0001_one.sql": {Data: []byte("create table one (id int);")}}

	all, err := Discover(fsys, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if _, err := Apply(t.Context(), pool, all); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	fsys["0001_one.sql"] = &fstest.MapFile{Data: []byte("create table one (id int, extra int);")}
	edited, err := Discover(fsys, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if _, err := Apply(t.Context(), pool, edited); err == nil {
		t.Fatal("an edited migration was accepted: the database and the file now disagree about what ran")
	} else if !strings.Contains(err.Error(), "has changed since it was applied") {
		t.Fatalf("err = %v, want a drift error naming the file", err)
	}
}

func TestApplyRefusesToRunWhileAnotherHoldsTheLock(t *testing.T) {
	pool := scratchDatabase(t)

	release, err := lock(t.Context(), pool)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	// A failure below leaves before the explicit release, and a checked-out
	// connection makes pool.Close block forever, so the test would hang instead
	// of reporting. Register it, and let the idempotent release handle both.
	t.Cleanup(release)

	if _, err := Apply(t.Context(), pool, nil); !errors.Is(err, ErrLocked) {
		t.Fatalf("Apply with the lock held: err = %v, want ErrLocked; two runners must not both apply", err)
	}

	release()

	if _, err := Apply(t.Context(), pool, nil); err != nil {
		t.Fatalf("Apply after the lock was released: %v", err)
	}
}

func TestApplyBackfillsTheChecksumOfAnOlderRow(t *testing.T) {
	pool := scratchDatabase(t)

	all, err := Discover(fstest.MapFS{"0001_one.sql": {Data: []byte("create table one (id int);")}}, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if _, err := Apply(t.Context(), pool, all); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// What a runner without checksums left behind.
	if _, err := pool.Exec(t.Context(), "UPDATE schema_migrations SET checksum = NULL"); err != nil {
		t.Fatalf("clear checksums: %v", err)
	}

	report, err := Apply(t.Context(), pool, all)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Backfilled) != 1 || report.Backfilled[0].Name != "0001_one.sql" {
		t.Fatalf("backfilled = %v, want 0001_one.sql: the row stays exempt from drift detection otherwise", report.Backfilled)
	}

	var stored *string
	if err := pool.QueryRow(t.Context(), "SELECT checksum FROM schema_migrations WHERE version = 1").Scan(&stored); err != nil {
		t.Fatalf("read checksum: %v", err)
	}
	if stored == nil || *stored != all[0].Checksum {
		t.Fatalf("checksum = %v, want %s", stored, all[0].Checksum)
	}
}

func TestStatusWritesNothing(t *testing.T) {
	pool := scratchDatabase(t)

	all, err := Discover(fstest.MapFS{"0001_one.sql": {Data: []byte("create table one (id int);")}}, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	report, err := Status(t.Context(), pool, all)
	if err != nil {
		t.Fatalf("Status on a database with no migrations: %v", err)
	}
	if len(report.Pending) != 1 {
		t.Fatalf("pending = %v, want the one migration", report.Pending)
	}

	// The table must still not exist: Status is what an operator runs against a
	// database it does not own, so it may not create anything.
	var table *string
	if err := pool.QueryRow(t.Context(), "SELECT to_regclass('schema_migrations')::text").Scan(&table); err != nil {
		t.Fatalf("look for schema_migrations: %v", err)
	}
	if table != nil {
		t.Fatal("Status created schema_migrations; it must not write")
	}
}

// scratchDatabase gives one test its own database, so a test that migrates does
// not collide with another package and does not leave state behind for the next
// run. It is the owner DSN because migrations create tables.
func scratchDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testing.Short() {
		t.Skip("database test: make db-up db-test, then make test")
	}

	owner := os.Getenv("TEST_DATABASE_URL_OWNER")
	if owner == "" {
		t.Fatal("TEST_DATABASE_URL_OWNER is not set: run make db-test")
	}

	ctx := t.Context()

	admin, err := pgxpool.New(ctx, owner)
	if err != nil {
		t.Fatalf("open %s: %v", owner, err)
	}
	if err := admin.Ping(ctx); err != nil {
		t.Fatalf("ping %s: %v", owner, err)
	}

	name := "migrate_test_" + strings.ToLower(strings.NewReplacer("/", "_", "-", "_").Replace(t.Name()))
	if len(name) > 63 {
		name = name[:63]
	}

	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name); err != nil {
		t.Fatalf("drop %s: %v", name, err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name); err != nil {
			t.Logf("drop %s: %v", name, err)
		}
		admin.Close()
	})

	cfg, err := pgxpool.ParseConfig(owner)
	if err != nil {
		t.Fatalf("parse the owner DSN: %v", err)
	}
	cfg.ConnConfig.Database = name

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	// Registered after the drop, so it closes first: the drop fails while a
	// connection is still open.
	t.Cleanup(pool.Close)

	return pool
}

func versions(ms []Migration) []int64 {
	var out []int64
	for _, m := range ms {
		out = append(out, m.Version)
	}
	return out
}
