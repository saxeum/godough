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
	// readTimeout bounds the whole request, headers and body. The body is capped
	// at 1 MiB in the transport, so a client that needs longer than this is
	// trickling bytes on purpose.
	readTimeout = 10 * time.Second
	// writeTimeout must exceed the handler's own deadline, or the connection is
	// cut before the handler can report why it gave up.
	writeTimeout = httpapi.RequestTimeout + 5*time.Second
	idleTimeout  = 60 * time.Second
	// maxHeaderBytes is 1 MiB, the same as the body cap. Go's default is 1 MiB
	// too, but naming it means a change here is visible rather than inherited.
	maxHeaderBytes = 1 << 20

	// shutdownGrace is how long a draining server waits for in-flight requests.
	// Kubernetes' terminationGracePeriodSeconds must exceed this plus its preStop
	// hook, or the pod is killed mid-drain. It is a constant and not a setting
	// on purpose: the relationship to the manifest is the thing that matters, and
	// two places to configure one number is how they come to disagree.
	shutdownGrace = 10 * time.Second
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
	svc := service.NewSnippetService(repo)
	handler := httpapi.NewServer(log, svc, db.Ping).Routes()

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

	shutdownCtx, release := context.WithTimeout(context.Background(), shutdownGrace)
	defer release()

	if err := a.srv.Shutdown(shutdownCtx); err != nil {
		// Shutdown returning an error means requests are still running. Close the
		// connections so they stop, rather than letting the deferred pool Close
		// pull the database out from under handlers that are mid-query.
		a.srv.Close()
		return fmt.Errorf("shutdown: %w", err)
	}
	a.log.Info("graceful shutdown complete")
	return nil
}

// Close releases the database pool. It is separate from Run so the caller
// controls the order: the server drains first, then the pool goes.
func (a *App) Close() {
	a.db.Close()
}
