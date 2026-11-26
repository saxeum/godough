package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/saxeum/godough/internal/domain/auth"
)

const sessionCookieName = "session"

// SessionService is the one method the transport needs. The service package
// owns hashing and expiry; the middleware only turns a cookie into a session
// or a 401.
type SessionService interface {
	Authenticate(ctx context.Context, token string) (auth.Session, error)
	Create(ctx context.Context, userID int64, ip, userAgent string) (string, auth.Session, error)
}

type sessionKey struct{}

// sessionFromContext is the only way a handler reads the session. A private key
// type means no other package can collide with a string key.
func sessionFromContext(ctx context.Context) (auth.Session, bool) {
	session, ok := ctx.Value(sessionKey{}).(auth.Session)
	return session, ok
}

func sessionUserID(ctx context.Context) (int64, bool) {
	session, ok := sessionFromContext(ctx)
	return session.UserID, ok
}

// requireSession protects the snippet routes and nothing else. It runs before
// the origin check, so an anonymous write is 401 rather than a 403 that leaks
// whether the route exists.
func requireSession(sessions SessionService, log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeJSON(w, http.StatusUnauthorized, errorBody(http.StatusUnauthorized))
			return
		}

		session, err := sessions.Authenticate(r.Context(), cookie.Value)
		switch {
		case errors.Is(err, auth.ErrNotFound):
			writeJSON(w, http.StatusUnauthorized, errorBody(http.StatusUnauthorized))
			return
		case err != nil:
			log.Error("session lookup failed",
				"request_id", requestIDFromContext(r.Context()),
				"method", r.Method,
				"uri", r.URL.RequestURI(),
				"err", err,
			)
			writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, session)))
	})
}

// requireTrustedOrigin is the CSRF check for cookie-authenticated writes. It
// requires an exact Origin, because the app is one origin and no token
// plumbing is needed. SameSite=Lax is a setting, not a check; this is the check.
func requireTrustedOrigin(allowed []string, log *slog.Logger, next http.Handler) http.Handler {
	trusted := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		trusted[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if _, ok := trusted[origin]; !ok {
			log.Warn("csrf rejected",
				"request_id", requestIDFromContext(r.Context()),
				"method", r.Method,
				"uri", r.URL.RequestURI(),
				"origin", origin,
			)
			writeJSON(w, http.StatusForbidden, errorBody(http.StatusForbidden))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// setSessionCookie is the only writer of the cookie. The token is HttpOnly,
// Secure and SameSite=Lax, and its expiry matches the row's.
func setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
