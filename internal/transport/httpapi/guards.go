package httpapi

import (
	"errors"
	"mime"
	"net/http"
)

// maxBodyBytes is the largest request body the application will read.
//
// It has to live here rather than in the contract: the generated strict handler
// decodes r.Body before its own StrictMiddlewareFunc chain runs, so a strict
// middleware is already too late. nginx happens to enforce the same number,
// which is why its absence was invisible until a request was sent to the port
// behind nginx.
//
// 1 MiB is what nginx was enforcing by default. #40 lowers it and reports an
// over-long `content` as a field error rather than a 413; this is the transport
// guard that makes that safe to do.
const maxBodyBytes = 1 << 20

// bodyGuards applies the two checks the generated code does not make: how large
// a body may be, and what media type it must claim. Both are request-shaping, so
// both belong in front of everything else.
func bodyGuards(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Body != http.NoBody {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}

		// Only when there is a body to describe. A POST with nothing in it is not
		// a media-type problem, and routing must get the chance to answer 405
		// before anything here does: this runs in front of the mux, so a check
		// that fires on every POST would turn "wrong method" into "wrong type".
		// -1 means an unknown length (chunked), which still carries a body.
		if carriesBody(r.Method) && r.ContentLength != 0 && !isJSON(r.Header.Get("Content-Type")) {
			writeJSON(w, http.StatusUnsupportedMediaType, errorBody(http.StatusUnsupportedMediaType))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func carriesBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	}
	return false
}

// isJSON tolerates the charset parameter real clients send. It is deliberately
// strict about the media type itself: text/plain, multipart/form-data and
// application/x-www-form-urlencoded are the three a cross-site <form> can send
// without a preflight, so accepting them is the first half of a CSRF problem
// that #55 exists to close.
func isJSON(contentType string) bool {
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}

// isBodyTooLarge reports whether the decoder failed because MaxBytesReader cut
// the body off, which is a 413 rather than the 400 a malformed body gets.
func isBodyTooLarge(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}
