package middleware

import (
	"mime"
	"net/http"
)

// Default request body limits applied globally in cmd/server.
const (
	DefaultJSONBodyLimit      int64 = 1 << 20  // 1 MB for JSON / form / any non-multipart body
	DefaultMultipartBodyLimit int64 = 20 << 20 // 20 MB ceiling for multipart uploads
)

// MaxBodySize caps the request body. Every request is limited to jsonLimit
// except multipart/form-data requests to one of uploadRoutes ("METHOD /path",
// exact match on r.Method and r.URL.Path), which get the larger
// multipartLimit as a global ceiling (upload handlers still apply their own,
// smaller, http.MaxBytesReader). The Content-Type alone never raises the
// limit: it is chosen by the client, so a JSON body sent as multipart to a
// JSON endpoint would otherwise be read up to multipartLimit.
//
// A request whose declared Content-Length already exceeds the limit is
// rejected with 413 before the handler runs. Bodies of unknown length are
// wrapped in http.MaxBytesReader, so reading past the limit fails with
// *http.MaxBytesError (handlers should map it to 413).
func MaxBodySize(jsonLimit, multipartLimit int64, uploadRoutes ...string) func(http.Handler) http.Handler {
	uploads := make(map[string]bool, len(uploadRoutes))
	for _, route := range uploadRoutes {
		uploads[route] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || r.Body == http.NoBody {
				next.ServeHTTP(w, r)
				return
			}
			limit := jsonLimit
			if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil && mt == "multipart/form-data" &&
				uploads[r.Method+" "+r.URL.Path] {
				limit = multipartLimit
			}
			if r.ContentLength > limit {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "El cuerpo de la solicitud es demasiado grande.")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
