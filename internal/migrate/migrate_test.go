package migrate

import (
	"slices"
	"testing"
	"testing/fstest"
)

func TestDiscoverOrdersByVersionNotByName(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_second.sql": {Data: []byte("select 2;")},
		"0010_tenth.sql":  {Data: []byte("select 10;")},
		"0001_first.sql":  {Data: []byte("select 1;")},
		"notes.txt":       {Data: []byte("ignored")},
	}

	got, err := Discover(fsys, ".")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	var versions []int64
	for _, m := range got {
		versions = append(versions, m.Version)
	}
	if want := []int64{1, 2, 10}; !slices.Equal(versions, want) {
		t.Errorf("versions = %v, want %v — string order would put 10 before 2", versions, want)
	}
	if len(got) != 3 {
		t.Errorf("got %d migrations, want 3: a non-.sql file is not one", len(got))
	}
}

func TestDiscoverRejectsBadNames(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "no prefix", file: "create_snippets.sql"},
		{name: "not a number", file: "abc_create.sql"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Discover(fstest.MapFS{tt.file: {Data: []byte("select 1;")}}, "."); err == nil {
				t.Fatalf("Discover accepted %q, want an error", tt.file)
			}
		})
	}
}

func TestDiscoverRejectsDuplicateVersions(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_a.sql": {Data: []byte("select 1;")},
		"0001_b.sql": {Data: []byte("select 2;")},
	}

	if _, err := Discover(fsys, "."); err == nil {
		t.Fatal("two files with version 1 were accepted; which one ran first would be undefined")
	}
}

func TestPending(t *testing.T) {
	all := []Migration{{Version: 1}, {Version: 2}, {Version: 3}}

	got := Pending(all, map[int64]bool{1: true})
	if want := []int64{2, 3}; !slices.Equal(versions(got), want) {
		t.Errorf("pending = %v, want %v", versions(got), want)
	}

	if got := Pending(all, map[int64]bool{1: true, 2: true, 3: true}); len(got) != 0 {
		t.Errorf("pending = %v, want none", versions(got))
	}

	// A version recorded in the database that no longer has a file is not an
	// error here: files are never removed, and detecting it is out of scope.
	if got := Pending(all, map[int64]bool{99: true}); len(got) != 3 {
		t.Errorf("pending = %v, want all three", versions(got))
	}
}

func versions(ms []Migration) []int64 {
	var out []int64
	for _, m := range ms {
		out = append(out, m.Version)
	}
	return out
}
