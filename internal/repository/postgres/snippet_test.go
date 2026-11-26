package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saxeum/godough/internal/domain/snippet"
)

// testPool opens the database the DSN names, or fails: a test that silently
// skips when the database is absent would report green for work it never did.
//
// It goes through NewPool rather than pgxpool.New so the tests run against the
// same session configuration the process does. A test pool built another way
// would pass while production ran with a different statement_timeout.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testing.Short() {
		t.Skip("database test: make db-up db-migrate, then make test")
	}

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is not set: run make db-test")
	}

	p, err := NewPool(t.Context(), dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(p.Close)

	return p
}

// TestInsertGetDelete walks the repository's own round trip. It creates the row
// it asserts on, so it does not depend on which seed rows the database happens
// to hold.
func TestInsertGetDelete(t *testing.T) {
	repo := NewSnippetRepository(testPool(t))
	ctx := t.Context()

	created, err := repo.Insert(ctx, snippet.New{Title: "repository round trip", Content: "first\nsecond", ExpiresInDays: 7})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), created.ID) })

	if created.ID < 1 {
		t.Fatalf("created.ID = %d, want >= 1", created.ID)
	}
	if !created.Expires.After(created.Created) {
		t.Errorf("Expires %v is not after Created %v", created.Expires, created.Created)
	}
	if created.Title != "repository round trip" {
		t.Errorf("Title = %q, want the inserted value", created.Title)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != created {
		t.Errorf("Get returned %+v, want the inserted row %+v", got, created)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.Get(ctx, created.ID); !errors.Is(err, snippet.ErrNotFound) {
		t.Errorf("get after delete: err = %v, want ErrNotFound", err)
	}
	// The second delete matched nothing; that is not a failure, it is a 404.
	if err := repo.Delete(ctx, created.ID); !errors.Is(err, snippet.ErrNotFound) {
		t.Errorf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestListIsNewestFirstAndIncludesTheNewRow(t *testing.T) {
	repo := NewSnippetRepository(testPool(t))
	ctx := t.Context()

	before, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	created, err := repo.Insert(ctx, snippet.New{Title: "list ordering probe", Content: "x", ExpiresInDays: 1})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), created.ID) })

	after, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(after) != len(before)+1 {
		t.Fatalf("list grew from %d to %d, want one more", len(before), len(after))
	}
	// Newest first: the row just written is at the front, not merely present.
	if after[0].ID != created.ID {
		t.Errorf("first row is id %d, want the row just created (%d)", after[0].ID, created.ID)
	}

	for i := 1; i < len(after); i++ {
		if after[i].Created.After(after[i-1].Created) {
			t.Errorf("row %d is newer than row %d; the list is not newest-first", i, i-1)
		}
	}
}

// TestListIsAnAllocatedSliceNotNil guards the JSON: a nil slice marshals to
// null, and the contract says an empty list is [].
func TestListIsAnAllocatedSliceNotNil(t *testing.T) {
	repo := NewSnippetRepository(testPool(t))

	items, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if items == nil {
		t.Error("List returned nil, which marshals to null; want an allocated slice")
	}
}

// AfterConnect has to run for every connection, including those opened later.
// Two held at once proves it is a per-connection setting and not a one-off
// applied to whichever connection happened to be created first.
func TestSessionLimitsAreSetOnEveryConnection(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()

	first, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer first.Release()

	second, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire a second connection: %v", err)
	}
	defer second.Release()

	for i, conn := range []*pgxpool.Conn{first, second} {
		var statement, lock string
		if err := conn.QueryRow(ctx, "SHOW statement_timeout").Scan(&statement); err != nil {
			t.Fatalf("connection %d: show statement_timeout: %v", i, err)
		}
		if err := conn.QueryRow(ctx, "SHOW lock_timeout").Scan(&lock); err != nil {
			t.Fatalf("connection %d: show lock_timeout: %v", i, err)
		}

		if statement != StatementTimeout.String() {
			t.Errorf("connection %d: statement_timeout = %q, want %q", i, statement, StatementTimeout)
		}
		if lock != LockTimeout.String() {
			t.Errorf("connection %d: lock_timeout = %q, want %q", i, lock, LockTimeout)
		}
	}
}
