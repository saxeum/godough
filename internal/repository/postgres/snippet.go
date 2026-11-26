// Package postgres is the storage layer: the only package in the tree that
// knows SQL. It implements service.SnippetRepository and translates pgx's
// vocabulary into the domain's — ErrNoRows becomes snippet.ErrNotFound, and a
// driver error keeps its cause for the log.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saxeum/godough/internal/domain/snippet"
)

// SnippetRepository talks to the snippets table.
type SnippetRepository struct {
	db *pgxpool.Pool
}

func NewSnippetRepository(db *pgxpool.Pool) *SnippetRepository {
	return &SnippetRepository{db: db}
}

// List returns every snippet, newest first. The id breaks the tie: created is
// identical for rows written in the same transaction, which the seed rows are.
func (r *SnippetRepository) List(ctx context.Context) ([]snippet.Snippet, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, title, content, created, expires
		FROM snippets
		ORDER BY created DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query snippets: %w", err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[snippet.Snippet])
	if err != nil {
		return nil, fmt.Errorf("collect snippets: %w", err)
	}

	return items, nil
}

// Get returns one snippet, or snippet.ErrNotFound.
func (r *SnippetRepository) Get(ctx context.Context, id int64) (snippet.Snippet, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, title, content, created, expires
		FROM snippets
		WHERE id = $1`, id)
	if err != nil {
		return snippet.Snippet{}, fmt.Errorf("query snippet %d: %w", id, err)
	}

	item, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[snippet.Snippet])
	if errors.Is(err, pgx.ErrNoRows) {
		return snippet.Snippet{}, snippet.ErrNotFound
	}
	if err != nil {
		return snippet.Snippet{}, fmt.Errorf("collect snippet %d: %w", id, err)
	}

	return item, nil
}

// Insert writes one snippet and returns the row as the database stored it, so
// the caller never has to guess at created and expires.
func (r *SnippetRepository) Insert(ctx context.Context, in snippet.New) (snippet.Snippet, error) {
	rows, err := r.db.Query(ctx, `
		INSERT INTO snippets (title, content, created, expires)
		VALUES ($1, $2, now(), now() + make_interval(days => $3))
		RETURNING id, title, content, created, expires`, in.Title, in.Content, in.ExpiresInDays)
	if err != nil {
		return snippet.Snippet{}, fmt.Errorf("insert snippet: %w", err)
	}

	item, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[snippet.Snippet])
	if err != nil {
		return snippet.Snippet{}, fmt.Errorf("collect inserted snippet: %w", err)
	}

	return item, nil
}

// Delete removes one snippet, or reports snippet.ErrNotFound when it matched
// nothing. The distinction matters: the second DELETE of one row is a 404.
func (r *SnippetRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM snippets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete snippet %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return snippet.ErrNotFound
	}
	return nil
}
