package postgres

import "testing"

// TestTheAppRoleCanDoOnlyWhatTheAppDoes is the check that #39 exists for: before
// it, the grants came from a Docker-only hook, so nothing in CI had ever run
// with the privileges the application actually has.
func TestTheAppRoleCanDoOnlyWhatTheAppDoes(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()

	var currentUser string
	if err := pool.QueryRow(ctx, "SELECT current_user").Scan(&currentUser); err != nil {
		t.Fatalf("select current_user: %v", err)
	}
	if currentUser != "godough_app" {
		t.Fatalf("connected as %q, want godough_app: this test is about the grants the application actually has, so running it as the owner proves nothing", currentUser)
	}

	for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		var granted bool
		if err := pool.QueryRow(ctx, "SELECT has_table_privilege(current_user, 'snippets', $1)", privilege).Scan(&granted); err != nil {
			t.Fatalf("has_table_privilege(%s): %v", privilege, err)
		}
		if !granted {
			t.Errorf("the application role cannot %s snippets", privilege)
		}
	}

	denied := []struct {
		what  string
		query string
	}{
		{"TRUNCATE snippets", "SELECT has_table_privilege(current_user, 'snippets', 'TRUNCATE')"},
		{"REFERENCES on snippets", "SELECT has_table_privilege(current_user, 'snippets', 'REFERENCES')"},
		{"CREATE in schema public", "SELECT has_schema_privilege(current_user, 'public', 'CREATE')"},
		{"CREATE on the database", "SELECT has_database_privilege(current_user, current_database(), 'CREATE')"},
	}

	for _, d := range denied {
		var granted bool
		if err := pool.QueryRow(ctx, d.query).Scan(&granted); err != nil {
			t.Fatalf("%s: %v", d.what, err)
		}
		if granted {
			t.Errorf("the application role can %s", d.what)
		}
	}

	// DROP TABLE is ownership, not a grant, so it needs its own check.
	var owner string
	if err := pool.QueryRow(ctx, `
		SELECT tableowner FROM pg_tables
		WHERE schemaname = 'public' AND tablename = 'snippets'`).Scan(&owner); err != nil {
		t.Fatalf("select tableowner: %v", err)
	}
	if owner == currentUser {
		t.Errorf("the application role owns snippets, so it can drop it")
	}
}
