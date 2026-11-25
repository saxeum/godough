package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestCreateSnippet(t *testing.T) {
	pool := testPool(t)
	app := &application{logger: slog.New(slog.DiscardHandler), db: pool}

	body := `{"Title":"a new haiku","Content":"first line\nsecond line","ExpiresInDays":7}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/snippet/create", strings.NewReader(body))
	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var created snippet
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("body is not a JSON snippet: %v (body %q)", err, rec.Body.String())
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM snippets WHERE id = $1", created.ID) })

	if created.ID < 1 {
		t.Fatalf("created.ID = %d, want >= 1", created.ID)
	}
	if want := fmt.Sprintf("/api/snippet/view/%d", created.ID); rec.Header().Get("Location") != want {
		t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
	}
	if !created.Expires.After(created.Created) {
		t.Errorf("Expires %v is not after Created %v", created.Expires, created.Created)
	}

	// The write has to be visible on the read path, not just in the response.
	rec = httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/snippet/view/%d", created.ID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("read back: status = %d, want %d", rec.Code, http.StatusOK)
	}
	var read snippet
	if err := json.Unmarshal(rec.Body.Bytes(), &read); err != nil {
		t.Fatalf("read back is not JSON: %v", err)
	}
	if read.Title != "a new haiku" || read.Content != "first line\nsecond line" {
		t.Errorf("read back = %+v, want the values that were posted", read)
	}
}

func TestCreateSnippetRejects(t *testing.T) {
	pool := testPool(t)
	app := &application{logger: slog.New(slog.DiscardHandler), db: pool}

	before := snippetCount(t, pool)

	tests := []struct {
		name     string
		body     string
		want     int
		wantKeys []string
	}{
		{name: "not json", body: `not json`, want: http.StatusBadRequest},
		{name: "empty title", body: `{"Title":"","Content":"x","ExpiresInDays":7}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"title"}},
		{name: "title too long", body: `{"Title":"` + strings.Repeat("a", 101) + `","Content":"x","ExpiresInDays":7}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"title"}},
		{name: "empty content", body: `{"Title":"t","Content":"   ","ExpiresInDays":7}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"content"}},
		{name: "expiry zero", body: `{"Title":"t","Content":"x","ExpiresInDays":0}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"expiresInDays"}},
		{name: "expiry too long", body: `{"Title":"t","Content":"x","ExpiresInDays":366}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"expiresInDays"}},
		{name: "everything missing", body: `{}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"title", "content", "expiresInDays"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/snippet/create", strings.NewReader(tt.body))
			app.routes().ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.want, rec.Body.String())
			}

			if len(tt.wantKeys) == 0 {
				return
			}

			var got struct {
				Errors map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not a JSON error map: %v (body %q)", err, rec.Body.String())
			}
			for _, k := range tt.wantKeys {
				if _, ok := got.Errors[k]; !ok {
					t.Errorf("errors has no %q, got %v", k, got.Errors)
				}
			}
		})
	}

	// A rejected request must not have written anything.
	if after := snippetCount(t, pool); after != before {
		t.Errorf("row count changed from %d to %d: a rejected request inserted", before, after)
	}
}

func snippetCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM snippets").Scan(&n); err != nil {
		t.Fatalf("count snippets: %v", err)
	}
	return n
}

func TestDeleteSnippet(t *testing.T) {
	pool := testPool(t)
	app := &application{logger: slog.New(slog.DiscardHandler), db: pool}

	// Make a row to delete rather than deleting a seed row: this test must not
	// depend on which rows happen to be in the database.
	created, err := app.snippetInsert(t.Context(), "delete me", "x", 7)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	before := snippetCount(t, pool)

	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/snippet/view/%d", created.ID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status = %d, want %d (body %q)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("delete: body = %q, want empty", body)
	}
	if after := snippetCount(t, pool); after != before-1 {
		t.Errorf("row count %d -> %d, want one fewer", before, after)
	}

	// Gone: the read path agrees.
	rec = httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/snippet/view/%d", created.ID), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("read after delete: status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// Deleting it again is a 404, not a 204: the second call did nothing.
	rec = httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/snippet/view/%d", created.ID), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("second delete: status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// A malformed id is a 404, not a 500.
	rec = httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/snippet/view/abc", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("malformed id: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
