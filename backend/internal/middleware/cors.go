package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that sets Access-Control-* headers.
// origins is a comma-separated list of allowed origins (e.g. "http://localhost:3000,https://app.example.com").
// If empty, defaults to http://localhost:3000 (the Next.js dev server). This
// default is for development only: config.Load refuses to start with
// an empty CORS_ORIGINS unless ENV is development or test.
func CORS(origins string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = true
		}
	}
	if len(allowed) == 0 {
		allowed["http://localhost:3000"] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
