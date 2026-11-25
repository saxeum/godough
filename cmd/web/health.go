package main

import (
	"context"
	"net/http"
	"time"
)

// healthz says the process is alive. It must not touch the database: a
// database blip is not a reason to restart the container.
func (app *application) healthz(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok\n"))
}

// readyz says the server can serve real traffic, so it does touch the database.
func (app *application) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := app.db.Ping(ctx); err != nil {
		app.logger.Error("readiness check failed", "err", err)
		app.clientError(w, http.StatusServiceUnavailable)
		return
	}

	w.Write([]byte("ok\n"))
}
