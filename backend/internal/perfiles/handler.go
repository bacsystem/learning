package perfiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/httpx"
)

// Stable error codes of F1–F3 (docs/api/README.md §4).
const (
	codigoValidacion            = "validacion"
	codigoInscripcionRequerida  = "inscripcion_requerida"
	codigoEspecialidadInvalida  = "especialidad_invalida"
	codigoProfesionNoDefinida   = "profesion_no_definida"
	codigoConfirmacionRequerida = "confirmacion_requerida"
	codigoEstadoInvalido        = "estado_invalido"
	msgNoAutenticado            = "No autenticado."
)

// servicio is what the handler needs from the Service.
type servicio interface {
	Opciones(ctx context.Context, usuarioID int64) (Opciones, error)
	Perfil(ctx context.Context, usuarioID int64) (Perfil, error)
	Elegir(ctx context.Context, usuarioID int64, req ElegirRequest) (Perfil, error)
}

// Handler exposes F1–F3. The user always comes from the session.
type Handler struct {
	svc servicio
}

// NewHandler creates a Handler.
func NewHandler(svc servicio) *Handler {
	return &Handler{svc: svc}
}

// Opciones handles GET /serums-ia/profesion/opciones (F1).
func (h *Handler) Opciones(w http.ResponseWriter, r *http.Request) {
	usuarioID, ok := usuario(w, r)
	if !ok {
		return
	}
	o, err := h.svc.Opciones(r.Context(), usuarioID)
	if err != nil {
		responderError(w, "opciones", err)
		return
	}
	jsonOK(w, o)
}

// Perfil handles GET /serums-ia/profesion (F2).
func (h *Handler) Perfil(w http.ResponseWriter, r *http.Request) {
	usuarioID, ok := usuario(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Perfil(r.Context(), usuarioID)
	if err != nil {
		responderError(w, "perfil", err)
		return
	}
	jsonOK(w, p)
}

// Elegir handles PUT /serums-ia/profesion (F3).
func (h *Handler) Elegir(w http.ResponseWriter, r *http.Request) {
	usuarioID, ok := usuario(w, r)
	if !ok {
		return
	}
	var req ElegirRequest
	if !httpx.DecodeJSONStrict(w, r, &req) {
		return
	}
	p, err := h.svc.Elegir(r.Context(), usuarioID, req)
	if err != nil {
		responderError(w, "elegir", err)
		return
	}
	jsonOK(w, p)
}

// usuario returns the session user; the routes run behind RequireAuth.
func usuario(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.JSONError(w, http.StatusUnauthorized, msgNoAutenticado)
		return 0, false
	}
	return claims.UserID, true
}

// responderError maps domain errors to their codes. Logs carry only the op
// and the error (ids), never personal data (E1-04, Seguridad).
func responderError(w http.ResponseWriter, op string, err error) {
	var verr *ValidacionError
	var cerr *ConfirmacionError
	switch {
	case errors.As(err, &verr):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoValidacion, err.Error(), map[string]any{"campos": verr.Campos})
	case errors.As(err, &cerr):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoConfirmacionRequerida, err.Error(), map[string]any{"actual": cerr.Actual})
	case errors.Is(err, ErrInscripcionRequerida):
		httpx.JSONErrorCode(w, http.StatusForbidden, codigoInscripcionRequerida, err.Error(), nil)
	case errors.Is(err, ErrEspecialidadInvalida):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoEspecialidadInvalida, err.Error(), nil)
	case errors.Is(err, ErrProfesionNoDefinida):
		httpx.JSONErrorCode(w, http.StatusNotFound, codigoProfesionNoDefinida, err.Error(), nil)
	case errors.Is(err, ErrEstadoInvalido):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoEstadoInvalido, err.Error(), nil)
	default:
		httpx.InternalErrorCode(w, "perfiles."+op, err)
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}
