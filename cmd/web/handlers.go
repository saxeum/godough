package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
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

// snippetDelete removes a snippet. 204, not 200: there is nothing to say back.
func (app *application) snippetDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.clientError(w, http.StatusNotFound)
		return
	}

	if err := app.snippetDeleteByID(r.Context(), id); errors.Is(err, errNotFound) {
		app.clientError(w, http.StatusNotFound)
		return
	} else if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("snippet deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

type createRequest struct {
	Title         string
	Content       string
	ExpiresInDays int
}

// validateCreate reports every bad field at once, so a form can show them all.
func validateCreate(req createRequest) map[string]string {
	bad := map[string]string{}

	switch {
	case strings.TrimSpace(req.Title) == "":
		bad["title"] = "is required"
	case utf8.RuneCountInString(req.Title) > 100:
		// varchar(100) counts characters, not bytes.
		bad["title"] = "must be at most 100 characters"
	}

	if strings.TrimSpace(req.Content) == "" {
		bad["content"] = "is required"
	}

	if req.ExpiresInDays < 1 || req.ExpiresInDays > 365 {
		bad["expiresInDays"] = "must be between 1 and 365"
	}

	return bad
}

func (app *application) snippetCreatePost(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	if bad := validateCreate(req); len(bad) > 0 {
		app.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": bad})
		return
	}

	s, err := app.snippetInsert(r.Context(), req.Title, req.Content, req.ExpiresInDays)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("snippet created", "id", s.ID)
	w.Header().Set("Location", fmt.Sprintf("/api/snippet/%d", s.ID))
	app.writeJSON(w, http.StatusCreated, s)
}
