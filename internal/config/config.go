// Package config reads the process environment once, at startup, into a struct.
// Validation happens here rather than at first use, so a missing setting is a
// clear exit at boot instead of a nil pointer under load.
package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// The logging defaults. JSON is the production shape: a log line has to be
// parseable by the collector before anyone can search it. Development sets
// LOG_FORMAT=text in .env, which is the only environment that reads .env.
const (
	defaultLogLevel  = slog.LevelInfo
	defaultLogFormat = "json"
)

// Config is what the process needs to run. There are no defaults for the two
// runtime values: a missing one names itself and exits. The logging values
// have defaults because a missing log setting must not stop the process.
type Config struct {
	Addr        string
	DatabaseURL string
	LogLevel    slog.Level
	LogFormat   string
}

// FromEnv reads and validates the environment.
func FromEnv() (Config, error) {
	addr := os.Getenv("SNIPPET_ADDR")
	if addr == "" {
		return Config{}, errors.New("SNIPPET_ADDR is not set")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return Config{}, errors.New("DATABASE_URL is not set")
	}

	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return Config{}, err
	}

	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	if format == "" {
		format = defaultLogFormat
	}
	switch format {
	case "text", "json":
	default:
		return Config{}, fmt.Errorf("LOG_FORMAT %q: want text or json", os.Getenv("LOG_FORMAT"))
	}

	return Config{Addr: addr, DatabaseURL: dsn, LogLevel: level, LogFormat: format}, nil
}

// Logger is the one place the handler is chosen. The level is applied to the
// handler, not to a wrapper, so a Debug line costs nothing when the level is
// Info.
func (c Config) Logger(w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: c.LogLevel}
	if c.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

// parseLogLevel accepts the values an operator is likely to type. A wrong value
// is an error with the value in it: silently falling back to Info makes a typo
// look like a logging problem later.
func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL %q: want debug, info, warn or error", raw)
	}
}
