package main

import (
	"io/fs"
	"net/http"
)

func (app *application) routes(ui fs.FS) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /snippet/view/{id}", app.snippetView)
	mux.HandleFunc("GET /snippet/create", app.snippetCreate)
	mux.HandleFunc("POST /snippet/create", app.snippetCreatePost)

	mux.Handle("GET /", http.FileServerFS(ui))

	return mux
}
