package main

import (
	"encoding/json"
	"net/http"
)

// writeJSON is the only way the API writes a body: every response is JSON,
// except the probe bodies from healthz and readyz, which nothing parses.
func (app *application) writeJSON(w http.ResponseWriter, status int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		app.logger.Error("marshal json", "err", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error("server error", "err", err, "method", r.Method, "uri", r.URL.RequestURI())
	app.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func (app *application) clientError(w http.ResponseWriter, status int) {
	app.writeJSON(w, status, map[string]string{"error": http.StatusText(status)})
}
