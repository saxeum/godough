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
	log      *slog.Logger
	snippets *service.SnippetService
	// ready is the readiness check. It is a function rather than a pool so this
	// package does not import a driver.
	ready func(ctx context.Context) error
}

func NewServer(log *slog.Logger, snippets *service.SnippetService, ready func(ctx context.Context) error) *Server {
	return &Server{log: log, snippets: snippets, ready: ready}
}

// Routes builds the mux. The generated handler is mounted on the same mux as the
// probes, so there is one server and one set of routes; the API's paths carry
// the /api prefix in the spec, so nothing is rewritten here.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)

	// Two callbacks, because the generated router fails in two places: a body
	// that will not decode (strict options) and a path or query parameter that
	// will not parse (the wrapper's own ErrorHandlerFunc). Both must answer in
	// the API's JSON shape, or a malformed id is the only text/plain response
	// the service ever sends.
	badRequest := func(w http.ResponseWriter, r *http.Request, err error) {
		// MaxBytesReader cutting the body off is a 413, not the 400 a malformed
		// body gets. Same callback, so the distinction has to be made here.
		if isBodyTooLarge(err) {
			s.log.Warn("request body too large", "limit", maxBodyBytes, "method", r.Method, "uri", r.URL.RequestURI())
			writeJSON(w, http.StatusRequestEntityTooLarge, errorBody(http.StatusRequestEntityTooLarge))
			return
		}
		s.log.Warn("bad request", "err", err, "method", r.Method, "uri", r.URL.RequestURI())
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
				s.log.Info("request abandoned by the client", "method", r.Method, "uri", r.URL.RequestURI())
				return
			}
			s.log.Error("server error", "err", err, "method", r.Method, "uri", r.URL.RequestURI())
			writeJSON(w, http.StatusInternalServerError, errorBody(http.StatusInternalServerError))
		},
	})

	api := HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: badRequest,
	})

	// Outside, not inside: the generated handler reads the body before any of its
	// own middleware sees the request.
	return bodyGuards(api)
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
		s.log.Error("readiness check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, errorBody(http.StatusServiceUnavailable))
		return
	}

	_, _ = w.Write([]byte("ok\n"))
}

// ListSnippets implements GET /api/snippet.
func (s *Server) ListSnippets(ctx context.Context, _ ListSnippetsRequestObject) (ListSnippetsResponseObject, error) {
	items, err := s.snippets.List(ctx)
	if err != nil {
		return nil, err
	}
	return ListSnippets200JSONResponse(toWireAll(items)), nil
}

// GetSnippet implements GET /api/snippet/{id}.
func (s *Server) GetSnippet(ctx context.Context, request GetSnippetRequestObject) (GetSnippetResponseObject, error) {
	item, err := s.snippets.Get(ctx, request.Id)
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
	created, err := s.snippets.Create(ctx, snippet.New{
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

	s.log.Info("snippet created", "id", created.ID)
	location := fmt.Sprintf("/api/snippet/%d", created.ID)
	return CreateSnippet201JSONResponse{
		Body:    toWire(created),
		Headers: CreateSnippet201ResponseHeaders{Location: &location},
	}, nil
}

// DeleteSnippet implements DELETE /api/snippet/{id}.
func (s *Server) DeleteSnippet(ctx context.Context, request DeleteSnippetRequestObject) (DeleteSnippetResponseObject, error) {
	err := s.snippets.Delete(ctx, request.Id)
	switch {
	case errors.Is(err, snippet.ErrNotFound):
		return DeleteSnippet404JSONResponse{NotFoundJSONResponse: NotFoundJSONResponse(errorBody(http.StatusNotFound))}, nil
	case err != nil:
		return nil, err
	}

	s.log.Info("snippet deleted", "id", request.Id)
	return DeleteSnippet204Response{}, nil
}
