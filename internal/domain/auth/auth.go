// Package auth is the identity entity shared by the session store, the
// middleware and the handlers. It imports only the standard library.
package auth

import (
	"errors"
	"time"
)

// ErrNotFound is returned when no live session matches a token. Expired,
// deleted and tampered tokens all look the same from here, which is exactly
// what the client should see.
var ErrNotFound = errors.New("session not found")

// User is one account. Sub is the stable identity from the provider; email and
// name are display values and may change.
type User struct {
	ID      int64
	Sub     string
	Email   string
	Name    string
	Created time.Time
}

// Session is one stored session row. TokenHash is the SHA-256 hex digest of the
// cookie value; the raw token is never stored.
type Session struct {
	ID        int64
	TokenHash string
	UserID    int64
	Created   time.Time
	Expires   time.Time
	IP        string
	UserAgent string
}
