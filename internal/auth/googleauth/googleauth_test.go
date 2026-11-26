package googleauth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
)

type testIssuer struct {
	*oidctest.Server
	idToken string
}

func (s *testIssuer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"id_token":     s.idToken,
		})
		return
	}
	s.Server.ServeHTTP(w, r)
}

func newTestProvider(t *testing.T) (*Provider, *testIssuer, *rsa.PrivateKey, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	issuer := &testIssuer{Server: &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{{
			PublicKey: &privateKey.PublicKey,
			KeyID:     "test-key",
			Algorithm: oidc.RS256,
		}},
	}}
	server := httptest.NewServer(issuer)
	t.Cleanup(server.Close)
	issuer.SetIssuer(server.URL)

	provider, err := New(t.Context(), "client-id", "client-secret", "https://app.example.com/api/auth/google/callback", server.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return provider, issuer, privateKey, server.URL
}

func tokenClaims(issuer, audience, nonce string, expires time.Time) string {
	claims, _ := json.Marshal(map[string]any{
		"iss":   issuer,
		"aud":   audience,
		"sub":   "google-sub-1",
		"nonce": nonce,
		"exp":   expires.Unix(),
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"email": "person@example.com",
		"name":  "A Person",
	})
	return string(claims)
}

func TestGoogleFlowAuthCodeURLUsesPKCEAndNonce(t *testing.T) {
	provider, _, _, _ := newTestProvider(t)

	raw := provider.AuthCodeURL("state-1", "nonce-1", "verifier-1")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	q := u.Query()
	if q.Get("state") != "state-1" {
		t.Errorf("state = %q", q.Get("state"))
	}
	if q.Get("nonce") != "nonce-1" {
		t.Errorf("nonce = %q", q.Get("nonce"))
	}
	if q.Get("code_challenge") != Challenge("verifier-1") {
		t.Errorf("code_challenge = %q", q.Get("code_challenge"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q", q.Get("code_challenge_method"))
	}
	if q.Get("client_id") != "client-id" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
}

func TestGoogleFlowExchangeAcceptsAValidIdentity(t *testing.T) {
	provider, issuer, privateKey, issuerURL := newTestProvider(t)
	nonce := "nonce-1"
	issuer.idToken = oidctest.SignIDToken(privateKey, "test-key", oidc.RS256, tokenClaims(issuerURL, "client-id", nonce, time.Now().Add(time.Hour)))

	identity, err := provider.Exchange(t.Context(), "code", "verifier-1", nonce)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if identity.Sub != "google-sub-1" || identity.Email != "person@example.com" || identity.Name != "A Person" {
		t.Errorf("identity = %+v", identity)
	}
}

func TestGoogleFlowExchangeRejectsBadTokens(t *testing.T) {
	provider, issuer, privateKey, issuerURL := newTestProvider(t)
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate wrong key: %v", err)
	}
	nonce := "nonce-1"

	tests := []struct {
		name  string
		token string
	}{
		{name: "wrong issuer", token: oidctest.SignIDToken(privateKey, "test-key", oidc.RS256, tokenClaims(issuerURL+"-wrong", "client-id", nonce, time.Now().Add(time.Hour)))},
		{name: "wrong audience", token: oidctest.SignIDToken(privateKey, "test-key", oidc.RS256, tokenClaims(issuerURL, "other-client", nonce, time.Now().Add(time.Hour)))},
		{name: "expired", token: oidctest.SignIDToken(privateKey, "test-key", oidc.RS256, tokenClaims(issuerURL, "client-id", nonce, time.Now().Add(-time.Hour)))},
		{name: "mismatched nonce", token: oidctest.SignIDToken(privateKey, "test-key", oidc.RS256, tokenClaims(issuerURL, "client-id", "wrong", time.Now().Add(time.Hour)))},
		{name: "bad signature", token: oidctest.SignIDToken(wrongKey, "test-key", oidc.RS256, tokenClaims(issuerURL, "client-id", nonce, time.Now().Add(time.Hour)))},
		{name: "missing", token: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuer.idToken = tt.token
			if _, err := provider.Exchange(t.Context(), "code", "verifier-1", nonce); err == nil {
				t.Fatal("Exchange succeeded, want an error")
			}
		})
	}
}

func TestGoogleFlowSecrets(t *testing.T) {
	a, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	b, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if a == b || len(a) < 40 {
		t.Errorf("secrets are not independent/strong enough: %q %q", a, b)
	}
	first, second := Challenge(a), Challenge(a)
	if first == a || first != second {
		t.Errorf("challenge is not deterministic: %q %q", first, second)
	}
}
