package config

import "testing"

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		addr, dsn string
		wantErr   string
	}{
		{name: "both set", addr: ":4000", dsn: "postgres://x"},
		{name: "addr missing", dsn: "postgres://x", wantErr: "SNIPPET_ADDR is not set"},
		{name: "dsn missing", addr: ":4000", wantErr: "DATABASE_URL is not set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SNIPPET_ADDR", tt.addr)
			t.Setenv("DATABASE_URL", tt.dsn)

			cfg, err := FromEnv()

			switch {
			case tt.wantErr != "":
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			case cfg.Addr != tt.addr || cfg.DatabaseURL != tt.dsn:
				t.Errorf("got %+v, want Addr=%q DatabaseURL=%q", cfg, tt.addr, tt.dsn)
			}
		})
	}
}
