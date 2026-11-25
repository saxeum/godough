package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes(t *testing.T) {
	// readyz needs a database and is covered in db_test.go.
	app := &application{logger: slog.New(slog.DiscardHandler)}
	mux := app.routes()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string // empty means: do not check the body
	}{
		{name: "healthz", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK, wantBody: "ok\n"},
		{name: "view snippet, bad id", method: http.MethodGet, path: "/api/snippet/view/abc", wantStatus: http.StatusNotFound, wantBody: `{"error":"Not Found"}`},
		{name: "view snippet, zero id", method: http.MethodGet, path: "/api/snippet/view/0", wantStatus: http.StatusNotFound, wantBody: `{"error":"Not Found"}`},
		{name: "view snippet, wrong method", method: http.MethodPost, path: "/api/snippet/view/7", wantStatus: http.StatusMethodNotAllowed},
		{name: "create post", method: http.MethodPost, path: "/api/snippet/create", wantStatus: http.StatusCreated, wantBody: `{"status":"created"}`},

		// The API owns the /api prefix and serves no files: the SPA and its
		// assets are the web container's job.
		{name: "unversioned api path", method: http.MethodGet, path: "/snippet/view/7", wantStatus: http.StatusNotFound},
		{name: "no ui at the root", method: http.MethodGet, path: "/", wantStatus: http.StatusNotFound},
		{name: "no static files", method: http.MethodGet, path: "/static/css/main.css", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s: status = %d, want %d", tt.method, tt.path, rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" {
				if got := rec.Body.String(); got != tt.wantBody {
					t.Errorf("%s %s: body = %q, want %q", tt.method, tt.path, got, tt.wantBody)
				}
			}
		})
	}
}

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		addr, dsn string
		wantErr   string
	}{
		{name: "both set", addr: ":4000", dsn: "postgres://x"},
		{name: "addr missing", dsn: "postgres://x", wantErr: "SNIPPET_ADDR is not set"},
		{name: "dsn missing", addr: ":4000", wantErr: "DATABASE_URL is not set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SNIPPET_ADDR", tt.addr)
			t.Setenv("DATABASE_URL", tt.dsn)

			addr, dsn, err := configFromEnv()

			switch {
			case tt.wantErr != "":
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			case addr != tt.addr || dsn != tt.dsn:
				t.Errorf("got (%q, %q), want (%q, %q)", addr, dsn, tt.addr, tt.dsn)
			}
		})
	}
}
