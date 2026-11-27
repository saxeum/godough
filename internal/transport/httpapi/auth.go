package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/saxeum/godough/internal/auth/googleauth"
	"github.com/saxeum/godough/internal/domain/auth"
)

const (
	oauthStateCookie    = "oauth_state"
	oauthNonceCookie    = "oauth_nonce"
	oauthVerifierCookie = "oauth_verifier"
	oauthCookiePath     = "/api/auth"
	oauthCookieTTL      = 10 * time.Minute
)

// GoogleAuth is the provider seam. Tests replace it with a fake; the
// production implementation lives in internal/auth/googleauth.
type GoogleAuth interface {
	AuthCodeURL(state, nonce, verifier string) string
	Exchange(ctx context.Context, code, verifier, nonce string) (googleauth.Identity, error)
}

// UserService is the one operation the callback needs.
type UserService interface {
	Ensure(ctx context.Context, sub, email, name string) (auth.User, error)
	Get(ctx context.Context, id int64) (auth.User, error)
}

// googleStart begins the authorization-code flow. The state, nonce and PKCE
// verifier are short-lived, HttpOnly cookies; the callback consumes them once.
func (s *Server) googleStart(w http.ResponseWriter, r *http.Request) {
	if s.google == nil {
		writeJSON(w, http.StatusServiceUnavailable, Error{Error: "sign-in is not configured"})
		return
	}

	state, err := googleauth.NewSecret()
	if err != nil {
		s.log.Error("sign-in state generation failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		return
	}
	nonce, err := googleauth.NewSecret()
	if err != nil {
		s.log.Error("sign-in nonce generation failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		return
	}
	verifier, err := googleauth.NewSecret()
	if err != nil {
		s.log.Error("sign-in verifier generation failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		return
	}

	setOAuthCookie(w, oauthStateCookie, state)
	setOAuthCookie(w, oauthNonceCookie, nonce)
	setOAuthCookie(w, oauthVerifierCookie, verifier)

	http.Redirect(w, r, s.google.AuthCodeURL(state, nonce, verifier), http.StatusFound)
}

// googleCallback consumes the one-use cookies, exchanges the code and creates
// the local session. Any failure is logged with the request id and answered
// with a generic message; the provider error stays in the log.
func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	if s.google == nil {
		writeJSON(w, http.StatusServiceUnavailable, Error{Error: "sign-in is not configured"})
		return
	}

	stateCookie, stateErr := r.Cookie(oauthStateCookie)
	nonceCookie, nonceErr := r.Cookie(oauthNonceCookie)
	verifierCookie, verifierErr := r.Cookie(oauthVerifierCookie)

	// Single use: clear all three before anything can fail.
	clearOAuthCookie(w, oauthStateCookie)
	clearOAuthCookie(w, oauthNonceCookie)
	clearOAuthCookie(w, oauthVerifierCookie)

	if stateErr != nil || nonceErr != nil || verifierErr != nil {
		s.signInFailed(w, r, "missing flow cookies")
		return
	}

	queryState := r.URL.Query().Get("state")
	if subtle.ConstantTimeCompare([]byte(queryState), []byte(stateCookie.Value)) != 1 {
		s.signInFailed(w, r, "state mismatch")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		s.signInFailed(w, r, "missing authorization code")
		return
	}

	identity, err := s.google.Exchange(r.Context(), code, verifierCookie.Value, nonceCookie.Value)
	if err != nil {
		s.log.Error("sign-in exchange failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusBadGateway, Error{Error: "sign-in failed"})
		return
	}

	user, err := s.users.Ensure(r.Context(), identity.Sub, identity.Email, identity.Name)
	if err != nil {
		s.log.Error("sign-in user upsert failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		return
	}

	token, session, err := s.sessions.Create(r.Context(), user.ID, clientIP(r), r.UserAgent())
	if err != nil {
		s.log.Error("sign-in session creation failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		return
	}

	setSessionCookie(w, token, session.Expires)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// authSession is what the UI asks for on load. It is the only endpoint that
// tells the browser who the cookie belongs to; no token or email is baked into
// the bundle.
func (s *Server) authSession(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody(http.StatusUnauthorized))
		return
	}
	user, err := s.users.Get(r.Context(), session.UserID)
	switch {
	case errors.Is(err, auth.ErrNotFound):
		writeJSON(w, http.StatusUnauthorized, errorBody(http.StatusUnauthorized))
		return
	case err != nil:
		s.log.Error("session user lookup failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}{Email: user.Email, Name: user.Name})
}

// authSignout deletes the database row, then clears the cookie. A failed delete
// is a 500 rather than a cheerful redirect: revocation is the point, and a
// cookie that looks cleared while the row lives on is the bug the ticket
// exists to prevent.
func (s *Server) authSignout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusUnauthorized, errorBody(http.StatusUnauthorized))
		return
	}
	if err := s.sessions.Revoke(r.Context(), cookie.Value); err != nil {
		s.log.Error("sign-out failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		return
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) signInFailed(w http.ResponseWriter, r *http.Request, reason string) {
	s.log.Warn("sign-in rejected", "request_id", requestIDFromContext(r.Context()), "reason", reason)
	writeJSON(w, http.StatusBadRequest, Error{Error: "sign-in failed"})
}

func setOAuthCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     oauthCookiePath,
		MaxAge:   int(oauthCookieTTL.Seconds()),
		Expires:  time.Now().Add(oauthCookieTTL),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     oauthCookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clientIP is the address the request came from, as far as anything can know it.
//
// Behind the ALB, RemoteAddr is the load balancer, so every session recorded the same
// address and a per-client limit would have counted the whole internet as one client
// (#150). The ALB appends the address it saw to X-Forwarded-For, which is what makes
// the *last* entry trustworthy and everything to its left whatever the client chose to
// claim: take the last, ignore the rest.
//
// A header that does not end in an address is ignored rather than trusted, because the
// fallback — the peer we are actually talking to — is at least true.
//
// This assumes exactly one trusted proxy, which is true of both paths: the ALB in the
// cluster, and the nginx load balancer in front of it locally. The API's own port is
// not exposed, so there is no route to it that skips that hop; if one ever appears, this
// is the line that becomes a spoofing hole.
func clientIP(r *http.Request) string {
	if addr, ok := forwardedClientIP(r.Header.Values("X-Forwarded-For")); ok {
		return addr
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// forwardedClientIP reads the last address in one or more X-Forwarded-For headers.
// Several headers are equivalent to one comma-joined header in every proxy that
// writes them, so they are flattened the same way.
func forwardedClientIP(values []string) (string, bool) {
	last := ""
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				last = part
			}
		}
	}
	return parseClientAddr(last)
}

// parseClientAddr accepts an address with or without the port a proxy may have
// recorded next to it, and returns it in canonical form so the column is comparable.
func parseClientAddr(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return "", false
	}
	return addr.String(), true
}
