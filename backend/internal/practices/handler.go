package practices

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/rs/zerolog/log"
)

// Handler exposes the practices HTTP handlers.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func pathID(r *http.Request, key string) (int64, error) {
	val := r.PathValue(key)
	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("ID inválido")
	}
	return id, nil
}

func userIDFromCtx(r *http.Request) (int64, error) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil || claims.UserID == 0 {
		return 0, errors.New("no autenticado")
	}
	return claims.UserID, nil
}

func mapErr(w http.ResponseWriter, err error) {
	var conflict ErrConflict
	var expirado ExpiradoError
	switch {
	case errors.As(err, &expirado):
		jsonOK(w, http.StatusConflict, map[string]any{
			"error":        "tiempo_agotado",
			"message":      ErrTiempoAgotado.Error(),
			"resultado_id": expirado.ResultadoID,
		})
	case errors.As(err, &conflict):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]any{
			"error":       "practica_en_curso",
			"practica_id": conflict.PracticaID,
		})
	case errors.Is(err, ErrValidation):
		jsonError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrNivelNoPermitido), errors.Is(err, ErrInscripcionRequired):
		jsonError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrLimiteExcedido):
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrTiempoAgotado):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "tiempo_agotado", "message": ErrTiempoAgotado.Error()})
	case errors.Is(err, ErrPreguntaValidada):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "pregunta_validada", "message": ErrPreguntaValidada.Error()})
	case errors.Is(err, ErrPracticaEstado):
		jsonError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrInsufficientQuestions):
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		log.Error().Err(err).Msg("practices: internal error")
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
	}
}

// ── US1 Handlers ─────────────────────────────────────────────────────────────

// CreatePractica handles POST /practicas.
func (h *Handler) CreatePractica(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil || claims.UserID == 0 {
		jsonError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	var req CreatePracticaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Payload inválido.")
		return
	}

	resp, err := h.svc.CreatePractica(r.Context(), req, claims.UserID, claims.Nivel, claims.GradoID, claims.EspecialidadID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusCreated, resp)
}

// GetPractica handles GET /practicas/{id}.
func (h *Handler) GetPractica(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	resp, err := h.svc.GetPractica(r.Context(), id, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// ToggleRespuesta handles POST /practicas/{id}/responder.
// Body: { "pregunta_id": N, "respuesta_id": N }
func (h *Handler) ToggleRespuesta(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var body struct {
		PreguntaID  int64 `json:"pregunta_id"`
		RespuestaID int64 `json:"respuesta_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PreguntaID <= 0 || body.RespuestaID <= 0 {
		jsonError(w, http.StatusBadRequest, "pregunta_id y respuesta_id son requeridos.")
		return
	}

	ids, err := h.svc.ToggleRespuesta(r.Context(), id, body.PreguntaID, body.RespuestaID, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, ResponderResponse{RespuestaIDs: ids})
}

// FinalizarPractica handles POST /practicas/{id}/finalizar.
func (h *Handler) FinalizarPractica(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req FinalizarPracticaRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // optional body

	resp, err := h.svc.FinalizarPractica(r.Context(), id, usuarioID, req)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// ── US2 Handlers ─────────────────────────────────────────────────────────────

// ValidarPregunta handles POST /practicas/{id}/validar/{preguntaId}.
func (h *Handler) ValidarPregunta(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	preguntaID, err := pathID(r, "preguntaId")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	resp, err := h.svc.ValidarPregunta(r.Context(), id, preguntaID, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// ── US3 Handlers ─────────────────────────────────────────────────────────────

// GetResultado handles GET /practicas/{id}/resultado.
func (h *Handler) GetResultado(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	resp, err := h.svc.GetResultado(r.Context(), id, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// GetRevision handles GET /practicas/{id}/revision.
func (h *Handler) GetRevision(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	resp, err := h.svc.GetRevision(r.Context(), id, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// ── US4 Handlers ─────────────────────────────────────────────────────────────

// ListHistorial handles GET /practicas.
func (h *Handler) ListHistorial(w http.ResponseWriter, r *http.Request) {
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	estado := r.URL.Query().Get("estado")

	resp, err := h.svc.ListHistorial(r.Context(), usuarioID, page, perPage, estado)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// DeletePractica handles DELETE /practicas/{id}.
func (h *Handler) DeletePractica(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if err := h.svc.DeletePractica(r.Context(), id, usuarioID); err != nil {
		mapErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BatchDeletePracticas handles DELETE /practicas/batch.
func (h *Handler) BatchDeletePracticas(w http.ResponseWriter, r *http.Request) {
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req BatchDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Payload inválido.")
		return
	}

	if err := h.svc.BatchDeletePracticas(r.Context(), req.IDs, usuarioID); err != nil {
		mapErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
