package main

import (
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Needs a migrated database: make db-up db-migrate.
func TestSnippetsAll(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(pool.Close)

	app := &application{logger: slog.New(slog.DiscardHandler), db: pool}

	got, err := app.snippetsAll(t.Context())
	if err != nil {
		t.Fatalf("snippetsAll: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("len(snippets) = %d, want 3", len(got))
	}

	// created is identical for the seed rows, so the secondary id DESC decides.
	want := []string{
		"First autumn morning",
		"Over the wintry forest",
		"An old silent pond",
	}
	for i, s := range got {
		if s.Title != want[i] {
			t.Errorf("snippets[%d].Title = %q, want %q", i, s.Title, want[i])
		}
		if s.ID < 1 {
			t.Errorf("snippets[%d].ID = %d, want >= 1", i, s.ID)
		}
		if !s.Expires.After(s.Created) {
			t.Errorf("snippets[%d]: Expires %v is not after Created %v", i, s.Expires, s.Created)
		}
	}
}
