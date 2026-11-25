package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultDSN matches the dev database in docker-compose.yml.
const defaultDSN = "postgres://godough_app:snippet_app@localhost:5432/snippet?sslmode=disable"

func databaseURL() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	return defaultDSN
}

type application struct {
	logger *slog.Logger
	db     *pgxpool.Pool
}

func main() {
	addr := os.Getenv("SNIPPET_ADDR")
	if addr == "" {
		addr = ":4000"
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx := context.Background()
	db, err := pgxpool.New(ctx, databaseURL())
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
