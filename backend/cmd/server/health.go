package main

import (
	"context"
	"encoding/json"
	"net/http"
)

// version is the release reported by GET /health and the startup log. The
// release build injects it without the "v" of the tag (the UI prints
// "v{version}"): go build -ldflags "-X main.version=1.2.0". Any other build
// reports this semver pre-release, which sorts before every release.
var version = "0.0.0-dev"

// pinger is the slice of *sqlx.DB that the health check needs.
type pinger interface {
	PingContext(ctx context.Context) error
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// healthHandler answers GET /health: 200 with the version when the database
// answers a ping, 503 otherwise. The version is encoded with encoding/json
// because it comes from the build, not from a literal.
func healthHandler(db pinger, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, code := healthResponse{Status: "ok", Version: version}, http.StatusOK
		if err := db.PingContext(r.Context()); err != nil {
			body, code = healthResponse{Status: "unhealthy"}, http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(body)
	}
}
