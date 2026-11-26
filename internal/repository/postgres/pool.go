package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session limits applied to every connection in the pool.
//
// Postgres defaults statement_timeout to zero, which is no limit at all.
const (
	StatementTimeout = 5 * time.Second
	LockTimeout      = 2 * time.Second
)

// NewPool opens the connection pool and applies the session limits to every
// connection, including ones opened later to replace a dropped connection —
// which is why this is AfterConnect and not three statements after startup.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// set_config rather than SET: SET does not take parameters.
		if _, err := conn.Exec(ctx, `SELECT set_config('statement_timeout', $1, false)`, StatementTimeout.String()); err != nil {
			return fmt.Errorf("set statement_timeout: %w", err)
		}
		if _, err := conn.Exec(ctx, `SELECT set_config('lock_timeout', $1, false)`, LockTimeout.String()); err != nil {
			return fmt.Errorf("set lock_timeout: %w", err)
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return pool, nil
}
