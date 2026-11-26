package config

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		addr, dsn   string
		level       string
		format      string
		origins     string
		wantLevel   slog.Level
		wantFormat  string
		wantOrigins []string
		wantErr     string
	}{
		{name: "both set", addr: ":4000", dsn: "postgres://x", wantLevel: slog.LevelInfo, wantFormat: "json"},
		{name: "development logging", addr: ":4000", dsn: "postgres://x", level: "debug", format: "text", wantLevel: slog.LevelDebug, wantFormat: "text"},
		{name: "origins", addr: ":4000", dsn: "postgres://x", origins: "http://localhost:5173, https://app.example.com/", wantLevel: slog.LevelInfo, wantFormat: "json", wantOrigins: []string{"http://localhost:5173", "https://app.example.com"}},
		{name: "duplicate origins", addr: ":4000", dsn: "postgres://x", origins: "http://localhost:8080,http://localhost:8080", wantLevel: slog.LevelInfo, wantFormat: "json", wantOrigins: []string{"http://localhost:8080"}},
		{name: "addr missing", dsn: "postgres://x", wantErr: "SNIPPET_ADDR is not set"},
		{name: "dsn missing", addr: ":4000", wantErr: "DATABASE_URL is not set"},
		{name: "bad level", addr: ":4000", dsn: "postgres://x", level: "bogus", wantErr: `LOG_LEVEL "bogus": want debug, info, warn or error`},
		{name: "bad format", addr: ":4000", dsn: "postgres://x", format: "xml", wantErr: `LOG_FORMAT "xml": want text or json`},
		{name: "bad origin scheme", addr: ":4000", dsn: "postgres://x", origins: "ftp://example.com", wantErr: `TRUSTED_ORIGINS "ftp://example.com": want an origin like https://app.example.com`},
		{name: "origin with a path", addr: ":4000", dsn: "postgres://x", origins: "https://app.example.com/login", wantErr: `TRUSTED_ORIGINS "https://app.example.com/login": want an origin like https://app.example.com`},
		{name: "origin with userinfo", addr: ":4000", dsn: "postgres://x", origins: "https://user@app.example.com", wantErr: `TRUSTED_ORIGINS "https://user@app.example.com": want an origin like https://app.example.com`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SNIPPET_ADDR", tt.addr)
			t.Setenv("DATABASE_URL", tt.dsn)
			t.Setenv("LOG_LEVEL", tt.level)
			t.Setenv("LOG_FORMAT", tt.format)
			t.Setenv("TRUSTED_ORIGINS", tt.origins)
			t.Setenv("GOOGLE_CLIENT_ID", "")
			t.Setenv("GOOGLE_CLIENT_SECRET", "")
			t.Setenv("GOOGLE_REDIRECT_URL", "")

			cfg, err := FromEnv()

			switch {
			case tt.wantErr != "":
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			case cfg.Addr != tt.addr || cfg.DatabaseURL != tt.dsn:
				t.Errorf("got %+v, want Addr=%q DatabaseURL=%q", cfg, tt.addr, tt.dsn)
			case cfg.LogLevel != tt.wantLevel || cfg.LogFormat != tt.wantFormat:
				t.Errorf("logging = %v/%q, want %v/%q", cfg.LogLevel, cfg.LogFormat, tt.wantLevel, tt.wantFormat)
			case !reflect.DeepEqual(cfg.TrustedOrigins, tt.wantOrigins):
				t.Errorf("origins = %v, want %v", cfg.TrustedOrigins, tt.wantOrigins)
			}
		})
	}
}

// The handler, not a wrapper, carries the level: a Debug line must not be
// formatted at all when the level is Info.
func TestLoggerUsesTheConfiguredLevelAndFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{LogLevel: slog.LevelWarn, LogFormat: "json"}
	log := cfg.Logger(&buf)

	log.Info("dropped")
	log.Warn("kept", "request_id", "abc")

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, buf.String())
	}
	if line["msg"] != "kept" {
		t.Errorf("msg = %v, want kept; Info should have been dropped", line["msg"])
	}
	if line["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", line["level"])
	}
	if line["request_id"] != "abc" {
		t.Errorf("request_id = %v, want abc", line["request_id"])
	}
}

func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{LogLevel: slog.LevelInfo, LogFormat: "text"}
	log := cfg.Logger(&buf)
	log.Info("hello", "key", "value")

	got := buf.String()
	if !strings.Contains(got, "msg=hello") || !strings.Contains(got, "key=value") {
		t.Errorf("text log = %q, want msg=hello and key=value", got)
	}
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Errorf("text log looks like JSON: %q", got)
	}
}

func TestGoogleOAuthConfig(t *testing.T) {
	tests := []struct {
		name                 string
		id, secret, redirect string
		wantErr              string
		wantConfigured       bool
	}{
		{name: "disabled", wantConfigured: false},
		{name: "complete", id: "id", secret: "secret", redirect: "https://app.example.com/api/auth/google/callback", wantConfigured: true},
		{name: "partial", id: "id", wantErr: "GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and GOOGLE_REDIRECT_URL must be set together"},
		{name: "not absolute", id: "id", secret: "secret", redirect: "/api/auth/google/callback", wantErr: `GOOGLE_REDIRECT_URL "/api/auth/google/callback": want an absolute http(s) URL without a fragment`},
		{name: "origin only", id: "id", secret: "secret", redirect: "https://app.example.com", wantErr: `GOOGLE_REDIRECT_URL "https://app.example.com": want the /api/auth/google/callback path`},
		{name: "wrong path", id: "id", secret: "secret", redirect: "https://app.example.com/login", wantErr: `GOOGLE_REDIRECT_URL "https://app.example.com/login": want the /api/auth/google/callback path`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SNIPPET_ADDR", ":4000")
			t.Setenv("DATABASE_URL", "postgres://x")
			t.Setenv("GOOGLE_CLIENT_ID", tt.id)
			t.Setenv("GOOGLE_CLIENT_SECRET", tt.secret)
			t.Setenv("GOOGLE_REDIRECT_URL", tt.redirect)

			cfg, err := FromEnv()
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Google.Configured() != tt.wantConfigured {
				t.Errorf("Configured = %v, want %v", cfg.Google.Configured(), tt.wantConfigured)
			}
		})
	}
}
