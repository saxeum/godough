package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saxeum/godough/internal/domain/auth"
)

type userRow struct {
	ID      int64
	Sub     string
	Email   string
	Name    string
	Created time.Time
}

func (r userRow) toDomain() auth.User {
	return auth.User{ID: r.ID, Sub: r.Sub, Email: r.Email, Name: r.Name, Created: r.Created}
}

// UserRepository owns the users table.
type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// UpsertUser maps a verified provider identity onto one row. sub is the key;
// email and name are display values and may change on a later sign-in.
func (r *UserRepository) UpsertUser(ctx context.Context, sub, email, name string) (auth.User, error) {
	rows, err := r.db.Query(ctx, `
		INSERT INTO users (sub, email, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (sub) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name
		RETURNING id, sub, email, name, created`, sub, email, name)
	if err != nil {
		return auth.User{}, fmt.Errorf("upsert user: %w", err)
	}

	scanned, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[userRow])
	if err != nil {
		return auth.User{}, fmt.Errorf("collect user: %w", err)
	}
	return scanned.toDomain(), nil
}
