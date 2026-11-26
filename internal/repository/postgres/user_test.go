package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestUserRepositoryUpsertsOnSub(t *testing.T) {
	pool := testPool(t)
	repo := NewUserRepository(pool)
	ctx := t.Context()
	sub := fmt.Sprintf("user-test-%d", time.Now().UnixNano())

	first, err := repo.UpsertUser(ctx, sub, "first@example.com", "First")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE sub = $1`, sub)
	})

	second, err := repo.UpsertUser(ctx, sub, "second@example.com", "Second")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("id changed from %d to %d", first.ID, second.ID)
	}
	if second.Email != "second@example.com" || second.Name != "Second" {
		t.Errorf("second upsert = %+v", second)
	}
}
