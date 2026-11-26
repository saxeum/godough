package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saxeum/godough/internal/auth/googleauth"
	"github.com/saxeum/godough/internal/domain/auth"
	"github.com/saxeum/godough/internal/service"
)

type fakeGoogle struct {
	identity    googleauth.Identity
	exchangeErr error
	authURL     string
	gotCode     string
	gotVerifier string
	gotNonce    string
}

func (f *fakeGoogle) AuthCodeURL(state, nonce, verifier string) string {
	f.gotNonce, f.gotVerifier = nonce, verifier
	return f.authURL
}

func (f *fakeGoogle) Exchange(_ context.Context, code, verifier, nonce string) (googleauth.Identity, error) {
	f.gotCode, f.gotVerifier, f.gotNonce = code, verifier, nonce
	return f.identity, f.exchangeErr
}

type fakeAuthSessions struct {
	token        string
	session      auth.Session
	err          error
	created      bool
	authenticate bool
	revoked      string
}

func (f *fakeAuthSessions) Authenticate(context.Context, string) (auth.Session, error) {
	if f.authenticate {
		return f.session, nil
	}
	return auth.Session{}, auth.ErrNotFound
}

func (f *fakeAuthSessions) Create(_ context.Context, userID int64, _, _ string) (string, auth.Session, error) {
	f.created = true
	f.session.UserID = userID
	return f.token, f.session, f.err
}

func (f *fakeAuthSessions) Revoke(_ context.Context, token string) error {
	f.revoked = token
	return f.err
}

type fakeAuthUsers struct {
	user auth.User
	err  error
}

func (f fakeAuthUsers) Ensure(context.Context, string, string, string) (auth.User, error) {
	return f.user, f.err
}

func (f fakeAuthUsers) Get(context.Context, int64) (auth.User, error) {
	return f.user, f.err
}

func newAuthServer(t *testing.T, google GoogleAuth) http.Handler {
	t.Helper()
	return NewServer(Deps{
		Log:            slog.New(slog.DiscardHandler),
		Snippets:       service.NewSnippets(nil),
		Sessions:       &fakeAuthSessions{token: "session-token", session: auth.Session{UserID: 42, Expires: time.Now().Add(time.Hour)}, authenticate: true},
		Users:          fakeAuthUsers{user: auth.User{ID: 42, Email: "person@example.com", Name: "A Person"}},
		Google:         google,
		TrustedOrigins: []string{"http://localhost:8080"},
	}).Routes()
}

func TestGoogleFlowStartRedirectsAndSetsFlowCookies(t *testing.T) {
	google := &fakeGoogle{authURL: "https://accounts.google.com/o/oauth2/auth?state=from-google"}
	handler := newAuthServer(t, google)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/google", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != google.authURL {
		t.Errorf("Location = %q, want %q", got, google.authURL)
	}

	cookies := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		cookies[c.Name] = c
	}
	for _, name := range []string{oauthStateCookie, oauthNonceCookie, oauthVerifierCookie} {
		c := cookies[name]
		if c == nil || c.Value == "" {
			t.Fatalf("cookie %s is missing or empty", name)
		}
		if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
			t.Errorf("cookie %s flags = HttpOnly:%v Secure:%v SameSite:%v", name, c.HttpOnly, c.Secure, c.SameSite)
		}
		if c.Path != oauthCookiePath {
			t.Errorf("cookie %s path = %q, want %q", name, c.Path, oauthCookiePath)
		}
	}
}

func TestGoogleFlowCallbackCreatesTheSession(t *testing.T) {
	google := &fakeGoogle{identity: googleauth.Identity{Sub: "sub-1", Email: "a@example.com", Name: "A"}}
	sessions := &fakeAuthSessions{token: "session-token", session: auth.Session{Expires: time.Now().Add(time.Hour)}}
	handler := NewServer(Deps{
		Log:      slog.New(slog.DiscardHandler),
		Snippets: service.NewSnippets(nil),
		Sessions: sessions,
		Users:    fakeAuthUsers{user: auth.User{ID: 42}},
		Google:   google,
	}).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=state-1&code=code-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "state-1"})
	req.AddCookie(&http.Cookie{Name: oauthNonceCookie, Value: "nonce-1"})
	req.AddCookie(&http.Cookie{Name: oauthVerifierCookie, Value: "verifier-1"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Errorf("Location = %q, want /", got)
	}
	if !sessions.created {
		t.Error("no session was created")
	}
	if google.gotCode != "code-1" || google.gotVerifier != "verifier-1" || google.gotNonce != "nonce-1" {
		t.Errorf("exchange args = %q/%q/%q", google.gotCode, google.gotVerifier, google.gotNonce)
	}

	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || sessionCookie.Value != "session-token" {
		t.Fatalf("session cookie = %+v, want session-token", sessionCookie)
	}
	if !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie flags = HttpOnly:%v Secure:%v SameSite:%v", sessionCookie.HttpOnly, sessionCookie.Secure, sessionCookie.SameSite)
	}

	for _, c := range rec.Result().Cookies() {
		switch c.Name {
		case oauthStateCookie, oauthNonceCookie, oauthVerifierCookie:
			if c.MaxAge != -1 {
				t.Errorf("flow cookie %s was not cleared: MaxAge=%d", c.Name, c.MaxAge)
			}
		}
	}
}

func TestGoogleFlowRejectsABadState(t *testing.T) {
	google := &fakeGoogle{identity: googleauth.Identity{Sub: "sub-1"}}
	handler := newAuthServer(t, google)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=wrong&code=code-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "state-1"})
	req.AddCookie(&http.Cookie{Name: oauthNonceCookie, Value: "nonce-1"})
	req.AddCookie(&http.Cookie{Name: oauthVerifierCookie, Value: "verifier-1"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if google.gotCode != "" {
		t.Error("the code was exchanged despite a state mismatch")
	}
}

func TestGoogleFlowRejectsMissingFlowCookies(t *testing.T) {
	handler := newAuthServer(t, &fakeGoogle{})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=state-1&code=code-1", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGoogleFlowReportsExchangeFailure(t *testing.T) {
	google := &fakeGoogle{exchangeErr: errors.New("provider rejected the code")}
	handler := newAuthServer(t, google)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=state-1&code=code-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "state-1"})
	req.AddCookie(&http.Cookie{Name: oauthNonceCookie, Value: "nonce-1"})
	req.AddCookie(&http.Cookie{Name: oauthVerifierCookie, Value: "verifier-1"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestGoogleFlowDisabledIs503(t *testing.T) {
	handler := newAuthServer(t, nil)

	for _, path := range []string{"/api/auth/google", "/api/auth/google/callback"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s = %d, want %d", path, rec.Code, http.StatusServiceUnavailable)
		}
	}
}

func TestAuthSessionReturnsTheCurrentUser(t *testing.T) {
	handler := newAuthServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Body.String() != "{\"email\":\"person@example.com\",\"name\":\"A Person\"}\n" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestAuthSessionWithoutACookieIs401(t *testing.T) {
	handler := newAuthServer(t, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/session", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthSignoutRevokesAndClearsTheCookie(t *testing.T) {
	sessions := &fakeAuthSessions{
		token:        "session-token",
		session:      auth.Session{UserID: 42, Expires: time.Now().Add(time.Hour)},
		authenticate: true,
	}
	handler := NewServer(Deps{
		Log:            slog.New(slog.DiscardHandler),
		Snippets:       service.NewSnippets(nil),
		Sessions:       sessions,
		Users:          fakeAuthUsers{user: auth.User{ID: 42}},
		TrustedOrigins: []string{"http://localhost:8080"},
	}).Routes()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/signout", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if sessions.revoked != "session-token" {
		t.Errorf("revoked = %q, want session-token", sessions.revoked)
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName && c.MaxAge == -1 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("session cookie was not cleared")
	}
}

func TestAuthSignoutRequiresATrustedOrigin(t *testing.T) {
	handler := newAuthServer(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAuthSignoutWithoutASessionIs401(t *testing.T) {
	handler := newAuthServer(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signout", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
