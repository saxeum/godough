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
