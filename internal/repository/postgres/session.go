package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saxeum/godough/internal/domain/auth"
)

type sessionRow struct {
	ID        int64
	TokenHash string
	UserID    int64
	Created   time.Time
	Expires   time.Time
	IP        string
	UserAgent string
}

func (r sessionRow) toDomain() auth.Session {
	return auth.Session{
		ID:        r.ID,
		TokenHash: r.TokenHash,
		UserID:    r.UserID,
		Created:   r.Created,
		Expires:   r.Expires,
		IP:        r.IP,
		UserAgent: r.UserAgent,
	}
}

// SessionRepository talks to the sessions table.
type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

// InsertSession stores one session and returns the row as the database wrote
// it, including created and expires.
func (r *SessionRepository) InsertSession(
	ctx context.Context,
	userID int64,
	tokenHash string,
	expires time.Time,
	ip string,
	userAgent string,
) (auth.Session, error) {
	rows, err := r.db.Query(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, token_hash, user_id, created, expires, ip, user_agent`,
		tokenHash, userID, expires, ip, userAgent)
	if err != nil {
		return auth.Session{}, fmt.Errorf("insert session: %w", err)
	}

	scanned, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[sessionRow])
	if err != nil {
		return auth.Session{}, fmt.Errorf("collect inserted session: %w", err)
	}
	return scanned.toDomain(), nil
}

// FindSessionByTokenHash returns a live session, or auth.ErrNotFound. Expiry is
// decided by the database clock, so a skewed application host cannot extend a
// session by accident.
func (r *SessionRepository) FindSessionByTokenHash(ctx context.Context, tokenHash string) (auth.Session, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, token_hash, user_id, created, expires, ip, user_agent
		FROM sessions
		WHERE token_hash = $1 AND expires > now()`, tokenHash)
	if err != nil {
		return auth.Session{}, fmt.Errorf("query session: %w", err)
	}

	scanned, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[sessionRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf("collect session: %w", err)
	}
	return scanned.toDomain(), nil
}

// DeleteSessionByTokenHash revokes one session. A missing row is not an error:
// sign-out has to be safe to retry.
func (r *SessionRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes rows the lookup already ignores. It is the
// cleanup the ticket asks us to decide about; a future scheduler can call it.
func (r *SessionRepository) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE expires <= now()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
