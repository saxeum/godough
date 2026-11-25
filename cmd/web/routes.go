package main

import "net/http"

func (app *application) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", app.healthz)
	mux.HandleFunc("GET /readyz", app.readyz)

	mux.HandleFunc("GET /api/snippet/view/{id}", app.snippetView)
	mux.HandleFunc("GET /api/snippet/create", app.snippetCreate)
	mux.HandleFunc("POST /api/snippet/create", app.snippetCreatePost)

	return mux
}
