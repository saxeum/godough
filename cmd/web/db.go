package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var errNotFound = errors.New("not found")

type snippet struct {
	ID      int
	Title   string
	Content string
	Created time.Time
	Expires time.Time
}

// snippetsAll returns every snippet, newest first.
func (app *application) snippetsAll(ctx context.Context) ([]snippet, error) {
	rows, err := app.db.Query(ctx, `
		SELECT id, title, content, created, expires
		FROM snippets
		ORDER BY created DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query snippets: %w", err)
	}

	snippets, err := pgx.CollectRows(rows, pgx.RowToStructByName[snippet])
	if err != nil {
		return nil, fmt.Errorf("collect snippets: %w", err)
	}

	return snippets, nil
}

// snippetInsert writes one snippet and returns the row as the database stored
// it, so the caller never has to guess at created/expires.
func (app *application) snippetInsert(ctx context.Context, title, content string, expiresInDays int) (snippet, error) {
	rows, err := app.db.Query(ctx, `
		INSERT INTO snippets (title, content, created, expires)
		VALUES ($1, $2, now(), now() + make_interval(days => $3))
		RETURNING id, title, content, created, expires`, title, content, expiresInDays)
	if err != nil {
		return snippet{}, fmt.Errorf("insert snippet: %w", err)
	}

	s, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[snippet])
	if err != nil {
		return snippet{}, fmt.Errorf("collect inserted snippet: %w", err)
	}

	return s, nil
}

// snippetDeleteByID removes one snippet, or reports errNotFound.
func (app *application) snippetDeleteByID(ctx context.Context, id int) error {
	tag, err := app.db.Exec(ctx, `DELETE FROM snippets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete snippet %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// snippetGet returns one snippet, or errNotFound.
func (app *application) snippetGet(ctx context.Context, id int) (snippet, error) {
	rows, err := app.db.Query(ctx, `
		SELECT id, title, content, created, expires
		FROM snippets
		WHERE id = $1`, id)
	if err != nil {
		return snippet{}, fmt.Errorf("query snippet %d: %w", id, err)
	}

	s, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[snippet])
	if errors.Is(err, pgx.ErrNoRows) {
		return snippet{}, errNotFound
	}
	if err != nil {
		return snippet{}, fmt.Errorf("collect snippet %d: %w", id, err)
	}

	return s, nil
}
