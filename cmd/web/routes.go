package main

import "net/http"

func (app *application) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", app.healthz)
	mux.HandleFunc("GET /readyz", app.readyz)

	mux.HandleFunc("GET /api/snippets", app.snippetsIndex)
	mux.HandleFunc("GET /api/snippet/view/{id}", app.snippetView)
	mux.HandleFunc("POST /api/snippet/create", app.snippetCreatePost)

	return mux
}
