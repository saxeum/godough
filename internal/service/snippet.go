// Package service holds the rules: what a valid snippet is, and what the API
// does about one that is not. It knows nothing about HTTP and nothing about
// Postgres, which is what makes it testable without either.
package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/saxeum/godough/internal/domain/snippet"
)

// SnippetRepository is the whole of what the service needs from storage. It is
// declared here, where it is consumed rather than where it is implemented, so
// the service can be exercised with a map and the Postgres package cannot leak a
// query detail upward.
type SnippetRepository interface {
	List(ctx context.Context) ([]snippet.Snippet, error)
	Get(ctx context.Context, id int64) (snippet.Snippet, error)
	Insert(ctx context.Context, in snippet.New) (snippet.Snippet, error)
	Delete(ctx context.Context, id int64) error
}

// SnippetService is the business layer for snippets.
type SnippetService struct {
	repo SnippetRepository
}

func NewSnippetService(repo SnippetRepository) *SnippetService {
	return &SnippetService{repo: repo}
}

// List returns every stored snippet, newest first.
func (s *SnippetService) List(ctx context.Context) ([]snippet.Snippet, error) {
	return s.repo.List(ctx)
}

// Get returns one snippet, or snippet.ErrNotFound.
func (s *SnippetService) Get(ctx context.Context, id int64) (snippet.Snippet, error) {
	if id < 1 {
		return snippet.Snippet{}, snippet.ErrNotFound
	}
	return s.repo.Get(ctx, id)
}

// Create validates the input and stores it, returning the row as the database
// wrote it.
func (s *SnippetService) Create(ctx context.Context, in snippet.New) (snippet.Snippet, error) {
	if bad := validate(in); len(bad) > 0 {
		return snippet.Snippet{}, &snippet.ValidationError{Fields: bad}
	}
	return s.repo.Insert(ctx, in)
}

// Delete removes one snippet, or reports snippet.ErrNotFound.
func (s *SnippetService) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return snippet.ErrNotFound
	}
	return s.repo.Delete(ctx, id)
}

// validate reports every bad field at once. The keys are the JSON names the
// client sent, because they are rendered under the matching input.
func validate(in snippet.New) map[string]string {
	bad := map[string]string{}

	switch {
	case strings.TrimSpace(in.Title) == "":
		bad["title"] = "is required"
	case utf8.RuneCountInString(in.Title) > snippet.TitleMaxRunes:
		bad["title"] = fmt.Sprintf("must be at most %d characters", snippet.TitleMaxRunes)
	}

	if strings.TrimSpace(in.Content) == "" {
		bad["content"] = "is required"
	}

	if in.ExpiresInDays < snippet.ExpiryMinDays || in.ExpiresInDays > snippet.ExpiryMaxDays {
		bad["expiresInDays"] = fmt.Sprintf("must be between %d and %d", snippet.ExpiryMinDays, snippet.ExpiryMaxDays)
	}

	return bad
}
