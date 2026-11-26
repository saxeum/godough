// Package service holds the rules: what a valid snippet is, and what the API
// does about one that is not. It knows nothing about HTTP and nothing about
// Postgres, which is what makes it testable without either.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/saxeum/godough/internal/domain/auth"
)

// SessionTTL is the absolute lifetime of a session. There is deliberately no
// sliding renewal: a stolen cookie cannot be kept alive by using it, and the
// table has one finite row per browser session. Expired rows are ignored by
// the lookup; DeleteExpired is safe to call from a scheduler.
const SessionTTL = 7 * 24 * time.Hour

// SessionRepository is what the service needs from session storage.
type SessionRepository interface {
	InsertSession(ctx context.Context, userID int64, tokenHash string, expires time.Time, ip, userAgent string) (auth.Session, error)
	FindSessionByTokenHash(ctx context.Context, tokenHash string) (auth.Session, error)
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context) (int64, error)
}

// Sessions is the session service. The raw token exists only in the cookie;
// the database sees its hash.
type Sessions struct {
	repo SessionRepository
	now  func() time.Time
}

func NewSessions(repo SessionRepository) *Sessions {
	return &Sessions{repo: repo, now: time.Now}
}

// Authenticate turns a cookie value into a live session. Missing, expired,
// deleted and tampered tokens all return auth.ErrNotFound.
func (s *Sessions) Authenticate(ctx context.Context, token string) (auth.Session, error) {
	if token == "" {
		return auth.Session{}, auth.ErrNotFound
	}
	return s.repo.FindSessionByTokenHash(ctx, HashToken(token))
}

// Create issues a new token and stores only its hash. The caller puts the token
// in the cookie and never logs it.
func (s *Sessions) Create(ctx context.Context, userID int64, ip, userAgent string) (string, auth.Session, error) {
	token, err := newToken()
	if err != nil {
		return "", auth.Session{}, fmt.Errorf("generate session token: %w", err)
	}
	expires := s.now().Add(SessionTTL)
	session, err := s.repo.InsertSession(ctx, userID, HashToken(token), expires, ip, userAgent)
	if err != nil {
		return "", auth.Session{}, err
	}
	return token, session, nil
}

// Revoke deletes the row. Sign-out that only clears the cookie gives up the one
// property database sessions exist for.
func (s *Sessions) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSessionByTokenHash(ctx, HashToken(token))
}

// DeleteExpired is the cleanup entry point. It is not on the request path.
func (s *Sessions) DeleteExpired(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpiredSessions(ctx)
}

// HashToken is the one definition of the stored value. It is exported so tests
// and the migration's first sign-in helper cannot invent a different hash.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
