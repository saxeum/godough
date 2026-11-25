package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type application struct {
	logger *slog.Logger
	db     *pgxpool.Pool
}

// mustEnv returns the value of key, or exits: config has no defaults.
func mustEnv(logger *slog.Logger, key string) string {
	value := os.Getenv(key)
	if value == "" {
		logger.Error("required environment variable is not set", slog.String("key", key))
		os.Exit(1)
	}
	return value
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	addr := mustEnv(logger, "SNIPPET_ADDR")
	dsn := mustEnv(logger, "DATABASE_URL")

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("open database", slog.Any("err", err))
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		logger.Error("ping database", slog.Any("err", err))
		os.Exit(1)
	}

	app := &application{
		logger: logger,
		db:     db,
	}

	logger.Info("starting server", slog.String("addr", addr))
	err = http.ListenAndServe(addr, app.routes(os.DirFS("ui/dist")))
	logger.Error(err.Error())
	os.Exit(1)
}
