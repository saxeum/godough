package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRecovererAnswersAJSON500AndKeepsServing is the two halves of the same
// claim: the request that panicked gets the contract's error shape, and the
// handler is still usable afterwards. The second half is what a naive
// recover-and-return would break, because it would leave the connection hanging.
func TestRecovererAnswersAJSON500AndKeepsServing(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	})
	mux.HandleFunc("GET /fine", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	handler := recoverer(logger, mux)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json — a panic must not be the one text/plain response", got)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	if body["error"] != "internal server error" {
		t.Errorf("body = %v, want the generic message", body)
	}

	if got := logs.String(); !strings.Contains(got, "panic recovered") || !strings.Contains(got, "kaboom") {
		t.Errorf("the panic was not logged with its value:\n%s", got)
	}
	if got := logs.String(); !strings.Contains(got, "goroutine") {
		t.Errorf("the log has no stack trace:\n%s", got)
	}

	// The process is still serving: recovery is per request, not per process.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fine", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("after a panic: %d %q, want 200 %q", rec.Code, rec.Body.String(), "ok")
	}
}

// http.ErrAbortHandler is how a handler asks for the connection to be aborted.
// net/http handles it itself, and swallowing it here would keep alive a
// connection the handler wanted gone.
func TestRecovererLetsAbortHandlerThrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /abort", func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})
	handler := recoverer(slog.New(slog.DiscardHandler), mux)

	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Errorf("recovered %v, want ErrAbortHandler re-panicked", recovered)
		}
	}()

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/abort", nil))
}

// The deadline has to reach the handler as a context, not as a timeout wrapper:
// pgx watches the context and stops the query, where an http.TimeoutHandler
// would answer the client and leave the query running.
func TestRequestDeadlineReachesTheHandler(t *testing.T) {
	var (
		hadDeadline bool
		slack       time.Duration
		endedWith   error
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		var deadline time.Time
		deadline, hadDeadline = r.Context().Deadline()
		slack = time.Until(deadline)
		<-r.Context().Done()
		endedWith = r.Context().Err()
		w.WriteHeader(http.StatusOK)
	})

	handler := requestDeadline(50*time.Millisecond, mux)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slow", nil))

	if !hadDeadline {
		t.Fatal("the handler saw no deadline on its context")
	}
	if slack > time.Second {
		t.Errorf("deadline is %v away, want about the configured 50ms", slack)
	}
	if !errors.Is(endedWith, context.DeadlineExceeded) {
		t.Errorf("context ended with %v, want context.DeadlineExceeded", endedWith)
	}
}

// A caller that hangs up must not be given a fresh deadline: the request is
// already over, and the middleware is not allowed to revive it.
func TestRequestDeadlineRespectsAnEarlierCancellation(t *testing.T) {
	var endedWith error

	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		endedWith = r.Context().Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/slow", nil).WithContext(ctx)

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	requestDeadline(time.Minute, mux).ServeHTTP(httptest.NewRecorder(), req)

	if !errors.Is(endedWith, context.Canceled) {
		t.Errorf("context ended with %v, want the caller's cancellation", endedWith)
	}
}

func TestRequestIDPreservesASafeIncomingID(t *testing.T) {
	var seen string
	handler := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFromContext(r.Context())
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestIDHeader, "upstream-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if seen != "upstream-123" {
		t.Errorf("context request id = %q, want upstream-123", seen)
	}
	if got := rec.Header().Get(requestIDHeader); got != "upstream-123" {
		t.Errorf("response request id = %q, want upstream-123", got)
	}
}

func TestRequestIDGeneratesForMissingOrUnsafeValues(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
	}{
		{name: "missing"},
		{name: "contains a space", incoming: "not a request id"},
		{name: "too long", incoming: strings.Repeat("a", maxRequestIDLength+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			handler := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = requestIDFromContext(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.incoming != "" {
				req.Header.Set(requestIDHeader, tt.incoming)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			got := rec.Header().Get(requestIDHeader)
			if got == "" {
				t.Fatal("no request id was generated")
			}
			if got == tt.incoming {
				t.Errorf("request id %q was accepted, want a generated one", got)
			}
			if !validRequestID(got) {
				t.Errorf("generated id %q is not valid", got)
			}
			if seen != got {
				t.Errorf("context request id = %q, response = %q", seen, got)
			}
		})
	}
}

func TestAccessLogRecordsOneStructuredLine(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /logged", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("nope"))
	})

	handler := withRequestID(accessLog(log, mux))
	req := httptest.NewRequest(http.MethodGet, "/logged", nil)
	req.Header.Set(requestIDHeader, "rid-1")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("access log is not one JSON line: %v (%q)", err, buf.String())
	}

	want := map[string]any{
		"msg":        "request",
		"request_id": "rid-1",
		"method":     "GET",
		"route":      "GET /logged",
		"status":     float64(http.StatusTeapot),
		"bytes":      float64(4),
	}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("%s = %v, want %v", key, line[key], value)
		}
	}
	if _, ok := line["duration"]; !ok {
		t.Error("access log has no duration")
	}
}

func TestAccessLogLevelFollowsTheStatus(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		level string
	}{
		{name: "client error is info", code: http.StatusNotFound, level: "INFO"},
		{name: "server error is error", code: http.StatusInternalServerError, level: "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			handler := accessLog(log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.code)
			}))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

			var line map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
				t.Fatalf("access log is not JSON: %v (%q)", err, buf.String())
			}
			if line["level"] != tt.level {
				t.Errorf("level = %v, want %s", line["level"], tt.level)
			}
		})
	}
}
