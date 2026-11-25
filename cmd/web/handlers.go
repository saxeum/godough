package main

import (
	"errors"
	"net/http"
	"strconv"
)

// snippetsIndex lists every snippet, newest first.
func (app *application) snippetsIndex(w http.ResponseWriter, r *http.Request) {
	snippets, err := app.snippetsAll(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.writeJSON(w, http.StatusOK, snippets)
}

func (app *application) snippetView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.clientError(w, http.StatusNotFound)
		return
	}

	snippet, err := app.snippetGet(r.Context(), id)
	if errors.Is(err, errNotFound) {
		app.clientError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.writeJSON(w, http.StatusOK, snippet)
}

// snippetCreatePost is still a stub: the insert, validation and the web form
// are a later step, so it reports success without writing anything.
func (app *application) snippetCreatePost(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("create a snippet")
	app.writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}
