package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

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
