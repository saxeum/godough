package main

import "net/http"

func (app *application) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", app.healthz)
	mux.HandleFunc("GET /readyz", app.readyz)

	mux.HandleFunc("GET /api/snippet", app.snippetsIndex)
	mux.HandleFunc("GET /api/snippet/{id}", app.snippetView)
	mux.HandleFunc("DELETE /api/snippet/{id}", app.snippetDelete)
	mux.HandleFunc("POST /api/snippet", app.snippetCreatePost)

	return mux
}
