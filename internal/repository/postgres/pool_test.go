package postgres

import (
	"path/filepath"
	"strings"
	"testing"
)

// A missing CA is a boot failure, not a quiet fallback to plaintext. NewPool
// builds the TLS config, so the refusal happens before the process can serve.
func TestNewPoolRefusesAMissingCA(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.pem")
	dsn := "postgres://godough_app:x@127.0.0.1:1/snippet?sslmode=verify-full&sslrootcert=" + missing

	_, err := NewPool(t.Context(), dsn)
	if err == nil {
		t.Fatal("NewPool accepted a verify-full DSN with no CA file")
	}
	if !strings.Contains(err.Error(), "CA file") && !strings.Contains(strings.ToLower(err.Error()), "certificate") {
		t.Errorf("err = %v, want it to name the CA or certificate", err)
	}
}
