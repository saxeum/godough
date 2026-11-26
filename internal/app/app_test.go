package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/saxeum/godough/internal/transport/httpapi"
)

// Every limit is a constant with a name, so this test is what stops one of them
// being quietly dropped back to Go's default of "no limit". No database needed:
// that is why newHTTPServer is separate from New.
func TestNewHTTPServerNamesEveryLimit(t *testing.T) {
	srv := newHTTPServer(":0", http.NotFoundHandler(), slog.New(slog.DiscardHandler))

	if srv.ReadHeaderTimeout <= 0 {
		t.Error("ReadHeaderTimeout is not set")
	}
	if srv.ReadTimeout <= 0 {
		t.Error("ReadTimeout is not set: a slow body would hold the connection forever")
	}
	if srv.WriteTimeout <= 0 {
		t.Error("WriteTimeout is not set")
	}
	if srv.IdleTimeout <= 0 {
		t.Error("IdleTimeout is not set")
	}
	if srv.MaxHeaderBytes <= 0 {
		t.Error("MaxHeaderBytes is not set")
	}
	if srv.ErrorLog == nil {
		t.Error("ErrorLog is nil: Go's server errors would bypass the application's logger")
	}

	// The relationship that matters. If the connection is cut before the handler's
	// own deadline, the handler never gets to say why it gave up.
	if srv.WriteTimeout <= httpapi.RequestTimeout {
		t.Errorf("WriteTimeout %v does not exceed RequestTimeout %v", srv.WriteTimeout, httpapi.RequestTimeout)
	}
	if srv.WriteTimeout < shutdownGrace {
		t.Errorf("WriteTimeout %v is shorter than shutdownGrace %v, so a draining request can be cut", srv.WriteTimeout, shutdownGrace)
	}
}

// TestServeLetsAnInFlightRequestFinish is the half of the lifecycle that #5
// depends on: Shutdown drains rather than cuts. The App is hand-built because
// the point is the drain, not the wiring, and it needs no database.
func TestServeLetsAnInFlightRequestFinish(t *testing.T) {
	entered := make(chan struct{})
	a := &App{
		log: slog.New(slog.DiscardHandler),
		srv: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("finished"))
		})},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	served := make(chan error, 1)
	go func() { served <- a.Serve(ctx, listener) }()

	response := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String() + "/")
		if err != nil {
			response <- "error: " + err.Error()
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		response <- string(body)
	}()

	// Cancel only once the handler is actually running, or there is nothing to
	// drain and the test proves nothing.
	<-entered
	cancel()

	select {
	case body := <-response:
		if body != "finished" {
			t.Errorf("in-flight request got %q, want it to run to completion", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the in-flight request did not complete during the drain")
	}

	select {
	case err := <-served:
		if err != nil {
			t.Errorf("Serve returned %v, want nil after a clean drain", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after Shutdown")
	}
}

// Serve must report a listener that cannot serve rather than blocking until the
// context ends, or a failed bind looks like a hung startup.
func TestServeReturnsWhenTheListenerDies(t *testing.T) {
	a := &App{
		log: slog.New(slog.DiscardHandler),
		srv: &http.Server{Handler: http.NotFoundHandler()},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	served := make(chan error, 1)
	go func() { served <- a.Serve(context.Background(), listener) }()

	select {
	case err := <-served:
		if err == nil {
			t.Error("Serve returned nil for a closed listener, want an error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return for a closed listener")
	}
}
