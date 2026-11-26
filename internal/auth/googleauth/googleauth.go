// Package googleauth is the server-side Google authorization-code flow. No
// Google JavaScript runs on the page: the browser is redirected, the code is
// exchanged here, and the ID token is verified against Google's JWKS.
package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const defaultIssuer = "https://accounts.google.com"

// Identity is what the session needs from a verified ID token. Sub is the
// stable key; email and name are display values.
type Identity struct {
	Sub   string
	Email string
	Name  string
}

// Provider owns the OAuth client and the ID-token verifier.
type Provider struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// New discovers the issuer and builds the client. The issuer is a parameter so
// tests can point the whole flow at an httptest server; production passes the
// Google issuer.
func New(ctx context.Context, clientID, clientSecret, redirectURL, issuer string) (*Provider, error) {
	if issuer == "" {
		issuer = defaultIssuer
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", issuer, err)
	}

	return &Provider{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// AuthCodeURL builds the redirect. PKCE binds the code to this browser even if
// the code leaks; the nonce binds the ID token to this start request.
func (p *Provider) AuthCodeURL(state, nonce, verifier string) string {
	return p.oauth.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", Challenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange trades the code for tokens, verifies the ID token, and returns the
// identity. The nonce is checked here, not by the caller.
func (p *Provider) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	token, err := p.oauth.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return Identity{}, errors.New("token response has no id_token")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id_token: %w", err)
	}
	if idToken.Nonce != nonce {
		return Identity{}, errors.New("id_token nonce does not match")
	}

	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("decode id_token claims: %w", err)
	}
	if claims.Sub == "" {
		return Identity{}, errors.New("id_token has no subject")
	}

	return Identity{Sub: claims.Sub, Email: claims.Email, Name: claims.Name}, nil
}

// NewSecret returns a URL-safe random value for state, nonce or the PKCE
// verifier. Thirty-two bytes gives 256 bits of entropy.
func NewSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// Challenge is the S256 PKCE challenge for a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
