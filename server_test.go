package main

import (
	"net/http"
	"testing"
)

func TestNewMux(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string // empty means: do not check the body
	}{
		{name: "hello", method: http.MethodGet, path: "/hello", wantStatus: http.StatusOK, wantBody: "hello\n"},
		{name: "wrong method", method: http.MethodPost, path: "/hello", wantStatus: http.StatusMethodNotAllowed},
		{name: "unknown path", method: http.MethodGet, path: "/nope", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}
