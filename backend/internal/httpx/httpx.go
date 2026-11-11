// Package httpx holds the small HTTP helpers shared by the API handlers: JSON
// error bodies, logged 500s and pagination parameters with one semantics.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

// GenericInternalMsg is the body of a 500 that has no more specific message.
const GenericInternalMsg = "Error interno del servidor."

// JSONError writes {"error": msg} with the given status.
func JSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// CodeInternal is the stable code of every coded 500 (docs/api/README.md §3).
const CodeInternal = "error_interno"

// codedError is the error body of the SERUMS IA endpoints: a stable code, a
// Spanish fallback message and an optional detail without personal data.
type codedError struct {
	Error   string         `json:"error"`
	Message string         `json:"message"`
	Detalle map[string]any `json:"detalle,omitempty"`
}

// JSONErrorCode writes {"error": code, "message": msg, "detalle": detalle}
// with the given status; detalle is omitted when nil or empty.
func JSONErrorCode(w http.ResponseWriter, status int, code, msg string, detalle map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(codedError{Error: code, Message: msg, Detalle: detalle})
}

// InternalErrorCode is InternalError for endpoints that use the coded format:
// it logs the cause under op (never PII) and answers error_interno.
func InternalErrorCode(w http.ResponseWriter, op string, err error) {
	log.Error().Err(err).Str("op", op).Msg("internal error")
	JSONErrorCode(w, http.StatusInternalServerError, CodeInternal, GenericInternalMsg, nil)
}

// InternalError logs the cause under op (e.g. "admin.dashboard"; never PII)
// and answers a generic 500.
func InternalError(w http.ResponseWriter, op string, err error) {
	InternalErrorMsg(w, op, err, GenericInternalMsg)
}

// InternalErrorMsg is InternalError with a caller-chosen user-facing message.
func InternalErrorMsg(w http.ResponseWriter, op string, err error, msg string) {
	log.Error().Err(err).Str("op", op).Msg("internal error")
	JSONError(w, http.StatusInternalServerError, msg)
}

// ErrBadNumber reports a malformed or non-positive pagination value. Lenient
// callers ignore it (the returned default is usable); strict ones answer 400.
var ErrBadNumber = errors.New("httpx: invalid pagination number")

// parsePositive returns def for "", max for a positive number beyond int
// range, min(v, max) for v > 0 and (def, ErrBadNumber) otherwise.
func parsePositive(s string, def, max int) (int, error) {
	if s == "" {
		return def, nil
	}
	v, err := strconv.Atoi(s)
	if errors.Is(err, strconv.ErrRange) && s[0] != '-' {
		return max, nil
	}
	if err != nil || v <= 0 {
		return def, ErrBadNumber
	}
	return min(v, max), nil
}

// ParsePage parses a 1-based ?page: "" → 1, huge → maxPage (so OFFSET never
// overflows and a huge page is an empty page, not page 1), 1..maxPage kept.
func ParsePage(s string, maxPage int) (int, error) {
	return parsePositive(s, 1, maxPage)
}

// ParsePerPage parses ?per_page / ?limit: "" → def, otherwise capped at max.
func ParsePerPage(s string, def, max int) (int, error) {
	return parsePositive(s, def, max)
}
