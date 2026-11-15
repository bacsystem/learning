package inscripciones

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/cbaciliod/learning/internal/examen"
	"github.com/cbaciliod/learning/internal/httpx"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
	"github.com/jmoiron/sqlx"
)

// Error codes of POST /inscripciones that no other package shares.
const (
	codigoNoEncontrado        = "no_encontrado"
	codigoYaInscrito          = "ya_inscrito"
	codigoSinMontoConfigurado = "sin_monto_configurado"
	codigoParametroInvalido   = "parametro_invalido"
	codigoTransicionInvalida  = "transicion_invalida"
	codigoValidacion          = "validacion"
	codigoMotivoRequerido     = "motivo_requerido"
)

type Handler struct {
	svc      *Service
	auditLog *audit.Logger
}

func NewHandler(svc *Service, auditLog *audit.Logger) *Handler {
	return &Handler{svc: svc, auditLog: auditLog}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// internalError logs the underlying cause (no PII) and answers 500.
func internalError(w http.ResponseWriter, op string, err error, msg string) {
	httpx.InternalErrorMsg(w, "inscripciones."+op, err, msg)
}

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func claimsFrom(r *http.Request) *jwtpkg.Claims {
	return auth.ClaimsFromContext(r.Context())
}

func pathID(r *http.Request, segment string) (int64, bool) {
	val := r.PathValue(segment)
	id, err := strconv.ParseInt(val, 10, 64)
	return id, err == nil && id > 0
}

// maxPage bounds ?page so the OFFSET can never overflow.
const maxPage = 1_000_000

// parsePage reads ?page and ?per_page (defaults 1 and maxListLimit) with the
// shared httpx semantics: malformed → default, huge → capped.
func parsePage(r *http.Request) (page, perPage int) {
	q := r.URL.Query()
	page, _ = httpx.ParsePage(q.Get("page"), maxPage)
	perPage, _ = httpx.ParsePerPage(q.Get("per_page"), maxListLimit, maxListLimit)
	return page, perPage
}

// ── Público ───────────────────────────────────────────────────────────────────

// GET /stats/publico — no auth required
func (h *Handler) GetStatsPublicos(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStatsPublicos(r.Context())
	if err != nil {
		internalError(w, "stats_publicos", err, "Error al obtener estadísticas.")
		return
	}
	jsonOK(w, http.StatusOK, stats)
}

// GET /procesos/publico — no auth required
func (h *Handler) ListProcesosPublico(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListTodosProcesosActivos(r.Context())
	if err != nil {
		internalError(w, "procesos_publico", err, "Error al obtener procesos.")
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// ── Usuario: ver procesos ─────────────────────────────────────────────────────

// GET /procesos
func (h *Handler) ListProcesos(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	list, err := h.svc.ListProcesosActivos(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, "list_procesos", err, "Error al obtener procesos.")
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// ── Usuario: inscripciones ────────────────────────────────────────────────────

// POST /inscripciones  body: {"proceso_id": N}
func (h *Handler) Inscribir(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	var req struct {
		ProcesoID int64 `json:"proceso_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProcesoID <= 0 {
		httpx.JSONErrorCode(w, http.StatusBadRequest, httpx.CodeInvalidBody, "proceso_id es requerido.", nil)
		return
	}
	onReapply := func(ctx context.Context, tx sqlx.ExecerContext, id int64) error {
		return h.auditLog.LogTx(ctx, tx, &claims.UserID, "reinscribir_inscripcion", "inscripcion", &id,
			map[string]any{"proceso_id": req.ProcesoID, "estado_anterior": "rechazado"},
			audit.ClientIP(r), r.UserAgent())
	}
	ins, err := h.svc.Inscribir(r.Context(), claims.UserID, req.ProcesoID, onReapply)
	if err != nil {
		inscribirError(w, err)
		return
	}
	jsonOK(w, http.StatusCreated, ins)
}

// inscribirError answers the errors of POST /inscripciones with the coded
// format of the API contract (plataforma-multi-institucion §2.3).
func inscribirError(w http.ResponseWriter, err error) {
	var (
		status int
		code   string
	)
	switch {
	case errors.Is(err, ErrProcesoNotFound):
		status, code = http.StatusNotFound, codigoNoEncontrado
	case errors.Is(err, ErrInstitucionNoPermitida):
		status, code = http.StatusForbidden, examen.CodigoInstitucionNoPermitida
	case errors.Is(err, ErrNivelNoPermitido):
		status, code = http.StatusForbidden, examen.CodigoNivelNoPermitido
	case errors.Is(err, ErrYaInscrito):
		status, code = http.StatusConflict, codigoYaInscrito
	case errors.Is(err, ErrSinConfigPago):
		status, code = http.StatusConflict, codigoSinMontoConfigurado
	default:
		httpx.InternalErrorCode(w, "inscripciones.inscribir", err)
		return
	}
	httpx.JSONErrorCode(w, status, code, err.Error(), nil)
}

// GET /inscripciones/me
func (h *Handler) MisInscripciones(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	list, err := h.svc.MisInscripciones(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, "mis_inscripciones", err, "Error al obtener inscripciones.")
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// ── Admin: inscripciones ──────────────────────────────────────────────────────

// GET /admin/inscripciones?estado=pendiente&page=1&per_page=500
// Returns a JSON array (one page, at most maxListLimit rows).
func (h *Handler) ListInscripciones(w http.ResponseWriter, r *http.Request) {
	estado := r.URL.Query().Get("estado")
	page, perPage := parsePage(r)
	list, err := h.svc.ListInscripciones(r.Context(), estado, page, perPage)
	if err != nil {
		internalError(w, "list_inscripciones", err, "Error al obtener inscripciones.")
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// PATCH /admin/inscripciones/{id}/estado  body: {"estado":"aprobado","observacion":"..."}
// Only pendiente inscriptions can be decided: 404 unknown id, 409 already decided.
// A rejection needs an observación (the motivo, 1 to 500 characters). Errors use
// the coded format of the API contract (plataforma-avisos-idioma §2.1).
func (h *Handler) UpdateInscripcionEstado(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		httpx.JSONErrorCode(w, http.StatusBadRequest, codigoParametroInvalido, "ID inválido.", nil)
		return
	}
	var req struct {
		Estado      string `json:"estado"`
		Observacion string `json:"observacion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Estado) == "" {
		httpx.JSONErrorCode(w, http.StatusBadRequest, httpx.CodeInvalidBody, "estado es requerido.", nil)
		return
	}
	auditFn := func(ctx context.Context, tx sqlx.ExecerContext) error {
		return h.auditLog.LogTx(ctx, tx, &claims.UserID, "cambiar_estado_inscripcion", "inscripcion", &id,
			map[string]any{"estado": req.Estado, "observacion": req.Observacion},
			audit.ClientIP(r), r.UserAgent())
	}
	if err := h.svc.UpdateEstado(r.Context(), id, req.Estado, req.Observacion, claims.UserID, auditFn); err != nil {
		updateEstadoError(w, err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Inscripción actualizada."})
}

// updateEstadoError answers the errors of the decision in the coded format.
func updateEstadoError(w http.ResponseWriter, err error) {
	var transicion *TransicionError
	switch {
	case errors.Is(err, ErrInscripcionNotFound):
		httpx.JSONErrorCode(w, http.StatusNotFound, codigoNoEncontrado, err.Error(), nil)
	case errors.As(err, &transicion):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoTransicionInvalida, err.Error(),
			map[string]any{"estado_actual": transicion.Actual})
	case errors.Is(err, ErrEstadoInvalido):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoValidacion, err.Error(), map[string]any{"campo": "estado"})
	case errors.Is(err, ErrObservacionLarga):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoValidacion, err.Error(), map[string]any{"campo": "observacion"})
	case errors.Is(err, ErrMotivoRequerido):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoMotivoRequerido, err.Error(), nil)
	default:
		httpx.InternalErrorCode(w, "inscripciones.update_estado", err)
	}
}
