package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// auditJSON encodes an audit_log.detalle object with encoding/json, so any
// string (control characters, invalid UTF-8 → U+FFFD) yields valid JSON for
// the jsonb column; NUL is dropped from string values because jsonb rejects
// \u0000. Never build detalle with fmt (%q emits Go escapes that are not
// JSON). It returns nil (SQL NULL) only if v cannot be encoded at all.
func auditJSON(v map[string]any) []byte {
	clean := make(map[string]any, len(v))
	for k, val := range v {
		if str, ok := val.(string); ok {
			val = strings.ReplaceAll(str, "\x00", "")
		}
		clean[k] = val
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return nil
	}
	return b
}
