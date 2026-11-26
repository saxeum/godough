// Package httpapi is the transport tier. It owns HTTP and nothing else: the
// contract in openapi.gen.go parses parameters and bodies, this package maps
// domain values to wire values and domain errors to documented status codes,
// and everything else is delegated to the service.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/saxeum/godough/internal/domain/snippet"
)

// toWire converts a stored snippet to the type the contract describes. It is a
// hand-written mapping on purpose: the generated type is the wire format, and
// keeping it separate from the domain means renaming a field in the spec is a
// change to this file and the spec, not to the business rules.
func toWire(in snippet.Snippet) Snippet {
	return Snippet{
		ID:      in.ID,
		Title:   in.Title,
		Content: in.Content,
		Created: in.Created,
		Expires: in.Expires,
	}
}

// toWireAll does the same for a list, allocating for the empty case so the
// response is [] rather than null.
func toWireAll(in []snippet.Snippet) []Snippet {
	out := make([]Snippet, 0, len(in))
	for _, item := range in {
		out = append(out, toWire(item))
	}
	return out
}

// errorBody is the shape every non-2xx response shares. It is the generated
// Error type, so a change to the contract reaches this function.
//
// 4xx bodies say what the status says, which is what a client would print
// anyway. A 5xx body says nothing useful on purpose: the cause is in the log,
// where it has a request context and is not shown to the caller.
func errorBody(status int) Error {
	if status >= 500 {
		return Error{Error: "internal server error"}
	}
	return Error{Error: http.StatusText(status)}
}

// writeJSON is the only way this package writes a non-generated body. Encoder,
// not Marshal, so the bytes match what the generated response visitors emit.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
