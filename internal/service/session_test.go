package service

import (
	"context"
	"testing"
	"time"

	"github.com/saxeum/godough/internal/domain/auth"
)

type fakeSessionRepo struct {
	insertedHash   string
	insertedUserID int64
	insertedExpiry time.Time
	findHash       string
	findSession    auth.Session
	findErr        error
	deletedHash    string
}

func (f *fakeSessionRepo) InsertSession(_ context.Context, userID int64, tokenHash string, expires time.Time, _, _ string) (auth.Session, error) {
	f.insertedUserID = userID
	f.insertedHash = tokenHash
	f.insertedExpiry = expires
	return auth.Session{UserID: userID, TokenHash: tokenHash, Expires: expires}, nil
}

func (f *fakeSessionRepo) FindSessionByTokenHash(_ context.Context, tokenHash string) (auth.Session, error) {
	f.findHash = tokenHash
	return f.findSession, f.findErr
}

func (f *fakeSessionRepo) DeleteSessionByTokenHash(_ context.Context, tokenHash string) error {
	f.deletedHash = tokenHash
	return nil
}

func (f *fakeSessionRepo) DeleteExpiredSessions(context.Context) (int64, error) { return 0, nil }

func TestSessionServiceStoresHashAndRevokes(t *testing.T) {
	repo := &fakeSessionRepo{}
	sessions := NewSessions(repo)

	token, session, err := sessions.Create(t.Context(), 7, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if token == "" {
		t.Fatal("Create returned an empty token")
	}
	if repo.insertedHash == token {
		t.Fatal("the raw token was stored")
	}
	if repo.insertedHash != HashToken(token) {
		t.Errorf("stored hash = %q, want HashToken(token)", repo.insertedHash)
	}
	if repo.insertedUserID != 7 || session.UserID != 7 {
		t.Errorf("user = %d/%d, want 7", repo.insertedUserID, session.UserID)
	}
	if !repo.insertedExpiry.After(time.Now()) {
		t.Errorf("expiry %v is not in the future", repo.insertedExpiry)
	}

	if err := sessions.Revoke(t.Context(), token); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if repo.deletedHash != HashToken(token) {
		t.Errorf("deleted hash = %q, want HashToken(token)", repo.deletedHash)
	}
}

func TestSessionServiceAuthenticate(t *testing.T) {
	repo := &fakeSessionRepo{findSession: auth.Session{UserID: 3}}
	sessions := NewSessions(repo)

	got, err := sessions.Authenticate(t.Context(), "token")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.UserID != 3 {
		t.Errorf("user = %d, want 3", got.UserID)
	}
	if repo.findHash != HashToken("token") {
		t.Errorf("lookup hash = %q, want HashToken(token)", repo.findHash)
	}

	repo.findHash = ""
	if _, err := sessions.Authenticate(t.Context(), ""); err != auth.ErrNotFound {
		t.Errorf("empty token err = %v, want auth.ErrNotFound", err)
	}
	if repo.findHash != "" {
		t.Error("empty token reached the repository")
	}
}
