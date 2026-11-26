// Package app wires the layers together and owns the process lifecycle. It is
// the only package that knows all of them: config builds it, main runs it, and
// nothing below it imports anything above it.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saxeum/godough/internal/config"
	"github.com/saxeum/godough/internal/repository/postgres"
	"github.com/saxeum/godough/internal/service"
	"github.com/saxeum/godough/internal/transport/httpapi"
)

// shutdownGrace is how long a draining server waits for in-flight requests.
// Kubernetes' terminationGracePeriodSeconds must exceed this plus its preStop
// hook, or the pod is killed mid-drain.
const shutdownGrace = 10 * time.Second

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

	app := &App{
		log: log,
		db:  db,
		srv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}

	return app, nil
}

// Run serves until ctx is cancelled, then drains. It returns the error that
// stopped it, or nil after a clean shutdown.
func (a *App) Run(ctx context.Context) error {
	serveErr := make(chan error, 1)
	go func() {
		err := a.srv.ListenAndServe()
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
