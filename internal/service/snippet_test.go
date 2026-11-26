package service

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/saxeum/godough/internal/domain/snippet"
	"github.com/saxeum/godough/internal/testutil"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		in   snippet.New
		want []string // field names expected in the error; empty means valid
	}{
		{name: "valid", in: snippet.New{Title: "t", Content: "c", ExpiresInDays: 7}},
		{name: "title at the limit", in: snippet.New{Title: strings.Repeat("a", snippet.TitleMaxRunes), Content: "c", ExpiresInDays: 7}},
		{name: "one rune over the title limit", in: snippet.New{Title: strings.Repeat("a", snippet.TitleMaxRunes+1), Content: "c", ExpiresInDays: 7}, want: []string{"title"}},
		{name: "multibyte title at the limit", in: snippet.New{Title: strings.Repeat("é", snippet.TitleMaxRunes), Content: "c", ExpiresInDays: 7}},
		{name: "empty title", in: snippet.New{Title: "", Content: "c", ExpiresInDays: 7}, want: []string{"title"}},
		{name: "whitespace title", in: snippet.New{Title: "   ", Content: "c", ExpiresInDays: 7}, want: []string{"title"}},
		{name: "empty content", in: snippet.New{Title: "t", Content: "", ExpiresInDays: 7}, want: []string{"content"}},
		{name: "whitespace content", in: snippet.New{Title: "t", Content: "\n\t ", ExpiresInDays: 7}, want: []string{"content"}},
		{name: "zero days", in: snippet.New{Title: "t", Content: "c", ExpiresInDays: 0}, want: []string{"expiresInDays"}},
		{name: "negative days", in: snippet.New{Title: "t", Content: "c", ExpiresInDays: -1}, want: []string{"expiresInDays"}},
		{name: "one day", in: snippet.New{Title: "t", Content: "c", ExpiresInDays: snippet.ExpiryMinDays}},
		{name: "one year", in: snippet.New{Title: "t", Content: "c", ExpiresInDays: snippet.ExpiryMaxDays}},
		{name: "one day over", in: snippet.New{Title: "t", Content: "c", ExpiresInDays: snippet.ExpiryMaxDays + 1}, want: []string{"expiresInDays"}},
		{
			name: "everything missing",
			in:   snippet.New{},
			want: []string{"content", "expiresInDays", "title"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validate(tt.in)

			keys := make([]string, 0, len(got))
			for k := range got {
				keys = append(keys, k)
			}
			slices.Sort(keys)

			if !slices.Equal(keys, tt.want) {
				t.Errorf("fields = %v, want %v (got %v)", keys, tt.want, got)
			}
		})
	}
}

func TestCreateRejectsAndDoesNotWrite(t *testing.T) {
	repo := &testutil.SnippetRepo{}
	svc := NewSnippets(repo)

	_, err := svc.Create(t.Context(), 1, snippet.New{Title: "", Content: "c", ExpiresInDays: 7})

	var invalid *snippet.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want *snippet.ValidationError", err)
	}
	if _, ok := invalid.Fields["title"]; !ok {
		t.Errorf("fields = %v, want a title entry", invalid.Fields)
	}
	if len(repo.Inserted) != 0 {
		t.Errorf("inserted %d rows for a rejected request, want 0", len(repo.Inserted))
	}
}

func TestCreatePassesInputThrough(t *testing.T) {
	repo := &testutil.SnippetRepo{}
	svc := NewSnippets(repo)

	in := snippet.New{Title: "  padded  ", Content: "body", ExpiresInDays: 3}
	if _, err := svc.Create(t.Context(), 1, in); err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(repo.Inserted) != 1 {
		t.Fatalf("inserted %d rows, want 1", len(repo.Inserted))
	}
	// Validation trims to decide emptiness; the stored value is what was sent.
	if repo.Inserted[0] != in {
		t.Errorf("inserted %+v, want %+v", repo.Inserted[0], in)
	}
}

func TestGetAndDeleteRejectNonPositiveIDsWithoutTouchingTheRepository(t *testing.T) {
	repo := testutil.NewSnippetRepo(snippet.Snippet{ID: 1})
	svc := NewSnippets(repo)

	if _, err := svc.Get(t.Context(), 1, 0); !errors.Is(err, snippet.ErrNotFound) {
		t.Errorf("Get(0) err = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(t.Context(), 1, -5); !errors.Is(err, snippet.ErrNotFound) {
		t.Errorf("Delete(-5) err = %v, want ErrNotFound", err)
	}
	if len(repo.Deleted) != 0 {
		t.Errorf("deleted %v, want nothing", repo.Deleted)
	}
}

func TestRepositoryErrorsArePassedThrough(t *testing.T) {
	sentinel := errors.New("database is on fire")
	svc := NewSnippets(&testutil.SnippetRepo{Err: sentinel})

	if _, err := svc.List(t.Context(), 1); !errors.Is(err, sentinel) {
		t.Errorf("List err = %v, want the repository's error unchanged", err)
	}
}
