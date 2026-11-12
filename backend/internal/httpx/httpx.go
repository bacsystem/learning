// Package httpx holds the small HTTP helpers shared by the API handlers: JSON
// error bodies, logged 500s and pagination parameters with one semantics.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
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

// Stable codes shared by the coded endpoints (docs/api/README.md §3, §4).
const (
	CodeInternal     = "error_interno"
	CodeInvalidBody  = "cuerpo_invalido"
	CodeBodyTooLarge = "cuerpo_demasiado_grande"
	// CodeFileTooLarge is the 413 of the multipart uploads (E2 S1).
	CodeFileTooLarge = "archivo_demasiado_grande"

	// MsgBodyTooLarge is the message of every 413 for a body over the limit.
	MsgBodyTooLarge = "El cuerpo de la solicitud es demasiado grande."

	msgInvalidBody = "Cuerpo de solicitud inválido."
)

// DecodeJSONStrict decodes the JSON body into v rejecting unknown fields and
// anything after the first value (only white space may follow). On failure it
// writes the coded error and returns false: 413 cuerpo_demasiado_grande when
// the body went past the http.MaxBytesReader limit while reading (no
// Content-Length, docs/api/README.md §3), 400 cuerpo_invalido otherwise.
func DecodeJSONStrict(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil {
		err = soloEspaciosDespues(dec)
	}
	if err == nil {
		return true
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		JSONErrorCode(w, http.StatusRequestEntityTooLarge, CodeBodyTooLarge, MsgBodyTooLarge, nil)
	} else {
		JSONErrorCode(w, http.StatusBadRequest, CodeInvalidBody, msgInvalidBody, nil)
	}
	return false
}

// errDatosDeMas: the body has something after its JSON value.
var errDatosDeMas = errors.New("httpx: datos después del cuerpo JSON")

// soloEspaciosDespues checks that the decoder has nothing left but white
// space: a second value, or garbage, is an error. A read error (such as the
// size limit) is returned as it is.
func soloEspaciosDespues(dec *json.Decoder) error {
	var resto json.RawMessage
	err := dec.Decode(&resto)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return errDatosDeMas
}

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
