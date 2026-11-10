package simulations

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/rs/zerolog/log"
)

// Handler exposes the simulations HTTP handlers.
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
			"error":        "simulacro_en_curso",
			"simulacro_id": conflict.SimulacroID,
		})
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		jsonError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrNivelNoPermitido):
		jsonError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrInscripcionRequired):
		jsonError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrSimulacroEstado):
		jsonError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNoConfig):
		jsonError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrInsufficientQuestions), errors.Is(err, ErrValidation):
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrTiempoAgotado):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "tiempo_agotado", "message": ErrTiempoAgotado.Error()})
	default:
		log.Error().Err(err).Msg("simulations: internal error")
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// CreateSimulacro handles POST /simulacros.
func (h *Handler) CreateSimulacro(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil || claims.UserID == 0 {
		jsonError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	var req CreateSimulacroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Payload inválido.")
		return
	}

	resp, err := h.svc.CreateSimulacro(r.Context(), req, claims.UserID, claims.Nivel, claims.GradoID, claims.EspecialidadID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusCreated, resp)
}

// GetSimulacro handles GET /simulacros/{id}.
func (h *Handler) GetSimulacro(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.svc.GetSimulacro(r.Context(), id, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// ResponderPregunta handles POST /simulacros/{id}/responder.
func (h *Handler) ResponderPregunta(w http.ResponseWriter, r *http.Request) {
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

	var req ResponderSimulacroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PreguntaID <= 0 || req.RespuestaID <= 0 {
		jsonError(w, http.StatusBadRequest, "pregunta_id y respuesta_id son requeridos.")
		return
	}

	ids, err := h.svc.ResponderPregunta(r.Context(), id, req, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"ok": true, "respuesta_ids": ids})
}

// SetRespuestas handles PUT /simulacros/{id}/preguntas/{preguntaId}/respuestas.
// Body: {"respuesta_ids": [..]} — replaces the whole selection (idempotent); an
// empty list clears it. 'unica' accepts at most one id.
func (h *Handler) SetRespuestas(w http.ResponseWriter, r *http.Request) {
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
	var req SetRespuestasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RespuestaIDs == nil {
		jsonError(w, http.StatusBadRequest, "respuesta_ids es requerido.")
		return
	}
	ids, err := h.svc.SetRespuestas(r.Context(), id, preguntaID, req.RespuestaIDs, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, SeleccionResponse{RespuestaIDs: ids})
}

// FinalizarSimulacro handles POST /simulacros/{id}/finalizar.
func (h *Handler) FinalizarSimulacro(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.svc.FinalizarSimulacro(r.Context(), id, usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// GetResultado handles GET /simulacros/{id}/resultado.
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

// GetRevision handles GET /simulacros/{id}/revision.
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

// GetHistorial handles GET /simulacros/historial.
func (h *Handler) GetHistorial(w http.ResponseWriter, r *http.Request) {
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	var filtros HistorialFiltros
	if pid := r.URL.Query().Get("proceso_id"); pid != "" {
		if n, err := strconv.ParseInt(pid, 10, 64); err == nil && n > 0 {
			filtros.ProcesoID = &n
		}
	}
	if aprobadoStr := r.URL.Query().Get("aprobado"); aprobadoStr != "" {
		b := aprobadoStr == "true"
		filtros.Aprobado = &b
	}

	resp, err := h.svc.GetHistorial(r.Context(), usuarioID, page, perPage, filtros)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// GetEnCurso handles GET /simulacros/en-curso.
func (h *Handler) GetEnCurso(w http.ResponseWriter, r *http.Request) {
	usuarioID, err := userIDFromCtx(r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, err.Error())
		return
	}

	id, err := h.svc.GetEnCurso(r.Context(), usuarioID)
	if err != nil {
		mapErr(w, err)
		return
	}
	if id == nil {
		jsonOK(w, http.StatusOK, map[string]any{"en_curso": false, "simulacro_id": nil})
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"en_curso": true, "simulacro_id": *id})
}

// ── Configuracion Handlers ────────────────────────────────────────────────────

// GetConfiguracion handles GET /simulacros/configuracion?proceso_id=N.
func (h *Handler) GetConfiguracion(w http.ResponseWriter, r *http.Request) {
	pidStr := r.URL.Query().Get("proceso_id")
	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil || pid <= 0 {
		jsonError(w, http.StatusBadRequest, "proceso_id requerido")
		return
	}
	var gradoID, especialidadID int64
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		gradoID = claims.GradoID
		especialidadID = claims.EspecialidadID
	}
	cfg, err := h.svc.GetConfiguracionByProceso(r.Context(), pid, gradoID, especialidadID)
	if err != nil {
		mapErr(w, err)
		return
	}
	if cfg == nil {
		jsonOK(w, http.StatusOK, map[string]any{"configuracion": nil})
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"configuracion": cfg})
}

// CreateConfiguracion handles POST /simulacros/configuracion.
func (h *Handler) CreateConfiguracion(w http.ResponseWriter, r *http.Request) {
	var req CreateConfiguracionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if req.ProcesoID <= 0 || req.TiempoTotalSeg < 60 || req.TotalPreguntas < 1 {
		jsonError(w, http.StatusBadRequest, "proceso_id, tiempo_total_seg (min 60) y total_preguntas (min 1) requeridos")
		return
	}
	cfg, err := h.svc.CreateConfiguracion(r.Context(), req)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusCreated, cfg)
}

// ListConfiguraciones handles GET /simulacros/configuraciones.
func (h *Handler) ListConfiguraciones(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListConfiguraciones(r.Context())
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// UpdateConfiguracion handles PATCH /simulacros/configuracion/{id}.
func (h *Handler) UpdateConfiguracion(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "id inválido")
		return
	}
	var req UpdateConfiguracionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	cfg, err := h.svc.UpdateConfiguracion(r.Context(), id, req)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, cfg)
}
