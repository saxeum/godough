// Package postgres is the storage layer: the only package in the tree that
// knows SQL. It implements service.SnippetRepository and translates pgx's
// vocabulary into the domain's — ErrNoRows becomes snippet.ErrNotFound, and a
// driver error keeps its cause for the log.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saxeum/godough/internal/domain/snippet"
)

// row is the shape of one snippets row: the columns of the query, in order.
//
// It exists so the scan and the query are in the same file. Scanning straight
// into snippet.Snippet, matching by field name, made
// the domain package implicitly coupled to column names and turned "add a field
// to the domain type" into a runtime failure in a package the author was not
// looking at. Positional scanning into a local struct keeps the coupling here,
// next to the SQL that has to change with it.
type row struct {
	ID      int64
	Title   string
	Content string
	Created time.Time
	Expires time.Time
}

func (r row) toDomain() snippet.Snippet {
	return snippet.Snippet{
		ID:      r.ID,
		Title:   r.Title,
		Content: r.Content,
		Created: r.Created,
		Expires: r.Expires,
	}
}

// SnippetRepository talks to the snippets table.
type SnippetRepository struct {
	db *pgxpool.Pool
}

func NewSnippetRepository(db *pgxpool.Pool) *SnippetRepository {
	return &SnippetRepository{db: db}
}

// List returns one owner's snippets, newest first. The id breaks the tie:
// created is identical for rows written in the same transaction.
func (r *SnippetRepository) List(ctx context.Context, ownerID int64) ([]snippet.Snippet, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, title, content, created, expires
		FROM snippets
		WHERE owner_id = $1
		ORDER BY created DESC, id DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("query snippets: %w", err)
	}

	scanned, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		return nil, fmt.Errorf("collect snippets: %w", err)
	}

	// Allocated, not nil: a nil slice marshals to null and the contract says [].
	items := make([]snippet.Snippet, 0, len(scanned))
	for _, r := range scanned {
		items = append(items, r.toDomain())
	}

	return items, nil
}

// Get returns one of the owner's snippets, or snippet.ErrNotFound. Another
// owner's id is indistinguishable from a missing one.
func (r *SnippetRepository) Get(ctx context.Context, ownerID, id int64) (snippet.Snippet, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, title, content, created, expires
		FROM snippets
		WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return snippet.Snippet{}, fmt.Errorf("query snippet %d: %w", id, err)
	}

	scanned, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[row])
	if errors.Is(err, pgx.ErrNoRows) {
		return snippet.Snippet{}, snippet.ErrNotFound
	}
	if err != nil {
		return snippet.Snippet{}, fmt.Errorf("collect snippet %d: %w", id, err)
	}

	return scanned.toDomain(), nil
}

// Insert writes one snippet for the owner and returns the row as the database
// stored it, so the caller never has to guess at created and expires.
func (r *SnippetRepository) Insert(ctx context.Context, ownerID int64, in snippet.New) (snippet.Snippet, error) {
	rows, err := r.db.Query(ctx, `
		INSERT INTO snippets (owner_id, title, content, created, expires)
		VALUES ($1, $2, $3, now(), now() + make_interval(days => $4))
		RETURNING id, title, content, created, expires`, ownerID, in.Title, in.Content, in.ExpiresInDays)
	if err != nil {
		return snippet.Snippet{}, fmt.Errorf("insert snippet: %w", err)
	}

	scanned, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[row])
	if err != nil {
		return snippet.Snippet{}, fmt.Errorf("collect inserted snippet: %w", err)
	}

	return scanned.toDomain(), nil
}

// Delete removes one of the owner's snippets, or reports snippet.ErrNotFound
// when it matched nothing. The second DELETE of one row is a 404, and so is
// another owner's id.
func (r *SnippetRepository) Delete(ctx context.Context, ownerID, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM snippets WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return fmt.Errorf("delete snippet %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return snippet.ErrNotFound
	}
	return nil
}
