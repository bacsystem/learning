package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingerFunc func(ctx context.Context) error

func (f pingerFunc) PingContext(ctx context.Context) error { return f(ctx) }

func TestHealthHandler(t *testing.T) {
	cases := []struct {
		name     string
		pingErr  error
		version  string
		wantCode int
		want     healthResponse
	}{
		{"database up", nil, "1.2.0", http.StatusOK, healthResponse{Status: "ok", Version: "1.2.0"}},
		{"default dev version", nil, "0.0.0-dev", http.StatusOK, healthResponse{Status: "ok", Version: "0.0.0-dev"}},
		{"version needing JSON escaping", nil, `1.2.0"x`, http.StatusOK, healthResponse{Status: "ok", Version: `1.2.0"x`}},
		{"database down", errors.New("down"), "1.2.0", http.StatusServiceUnavailable, healthResponse{Status: "unhealthy"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := healthHandler(pingerFunc(func(context.Context) error { return c.pingErr }), c.version)
			w := httptest.NewRecorder()
			h(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			if w.Code != c.wantCode {
				t.Errorf("code %d, want %d", w.Code, c.wantCode)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type %q", ct)
			}
			var got healthResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("body %q: %v", w.Body.String(), err)
			}
			if got != c.want {
				t.Errorf("body %+v, want %+v", got, c.want)
			}
		})
	}
}
