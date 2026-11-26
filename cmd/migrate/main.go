// Command migrate applies every pending embedded migration, in order, recording
// each in schema_migrations.
//
// It runs as the owner, because migrations create tables and the application
// role owns nothing. The DSN comes from DATABASE_URL_OWNER rather than
// DATABASE_URL so the two cannot be confused.
//
// There is no -dir flag: the SQL is embedded in this binary, so the thing that
// runs also contains the thing that is applied.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saxeum/godough/internal/migrate"
	"github.com/saxeum/godough/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dsn := os.Getenv("DATABASE_URL_OWNER")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL_OWNER is not set: migrations run as the owner, not as the application role")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	all, err := migrate.Discover(migrations.FS, ".")
	if err != nil {
		return err
	}
	if len(all) == 0 {
		return fmt.Errorf("no migrations were embedded; the build is broken")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	applied, err := migrate.Apply(ctx, pool, all)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		fmt.Printf("nothing to apply; %d migration(s) already recorded\n", len(all))
		return nil
	}
	for _, m := range applied {
		fmt.Printf("applied %s\n", m.Name)
	}

	return nil
}
