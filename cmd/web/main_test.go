package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestRoutes(t *testing.T) {
	ui := fstest.MapFS{
		"index.html":          &fstest.MapFile{Data: []byte("<h1>snippetbox</h1>")},
		"static/index.html":   &fstest.MapFile{Data: []byte("<p>static</p>")},
		"static/css/main.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string // empty means: do not check the body
	}{
		{name: "spa index", method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: "<h1>snippetbox</h1>"},
		{name: "static file", method: http.MethodGet, path: "/static/css/main.css", wantStatus: http.StatusOK, wantBody: "body{}"},
		{name: "directory serves its index", method: http.MethodGet, path: "/static/", wantStatus: http.StatusOK, wantBody: "<p>static</p>"},
		{name: "unknown asset", method: http.MethodGet, path: "/nope", wantStatus: http.StatusNotFound},
		{name: "view snippet", method: http.MethodGet, path: "/snippet/view/7", wantStatus: http.StatusOK, wantBody: "Display a snippet with ID 7..."},
		{name: "view snippet, bad id", method: http.MethodGet, path: "/snippet/view/abc", wantStatus: http.StatusNotFound},
		{name: "view snippet, zero id", method: http.MethodGet, path: "/snippet/view/0", wantStatus: http.StatusNotFound},
		{name: "create form", method: http.MethodGet, path: "/snippet/create", wantStatus: http.StatusOK, wantBody: "Display a form to create a new snippet..."},
		{name: "create post", method: http.MethodPost, path: "/snippet/create", wantStatus: http.StatusCreated, wantBody: "Save a new snippet..."},
	}

	app := &application{logger: slog.New(slog.DiscardHandler)}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.routes(ui).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

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
