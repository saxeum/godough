package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// RequestTimeout bounds one request end to end, from the moment the router hands
// it over to the moment the handler returns.
//
// It must exceed the database's statement_timeout (postgres.StatementTimeout,
// 5s) so a slow query surfaces as a database error that names the cause. If the
// request deadline fired first, every slow query would look identical and the
// log would say only "deadline exceeded".
const RequestTimeout = 15 * time.Second

// recoverer turns a panic into the API's own 500 and keeps the process serving.
//
// Without it, a panic is caught by net/http, which closes the connection with no
// body and writes to http.Server.ErrorLog — a different stream in a different
// format from everything else the service logs.
func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			// ErrAbortHandler is how a handler asks for the connection to be
			// aborted, and net/http handles it itself. Swallowing it would keep
			// a connection alive that the handler wanted gone.
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}

			log.Error("panic recovered",
				"panic", recovered,
				"stack", string(debug.Stack()),
				"method", r.Method,
				"uri", r.URL.RequestURI(),
			)
			writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		}()

		next.ServeHTTP(w, r)
	})
}

// requestDeadline puts a deadline on the request context, so a handler blocked on
// the database is cancelled by the server rather than by the client leaving.
//
// It is the context, not http.TimeoutHandler: pgx watches the context and stops
// the query, whereas a timeout handler would answer the client and leave the
// query running.
func requestDeadline(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
