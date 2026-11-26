package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/saxeum/godough/internal/domain/snippet"
	"github.com/saxeum/godough/internal/service"
	"github.com/saxeum/godough/internal/testutil"
	"github.com/saxeum/godough/internal/transport/httpapi"
)

func newRepo() *testutil.SnippetRepo {
	return testutil.NewSnippetRepo(
		snippet.Snippet{ID: 2, Title: "newer", Content: "b"},
		snippet.Snippet{ID: 1, Title: "older", Content: "a"},
	)
}

func newRouter(t *testing.T, repo service.SnippetRepository) http.Handler {
	t.Helper()

	ready := func(context.Context) error { return nil }
	return httpapi.NewServer(slog.New(slog.DiscardHandler), service.NewSnippetService(repo), ready).Routes()
}

// newRouterWithLog keeps the log stream so a test can assert on the level, which
// is the difference between "a client left" and "the server is broken".
func newRouterWithLog(t *testing.T, repo service.SnippetRepository) (http.Handler, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ready := func(context.Context) error { return nil }
	return httpapi.NewServer(logger, service.NewSnippetService(repo), ready).Routes(), &logs
}

// do runs one request the way a well-behaved client would: a JSON content type
// whenever it carries a body.
func do(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	header := http.Header{}
	if body != "" {
		header.Set("Content-Type", "application/json")
	}
	return doWith(t, handler, method, path, body, header)
}

// doWith is do with the headers under the test's control, for the requests that
// are deliberately malformed.
func doWith(t *testing.T, handler http.Handler, method, path, body string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	for k, values := range header {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not the expected JSON: %v (body %q)", err, rec.Body.String())
	}
	return out
}

func TestListSnippets(t *testing.T) {
	rec := do(t, newRouter(t, newRepo()), http.MethodGet, "/api/snippet", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not a JSON array: %v (body %q)", err, rec.Body.String())
	}
	if len(got) != 2 {
		t.Fatalf("got %d snippets, want 2", len(got))
	}

	// The wire format is the spec's, not the Go struct's: camelCase keys.
	for _, key := range []string{"id", "title", "content", "created", "expires"} {
		if _, ok := got[0][key]; !ok {
			t.Errorf("snippet has no %q key, got %v", key, got[0])
		}
	}
	if _, ok := got[0]["Title"]; ok {
		t.Errorf("snippet has a PascalCase Title key; the spec says camelCase")
	}
}

func TestListSnippetsIsAnEmptyArrayNotNull(t *testing.T) {
	rec := do(t, newRouter(t, &testutil.SnippetRepo{}), http.MethodGet, "/api/snippet", "")

	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("empty list body = %q, want []", body)
	}
}

func TestGetSnippet(t *testing.T) {
	router := newRouter(t, newRepo())

	rec := do(t, router, http.MethodGet, "/api/snippet/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := decode[map[string]any](t, rec)
	if got["title"] != "older" {
		t.Errorf("title = %v, want older", got["title"])
	}

	rec = do(t, router, http.MethodGet, "/api/snippet/999", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing id: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := decode[map[string]string](t, rec); body["error"] != "Not Found" {
		t.Errorf("missing id: body = %v, want a Not Found error", body)
	}
}

// A well-formed integer that names no resource is a missing resource, not a
// malformed request: identity starts at 1, so 0 and -1 are 404.
func TestOutOfRangeIDIsNotFound(t *testing.T) {
	router := newRouter(t, newRepo())

	for _, path := range []string{"/api/snippet/0", "/api/snippet/-1"} {
		t.Run(path, func(t *testing.T) {
			if rec := do(t, router, http.MethodGet, path, ""); rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

func TestMalformedIDIsABadRequest(t *testing.T) {
	router := newRouter(t, newRepo())

	// Not an integer at all, so the router rejects it before any handler runs.
	for _, path := range []string{"/api/snippet/abc", "/api/snippet/1.5", "/api/snippet/1x"} {
		t.Run(path, func(t *testing.T) {
			rec := do(t, router, http.MethodGet, path, "")

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if body := decode[map[string]string](t, rec); body["error"] != "Bad Request" {
				t.Errorf("body = %v, want a Bad Request error", body)
			}
		})
	}
}

func TestCreateSnippet(t *testing.T) {
	router := newRouter(t, newRepo())

	rec := do(t, router, http.MethodPost, "/api/snippet",
		`{"title":"a new haiku","content":"first line\nsecond line","expiresInDays":7}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if got, want := rec.Header().Get("Location"), "/api/snippet/3"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	created := decode[map[string]any](t, rec)
	if created["title"] != "a new haiku" {
		t.Errorf("title = %v, want %q", created["title"], "a new haiku")
	}
	if created["content"] != "first line\nsecond line" {
		t.Errorf("content = %v, want the posted value", created["content"])
	}
}

func TestCreateSnippetRejects(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		want     int
		wantKeys []string
	}{
		{name: "not json", body: `not json`, want: http.StatusBadRequest},
		// null decodes into zero values without an error, so it fails validation
		// rather than decoding: the client sent a body, it just was not a valid one.
		{name: "null body", body: `null`, want: http.StatusUnprocessableEntity, wantKeys: []string{"content", "expiresInDays", "title"}},
		{name: "empty", body: `{}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"content", "expiresInDays", "title"}},
		{name: "empty title", body: `{"title":"","content":"x","expiresInDays":7}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"title"}},
		{name: "title too long", body: `{"title":"` + strings.Repeat("a", 101) + `","content":"x","expiresInDays":7}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"title"}},
		{name: "expiry too long", body: `{"title":"t","content":"x","expiresInDays":366}`, want: http.StatusUnprocessableEntity, wantKeys: []string{"expiresInDays"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newRouter(t, newRepo()), http.MethodPost, "/api/snippet", tt.body)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.want, rec.Body.String())
			}
			if len(tt.wantKeys) == 0 {
				return
			}

			got := decode[struct {
				Errors map[string]string `json:"errors"`
			}](t, rec)
			keys := make([]string, 0, len(got.Errors))
			for k := range got.Errors {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			if !slices.Equal(keys, tt.wantKeys) {
				t.Errorf("errors = %v, want %v", keys, tt.wantKeys)
			}
		})
	}
}

func TestDeleteSnippet(t *testing.T) {
	repo := newRepo()
	router := newRouter(t, repo)

	rec := do(t, router, http.MethodDelete, "/api/snippet/1", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}

	// Gone.
	if rec := do(t, router, http.MethodGet, "/api/snippet/1", ""); rec.Code != http.StatusNotFound {
		t.Errorf("read after delete: status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// Deleting it again did nothing, so it is a 404 rather than a second 204.
	if rec := do(t, router, http.MethodDelete, "/api/snippet/1", ""); rec.Code != http.StatusNotFound {
		t.Errorf("second delete: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUnexpectedRepositoryErrorIsAJSON500(t *testing.T) {
	repo := newRepo()
	repo.Err = errors.New("connection refused")
	router := newRouter(t, repo)

	rec := do(t, router, http.MethodGet, "/api/snippet", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	// The body must not name the cause.
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("body leaks the internal error: %q", rec.Body.String())
	}
	if body := decode[map[string]string](t, rec); body["error"] != "internal server error" {
		t.Errorf("body = %v, want the generic message", body)
	}
}

func TestUnknownRouteAndWrongMethod(t *testing.T) {
	router := newRouter(t, newRepo())

	// The two responses the router writes itself. They are text/plain, which is
	// the exception recorded in spec/openapi.yaml; asserting it here is what stops
	// it becoming a different exception quietly.
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{name: "unknown api path", method: http.MethodGet, path: "/api/snippets", want: http.StatusNotFound},
		{name: "wrong method", method: http.MethodPost, path: "/api/snippet/1", want: http.StatusMethodNotAllowed},
		{name: "root", method: http.MethodGet, path: "/", want: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, router, tt.method, tt.path, "")

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
				t.Errorf("Content-Type = %q, want text/plain — the recorded exception", got)
			}
		})
	}
}

func TestProbes(t *testing.T) {
	router := newRouter(t, newRepo())

	rec := do(t, router, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok\n" {
		t.Errorf("healthz = %d %q, want 200 %q", rec.Code, rec.Body.String(), "ok\n")
	}

	rec = do(t, router, http.MethodGet, "/readyz", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok\n" {
		t.Errorf("readyz = %d %q, want 200 %q", rec.Code, rec.Body.String(), "ok\n")
	}
}

func TestReadyzFailsWhenTheDatabaseDoesNot(t *testing.T) {
	ready := func(context.Context) error { return errors.New("pool closed") }
	router := httpapi.NewServer(
		slog.New(slog.DiscardHandler),
		service.NewSnippetService(newRepo()),
		ready,
	).Routes()

	rec := do(t, router, http.MethodGet, "/readyz", "")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

// The body limit has to live in the application, because the generated handler
// reads r.Body before any of its own middleware runs. nginx enforces the same
// number, which is exactly why this was invisible: the request never reached the
// pod in development.
func TestBodyLimit(t *testing.T) {
	router := newRouter(t, newRepo())

	oversized := `{"title":"big","content":"` + strings.Repeat("a", 2<<20) + `","expiresInDays":7}`
	rec := do(t, router, http.MethodPost, "/api/snippet", oversized)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if body := decode[map[string]string](t, rec); body["error"] != "Request Entity Too Large" {
		t.Errorf("body = %v, want the JSON error shape", body)
	}
}

func TestContentTypeIsRequired(t *testing.T) {
	const valid = `{"title":"t","content":"c","expiresInDays":7}`

	tests := []struct {
		name        string
		contentType string
		want        int
	}{
		{name: "absent", contentType: "", want: http.StatusUnsupportedMediaType},
		{name: "text/plain", contentType: "text/plain", want: http.StatusUnsupportedMediaType},
		// The three a cross-site form can send without a preflight.
		{name: "form encoded", contentType: "application/x-www-form-urlencoded", want: http.StatusUnsupportedMediaType},
		{name: "multipart", contentType: "multipart/form-data; boundary=x", want: http.StatusUnsupportedMediaType},
		{name: "json", contentType: "application/json", want: http.StatusCreated},
		{name: "json with charset", contentType: "application/json; charset=utf-8", want: http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.contentType != "" {
				header.Set("Content-Type", tt.contentType)
			}
			rec := doWith(t, newRouter(t, newRepo()), http.MethodPost, "/api/snippet", valid, header)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

// A client that navigates away mid-request cancels the context, and the
// cancellation comes back up through the layers as an error. It is not a fault,
// and counting it as one makes the error rate measure browsers.
func TestCancelledRequestIsNotAServerError(t *testing.T) {
	repo := newRepo()
	repo.Err = fmt.Errorf("query snippets: %w", context.Canceled)
	router, logs := newRouterWithLog(t, repo)

	rec := do(t, router, http.MethodGet, "/api/snippet", "")

	if rec.Body.Len() != 0 {
		t.Errorf("wrote %q to an abandoned request, want nothing", rec.Body.String())
	}
	if got := logs.String(); strings.Contains(got, "level=ERROR") {
		t.Errorf("logged a cancellation at ERROR:\n%s", got)
	}
	if got := logs.String(); !strings.Contains(got, "abandoned") {
		t.Errorf("expected an info line about the abandoned request, got:\n%s", got)
	}
}
