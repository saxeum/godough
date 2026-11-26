// Package app wires the layers together and owns the process lifecycle. It is
// the only package that knows all of them: config builds it, main runs it, and
// nothing below it imports anything above it.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saxeum/godough/internal/config"
	"github.com/saxeum/godough/internal/repository/postgres"
	"github.com/saxeum/godough/internal/service"
	"github.com/saxeum/godough/internal/transport/httpapi"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 20

	// A request may run for httpapi.RequestTimeout, so anything that waits for
	// one has to outlast it: the time allowed to write the response, and the time
	// a drain waits before closing connections.
	responseOverhead = 5 * time.Second
	writeTimeout     = httpapi.RequestTimeout + responseOverhead
	shutdownGrace    = httpapi.RequestTimeout + responseOverhead
)

// newHTTPServer builds the server with every limit named.
//
// It is separate from New so the limits can be asserted without a database, and
// so that "which timeout is this" is answered by a constant rather than by a
// literal inside a struct.
func newHTTPServer(addr string, handler http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// Everything Go's server wants to say — TLS handshakes, handler errors,
		// panics that escaped a handler — goes through the application's logger
		// instead of the standard library's default one.
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
}

// App is a running configuration: the database pool, the HTTP server built over
// the layers, and the logger they share.
type App struct {
	log *slog.Logger
	db  *pgxpool.Pool
	srv *http.Server
}

// New opens the database, proves it is reachable, and builds the chain
// repository -> service -> transport. A failure here is a boot failure.
//
// ctx carries the boot deadline. It is the caller's, not a constant in here, so
// that a database which accepts the connection and never answers fails the
// process in bounded time instead of parking a pod in a half-started state.
func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	db, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	repo := postgres.NewSnippetRepository(db)
	svc := service.NewSnippets(repo)
	sessions := service.NewSessions(postgres.NewSessionRepository(db))
	if n, err := sessions.DeleteExpired(ctx); err != nil {
		log.Warn("expired session cleanup failed", "err", err)
	} else if n > 0 {
		log.Info("expired sessions removed", "count", n)
	}
	handler := httpapi.NewServer(log, svc, sessions, cfg.TrustedOrigins, db.Ping).Routes()

	return &App{
		log: log,
		db:  db,
		srv: newHTTPServer(cfg.Addr, handler, log),
	}, nil
}

// Run binds the configured address and serves until ctx is cancelled, then
// drains. It returns the error that stopped it, or nil after a clean shutdown.
//
// Binding before serving means a taken port is an error the caller sees, not a
// failure inside a goroutine racing the rest of startup.
func (a *App) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", a.srv.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", a.srv.Addr, err)
	}

	return a.Serve(ctx, listener)
}

// Serve serves an already-bound listener until ctx is cancelled, then drains.
//
// Split from Run so a test can bind port 0, learn the address, and cancel the
// context — with the listen call buried inside Run there was nothing to drive.
func (a *App) Serve(ctx context.Context, listener net.Listener) error {
	return a.serve(ctx, listener, shutdownGrace)
}

// serve is Serve with the grace period as an argument, so the expiry path can be
// exercised without waiting ten seconds for it.
func (a *App) serve(ctx context.Context, listener net.Listener, grace time.Duration) error {
	serveErr := make(chan error, 1)
	go func() {
		err := a.srv.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			a.log.Info("stopped serving new connections")
			return
		}
		serveErr <- fmt.Errorf("serve: %w", err)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	a.drain(grace)
	return nil
}

// drain stops accepting connections and waits up to grace for the ones already
// running.
//
// Running out of grace is not a failure of the process and must not be reported
// as one. A request that took eleven seconds is a slow request, not a crash, and
// returning an error here makes `kubectl rollout status` report a failed rollout
// for a pod that did what it was told. The number that should have been generous
// enough is terminationGracePeriodSeconds in the manifest (#5); the process's job
// is to leave a trace saying it was not.
func (a *App) drain(grace time.Duration) {
	ctx, release := context.WithTimeout(context.Background(), grace)
	defer release()

	err := a.srv.Shutdown(ctx)
	if err == nil {
		a.log.Info("graceful shutdown complete")
		return
	}

	// Requests are still running. Close the connections so they stop, rather than
	// letting the caller's pool Close pull the database out from under handlers
	// that are mid-query. Shutdown drains; Close stops.
	a.srv.Close()
	a.log.Warn("shutdown grace expired with requests in flight; connections closed",
		"err", err, "grace", grace)
}

// Close releases the database pool. It is separate from Run so the caller
// controls the order: the server drains first, then the pool goes.
func (a *App) Close() {
	a.db.Close()
}
