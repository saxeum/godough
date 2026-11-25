package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Needs a migrated database: make db-up db-migrate.
func TestSnippetsAll(t *testing.T) {
	if testing.Short() {
		t.Skip("database test: make db-up db-migrate, then make test")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is not set: copy .env.example to .env")
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

// readyz is the probe Kubernetes uses, so it must fail when the pool cannot
// serve, not just when the process is dead.
func TestReadyz(t *testing.T) {
	if testing.Short() {
		t.Skip("database test: make db-up db-migrate, then make test")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is not set: copy .env.example to .env")
	}

	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	app := &application{logger: slog.New(slog.DiscardHandler), db: pool}

	rec := httptest.NewRecorder()
	app.readyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("readyz with a live pool = %d, want %d", rec.Code, http.StatusOK)
	}

	pool.Close()

	rec = httptest.NewRecorder()
	app.readyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz with a closed pool = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestSnippetsIndex(t *testing.T) {
	pool := testPool(t)
	app := &application{logger: slog.New(slog.DiscardHandler), db: pool}

	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snippets", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var got []snippet
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not a JSON array of snippets: %v (body %q)", err, rec.Body.String())
	}
	if len(got) != 3 {
		t.Fatalf("len(snippets) = %d, want 3", len(got))
	}
	if got[0].Title != "First autumn morning" {
		t.Errorf("snippets[0].Title = %q, want the newest seed row", got[0].Title)
	}
}

func TestSnippetViewJSON(t *testing.T) {
	pool := testPool(t)
	app := &application{logger: slog.New(slog.DiscardHandler), db: pool}

	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snippet/view/1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got snippet
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not a JSON snippet: %v (body %q)", err, rec.Body.String())
	}
	if got.ID != 1 || got.Title != "An old silent pond" {
		t.Errorf("got id=%d title=%q, want id=1 title=%q", got.ID, got.Title, "An old silent pond")
	}

	// A well-formed id that does not exist must be a JSON 404, not a text one.
	rec = httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snippet/view/999", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing id: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("missing id: Content-Type = %q, want application/json", got)
	}
	if body := rec.Body.String(); body != `{"error":"Not Found"}` {
		t.Errorf("missing id: body = %q, want a JSON error", body)
	}
}

// testPool opens the database or fails: every test that calls it needs one.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testing.Short() {
		t.Skip("database test: make db-up db-migrate, then make test")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is not set: copy .env.example to .env")
	}

	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}
