package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type application struct {
	logger *slog.Logger
	db     *pgxpool.Pool
}

// configFromEnv reads the two required settings. There are no defaults.
func configFromEnv() (addr, dsn string, err error) {
	addr = os.Getenv("SNIPPET_ADDR")
	if addr == "" {
		return "", "", errors.New("SNIPPET_ADDR is not set")
	}
	dsn = os.Getenv("DATABASE_URL")
	if dsn == "" {
		return "", "", errors.New("DATABASE_URL is not set")
	}
	return addr, dsn, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// run owns every resource, so its defers (db.Close) run on the way out.
	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	addr, dsn, err := configFromEnv()
	if err != nil {
		return err
	}

	ctx := context.Background()

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	app := &application{logger: logger, db: db}

	srv := &http.Server{
		Addr:              addr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("starting server", "addr", addr)
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("serve: %w", err)
			return
		}
		logger.Info("stopped serving new connections")
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serveErr:
		return err
	case <-sigChan:
	}

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("graceful shutdown complete")
	return nil
}
