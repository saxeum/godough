// Package snippet is the snippet entity and the errors the layers above it are
// allowed to see. It imports nothing but the standard library: this is the
// bottom of the dependency graph, so a change up here cannot drag a driver or an
// HTTP concern down with it.
package snippet

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrNotFound is returned when no snippet has the requested id. The transport
// turns it into 404; nothing above this package needs to know how.
var ErrNotFound = errors.New("snippet not found")

// The limits the API publishes. They live here because both the service that
// enforces them and the tests that assert them need the same numbers, and
// because spec/openapi.yaml declares the same values in prose and constraints.
const (
	TitleMaxRunes = 100
	ExpiryMinDays = 1
	ExpiryMaxDays = 365
)

// Snippet is one stored snippet. Created and Expires are set by the database, so
// a caller never guesses at them.
type Snippet struct {
	ID      int64
	Title   string
	Content string
	Created time.Time
	Expires time.Time
}

// New is the input to Create. It is a separate type from Snippet because a
// caller cannot choose an id or a timestamp.
type New struct {
	Title         string
	Content       string
	ExpiresInDays int
}

// ValidationError carries every rejected field at once, so a form can show them
// all rather than one per round trip. The names are the JSON names, not the Go
// ones: this type is serialised directly.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("invalid fields: %v", keys)
}
