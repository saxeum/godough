// Package config reads the process environment once, at startup, into a struct.
// Validation happens here rather than at first use, so a missing setting is a
// clear exit at boot instead of a nil pointer under load.
package config

import (
	"errors"
	"os"
)

// Config is what the process needs to run. There are no defaults: a missing
// value names itself and exits.
type Config struct {
	Addr        string
	DatabaseURL string
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

	return Config{Addr: addr, DatabaseURL: dsn}, nil
}
