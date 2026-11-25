package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestNewMux(t *testing.T) {
	ui := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<h1>snippetbox</h1>")},
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string // empty means: do not check the body
	}{
		{name: "spa index", method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: "<h1>snippetbox</h1>"},
		{name: "unknown asset", method: http.MethodGet, path: "/nope", wantStatus: http.StatusNotFound},
		{name: "view snippet", method: http.MethodGet, path: "/snippet/view/7", wantStatus: http.StatusOK, wantBody: "Display a snippet with ID 7..."},
		{name: "view snippet, bad id", method: http.MethodGet, path: "/snippet/view/abc", wantStatus: http.StatusNotFound},
		{name: "view snippet, zero id", method: http.MethodGet, path: "/snippet/view/0", wantStatus: http.StatusNotFound},
		{name: "create form", method: http.MethodGet, path: "/snippet/create", wantStatus: http.StatusOK, wantBody: "Display a form to create a new snippet..."},
		{name: "create post", method: http.MethodPost, path: "/snippet/create", wantStatus: http.StatusCreated, wantBody: "Save a new snippet..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newMux(ui).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

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
