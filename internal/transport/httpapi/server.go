package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/saxeum/godough/internal/domain/snippet"
	"github.com/saxeum/godough/internal/service"
)

// Server implements StrictServerInterface, the generated interface that the
// contract defines. A documented status code that no branch produces does not
// compile, which is the reason to generate the interface rather than write the
// handlers by hand.
type Server struct {
	log            *slog.Logger
	snippets       *service.Snippets
	sessions       SessionService
	users          UserService
	google         GoogleAuth
	trustedOrigins []string
	// ready is the readiness check. It is a function rather than a pool so this
	// package does not import a driver.
	ready func(ctx context.Context) error
}

// Deps is the composition root's view of the transport. A nil Google means
// sign-in is not configured; the rest are required.
type Deps struct {
	Log            *slog.Logger
	Snippets       *service.Snippets
	Sessions       SessionService
	Users          UserService
	Google         GoogleAuth
	TrustedOrigins []string
	Ready          func(ctx context.Context) error
}

func NewServer(deps Deps) *Server {
	return &Server{
		log:            deps.Log,
		snippets:       deps.Snippets,
		sessions:       deps.Sessions,
		users:          deps.Users,
		google:         deps.Google,
		trustedOrigins: deps.TrustedOrigins,
		ready:          deps.Ready,
	}
}

// Routes builds the mux. The generated handler is mounted on the same mux as the
// probes, so there is one server and one set of routes; the API's paths carry
// the /api prefix in the spec, so nothing is rewritten here.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.HandleFunc("GET /api/auth/google", s.googleStart)
	mux.HandleFunc("GET /api/auth/google/callback", s.googleCallback)
	mux.Handle("GET /api/auth/session", requireSession(s.sessions, s.log, http.HandlerFunc(s.authSession)))
	mux.Handle("POST /api/auth/signout", requireSession(s.sessions, s.log,
		requireTrustedOrigin(s.trustedOrigins, s.log, http.HandlerFunc(s.authSignout))))

	// Two callbacks, because the generated router fails in two places: a body
	// that will not decode (strict options) and a path or query parameter that
	// will not parse (the wrapper's own ErrorHandlerFunc). Both must answer in
	// the API's JSON shape, or a malformed id is the only text/plain response
	// the service ever sends.
	badRequest := func(w http.ResponseWriter, r *http.Request, err error) {
		// MaxBytesReader cutting the body off is a 413, not the 400 a malformed
		// body gets. Same callback, so the distinction has to be made here.
		if isBodyTooLarge(err) {
			s.log.Warn("request body too large", "request_id", requestIDFromContext(r.Context()), "limit", maxBodyBytes, "method", r.Method, "uri", r.URL.RequestURI())
			writeJSON(w, http.StatusRequestEntityTooLarge, errorBody(http.StatusRequestEntityTooLarge))
			return
		}
		s.log.Warn("bad request", "request_id", requestIDFromContext(r.Context()), "err", err, "method", r.Method, "uri", r.URL.RequestURI())
		writeJSON(w, http.StatusBadRequest, errorBody(http.StatusBadRequest))
	}

	strict := NewStrictHandlerWithOptions(s, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc: badRequest,
		// A handler returned an error: only unexpected failures reach here, since
		// the documented ones are returned as response objects.
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			// A client that hung up is not a fault. Writing to its connection is
			// pointless and logging it at ERROR makes the error rate measure
			// browsers navigating away rather than anything going wrong.
			if errors.Is(err, context.Canceled) {
				s.log.Info("request abandoned by the client", "request_id", requestIDFromContext(r.Context()), "method", r.Method, "uri", r.URL.RequestURI())
				return
			}
			// A request that ran out of its own deadline is the server's failure,
			// not the client's, so it stays a 500 — with the distinction in the
			// log, because "deadline exceeded" alone does not say which deadline.
			if errors.Is(err, context.DeadlineExceeded) {
				s.log.Error("request deadline exceeded", "request_id", requestIDFromContext(r.Context()), "err", err, "timeout", RequestTimeout, "method", r.Method, "uri", r.URL.RequestURI())
				writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
				return
			}
			s.log.Error("server error", "request_id", requestIDFromContext(r.Context()), "err", err, "method", r.Method, "uri", r.URL.RequestURI())
			writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		},
	})

	// The snippet routes are the only protected ones. A small mux lets the
	// session and origin checks wrap exactly those paths, while the probes stay
	// public and the generated router keeps its own route table.
	snippetMux := http.NewServeMux()
	api := HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseRouter:       snippetMux,
		ErrorHandlerFunc: badRequest,
	})

	protected := requireSession(s.sessions, s.log,
		requireTrustedOrigin(s.trustedOrigins, s.log, bodyGuards(api)))
	mux.Handle("/api/snippet", protected)
	mux.Handle("/api/snippet/", protected)

	// Outside, not inside: the generated handler reads the body before any of its
	// own middleware sees the request. Outermost first, so a panic anywhere below
	// is recovered, and the deadline covers everything that follows. noStore sits
	// innermost so every response the mux can produce — including the ones the
	// generated handlers write themselves — carries the directive.
	return withRequestID(accessLog(s.log, recoverer(s.log, requestDeadline(RequestTimeout, noStore(mux)))))
}

// healthz says the process is alive. It must not touch the database: a database
// blip is not a reason to restart the container.
func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok\n"))
}

// readyz says the server can serve real traffic, so it does touch the database.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := s.ready(ctx); err != nil {
		s.log.Error("readiness check failed", "request_id", requestIDFromContext(r.Context()), "err", err)
		writeJSON(w, http.StatusServiceUnavailable, errorBody(http.StatusServiceUnavailable))
		return
	}

	_, _ = w.Write([]byte("ok\n"))
}

// ListSnippets implements GET /api/snippet.
func (s *Server) ListSnippets(ctx context.Context, _ ListSnippetsRequestObject) (ListSnippetsResponseObject, error) {
	ownerID, ok := sessionUserID(ctx)
	if !ok {
		return nil, errors.New("snippet route reached without a session")
	}
	items, err := s.snippets.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	return ListSnippets200JSONResponse(toWireAll(items)), nil
}

// GetSnippet implements GET /api/snippet/{id}.
func (s *Server) GetSnippet(ctx context.Context, request GetSnippetRequestObject) (GetSnippetResponseObject, error) {
	ownerID, ok := sessionUserID(ctx)
	if !ok {
		return nil, errors.New("snippet route reached without a session")
	}
	item, err := s.snippets.Get(ctx, ownerID, request.Id)
	switch {
	case errors.Is(err, snippet.ErrNotFound):
		return GetSnippet404JSONResponse{NotFoundJSONResponse: NotFoundJSONResponse(errorBody(http.StatusNotFound))}, nil
	case err != nil:
		return nil, err
	}
	return GetSnippet200JSONResponse(toWire(item)), nil
}

// CreateSnippet implements POST /api/snippet.
func (s *Server) CreateSnippet(ctx context.Context, request CreateSnippetRequestObject) (CreateSnippetResponseObject, error) {
	ownerID, ok := sessionUserID(ctx)
	if !ok {
		return nil, errors.New("snippet route reached without a session")
	}
	created, err := s.snippets.Create(ctx, ownerID, snippet.New{
		Title:         request.Body.Title,
		Content:       request.Body.Content,
		ExpiresInDays: request.Body.ExpiresInDays,
	})

	var invalid *snippet.ValidationError
	switch {
	case errors.As(err, &invalid):
		return CreateSnippet422JSONResponse{
			ValidationErrorJSONResponse: ValidationErrorJSONResponse{Errors: invalid.Fields},
		}, nil
	case err != nil:
		return nil, err
	}

	s.log.Info("snippet created", "request_id", requestIDFromContext(ctx), "id", created.ID)
	location := fmt.Sprintf("/api/snippet/%d", created.ID)
	return CreateSnippet201JSONResponse{
		Body:    toWire(created),
		Headers: CreateSnippet201ResponseHeaders{Location: &location},
	}, nil
}

// DeleteSnippet implements DELETE /api/snippet/{id}.
func (s *Server) DeleteSnippet(ctx context.Context, request DeleteSnippetRequestObject) (DeleteSnippetResponseObject, error) {
	ownerID, ok := sessionUserID(ctx)
	if !ok {
		return nil, errors.New("snippet route reached without a session")
	}
	err := s.snippets.Delete(ctx, ownerID, request.Id)
	switch {
	case errors.Is(err, snippet.ErrNotFound):
		return DeleteSnippet404JSONResponse{NotFoundJSONResponse: NotFoundJSONResponse(errorBody(http.StatusNotFound))}, nil
	case err != nil:
		return nil, err
	}

	s.log.Info("snippet deleted", "request_id", requestIDFromContext(ctx), "id", request.Id)
	return DeleteSnippet204Response{}, nil
}
