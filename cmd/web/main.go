package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
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
	mux.Handle("GET /", http.FileServerFS(noListing{ui}))

	return mux
}

// noListing hides directories that have no index.html. Without it http.FileServerFS
// answers a directory URL with a listing of the directory's contents.
type noListing struct{ fs.FS }

func (f noListing) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil || !info.IsDir() {
		return file, nil
	}

	if _, err := fs.Stat(f.FS, path.Join(name, "index.html")); err != nil {
		file.Close()
		return nil, fs.ErrNotExist
	}

	return file, nil
}
