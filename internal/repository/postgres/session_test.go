package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/saxeum/godough/internal/domain/auth"
	"github.com/saxeum/godough/internal/service"
)

func TestSessionRepository(t *testing.T) {
	pool := testPool(t)
	repo := NewSessionRepository(pool)
	ctx := t.Context()

	var userID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (sub, email, name) VALUES ($1, $2, $3) RETURNING id`,
		fmt.Sprintf("session-test-%d", time.Now().UnixNano()), "session@example.com", "Session Test",
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	validToken := "valid-token"
	validHash := service.HashToken(validToken)
	expires := time.Now().Add(time.Hour)
	if _, err := repo.InsertSession(ctx, userID, validHash, expires, "127.0.0.1", "test"); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	t.Run("valid", func(t *testing.T) {
		got, err := repo.FindSessionByTokenHash(ctx, validHash)
		if err != nil {
			t.Fatalf("find session: %v", err)
		}
		if got.UserID != userID {
			t.Errorf("UserID = %d, want %d", got.UserID, userID)
		}
	})

	t.Run("tampered", func(t *testing.T) {
		if _, err := repo.FindSessionByTokenHash(ctx, service.HashToken("tampered")); !errors.Is(err, auth.ErrNotFound) {
			t.Errorf("err = %v, want auth.ErrNotFound", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		expiredHash := service.HashToken("expired-token")
		if _, err := repo.InsertSession(ctx, userID, expiredHash, time.Now().Add(-time.Minute), "127.0.0.1", "test"); err != nil {
			t.Fatalf("insert expired session: %v", err)
		}
		if _, err := repo.FindSessionByTokenHash(ctx, expiredHash); !errors.Is(err, auth.ErrNotFound) {
			t.Errorf("err = %v, want auth.ErrNotFound", err)
		}
	})

	t.Run("deleted", func(t *testing.T) {
		if err := repo.DeleteSessionByTokenHash(ctx, validHash); err != nil {
			t.Fatalf("delete session: %v", err)
		}
		if _, err := repo.FindSessionByTokenHash(ctx, validHash); !errors.Is(err, auth.ErrNotFound) {
			t.Errorf("err = %v, want auth.ErrNotFound", err)
		}
	})
}
