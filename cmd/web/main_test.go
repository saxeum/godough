package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestRunServesAndShutsDown drives the whole of run: the environment, the
// config, the boot deadline, the wiring, the listener and the drain.
//
// It cancels a context rather than signalling the process. Sending SIGTERM to
// your own test process is a test that can take the suite with it, and the
// signal wiring in main is two lines of signal.NotifyContext.
func TestRunServesAndShutsDown(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a database: make db-up db-migrate, then make test")
	}
	if os.Getenv("DATABASE_URL") == "" {
		t.Fatal("DATABASE_URL is not set: copy .env.example to .env")
	}

	addr := freeAddr(t)
	t.Setenv("SNIPPET_ADDR", addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- run(ctx, slog.New(slog.DiscardHandler)) }()

	waitForHealthz(t, done, "http://"+addr+"/healthz")

	// Everything that matters about shutdown happens after this line.
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned %v, want nil after a clean shutdown", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("run did not return after its context was cancelled")
	}

	// The port is released, so a restart in the same sandbox would bind.
	if conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond); err == nil {
		conn.Close()
		t.Errorf("%s is still accepting connections after run returned", addr)
	}
}

// freeAddr reserves a port and gives it back. There is a window between the
// close and the bind, which is why the runner does not do this in parallel.
func freeAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}
	return addr
}

// waitForHealthz polls until the server answers, and breaks the moment it does,
// the process exits, or the budget runs out — never on a fixed sleep.
func waitForHealthz(t *testing.T, done <-chan error, url string) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(15 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("run exited before it served: %v", err)
		default:
		}

		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("%s did not answer 200 within 15s", url)
}
