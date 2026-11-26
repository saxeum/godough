// Package testutil holds the doubles that more than one package's tests need.
package testutil

import (
	"context"
	"slices"

	"github.com/saxeum/godough/internal/domain/snippet"
)

// SnippetRepo is an in-memory service.SnippetRepository.
type SnippetRepo struct {
	Items    []snippet.Snippet
	Err      error
	Inserted []snippet.New
	Deleted  []int64

	nextID int64
}

// NewSnippetRepo seeds the repository. Items are returned in the order given, so
// a caller that cares about ordering has to state it.
func NewSnippetRepo(items ...snippet.Snippet) *SnippetRepo {
	return &SnippetRepo{Items: items}
}

func (r *SnippetRepo) List(context.Context) ([]snippet.Snippet, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Items, nil
}

func (r *SnippetRepo) Get(_ context.Context, id int64) (snippet.Snippet, error) {
	if r.Err != nil {
		return snippet.Snippet{}, r.Err
	}
	idx := slices.IndexFunc(r.Items, func(s snippet.Snippet) bool { return s.ID == id })
	if idx < 0 {
		return snippet.Snippet{}, snippet.ErrNotFound
	}
	return r.Items[idx], nil
}

func (r *SnippetRepo) Insert(_ context.Context, in snippet.New) (snippet.Snippet, error) {
	if r.Err != nil {
		return snippet.Snippet{}, r.Err
	}
	if r.nextID == 0 {
		for _, item := range r.Items {
			r.nextID = max(r.nextID, item.ID)
		}
	}
	r.nextID++

	r.Inserted = append(r.Inserted, in)
	created := snippet.Snippet{ID: r.nextID, Title: in.Title, Content: in.Content}
	r.Items = append([]snippet.Snippet{created}, r.Items...)
	return created, nil
}

func (r *SnippetRepo) Delete(_ context.Context, id int64) error {
	if r.Err != nil {
		return r.Err
	}
	idx := slices.IndexFunc(r.Items, func(s snippet.Snippet) bool { return s.ID == id })
	if idx < 0 {
		return snippet.ErrNotFound
	}
	r.Items = slices.Delete(r.Items, idx, idx+1)
	r.Deleted = append(r.Deleted, id)
	return nil
}
