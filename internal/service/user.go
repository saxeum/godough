package service

import (
	"context"

	"github.com/saxeum/godough/internal/domain/auth"
)

// UserRepository is what the sign-in flow needs from storage: one row per
// verified provider subject.
type UserRepository interface {
	UpsertUser(ctx context.Context, sub, email, name string) (auth.User, error)
	GetUser(ctx context.Context, id int64) (auth.User, error)
}

// Users is the user service.
type Users struct {
	repo UserRepository
}

func NewUsers(repo UserRepository) *Users {
	return &Users{repo: repo}
}

// Ensure maps a verified identity to a user row. It does not trust email for
// identity: sub is the key, email and name are display values.
func (u *Users) Ensure(ctx context.Context, sub, email, name string) (auth.User, error) {
	return u.repo.UpsertUser(ctx, sub, email, name)
}

// Get returns the user behind a session, or auth.ErrNotFound.
func (u *Users) Get(ctx context.Context, id int64) (auth.User, error) {
	return u.repo.GetUser(ctx, id)
}
