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
		logger.Error("fatal", slog.Any("err", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	addr, dsn, err := configFromEnv()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
		Handler:           app.routes(os.DirFS("ui/dist")),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown", slog.Any("err", err))
		}
	}()

	logger.Info("starting server", slog.String("addr", addr))
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}

	logger.Info("server stopped")
	return nil
}
