package main

import (
	"fmt"
	"net/http"
	"strconv"
)

func (app *application) snippetView(w http.ResponseWriter, r *http.Request) {
	idValue := r.PathValue("id")
	id, err := strconv.Atoi(idValue)
	if err != nil || id < 1 {
		app.clientError(w, http.StatusNotFound)
		return
	}

	fmt.Fprintf(w, "Display a snippet with ID %d...", id)
}

func (app *application) snippetCreate(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Display a form to create a new snippet..."))
}

func (app *application) snippetCreatePost(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("create a snippet")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Save a new snippet..."))
}
