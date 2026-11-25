package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
)

func main() {
	// Path is relative to the directory the server is started from: run it from
	// the repo root (see `make run`).
	ui := os.DirFS("ui/dist")

	log.Print("starting server: http://localhost:4000")
	log.Fatal(http.ListenAndServe(":4000", newMux(ui)))
}

func newMux(ui fs.FS) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /snippet/view/{id}", snippetView)
	mux.HandleFunc("GET /snippet/create", snippetCreate)
	mux.HandleFunc("POST /snippet/create", snippetCreatePost)

	// Everything else is the React app: /src/* has no handler here, the built
	// bundle lives in ui/dist.
	mux.Handle("GET /", http.FileServerFS(ui))

	return mux
}
