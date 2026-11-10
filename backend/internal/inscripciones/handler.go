package inscripciones

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/cbaciliod/learning/internal/httpx"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
	"github.com/jmoiron/sqlx"
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
	list, err := h.svc.ListProcesosActivos(r.Context(), claims.Nivel)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProcesoID == 0 {
		jsonError(w, http.StatusBadRequest, "proceso_id es requerido.")
		return
	}
	onReapply := func(ctx context.Context, tx sqlx.ExecerContext, id int64) error {
		return h.auditLog.LogTx(ctx, tx, &claims.UserID, "reinscribir_inscripcion", "inscripcion", &id,
			map[string]any{"proceso_id": req.ProcesoID, "estado_anterior": "rechazado"},
			audit.ClientIP(r), r.UserAgent())
	}
	ins, err := h.svc.Inscribir(r.Context(), claims.UserID, req.ProcesoID, claims.Nivel, onReapply)
	if err != nil {
		switch {
		case errors.Is(err, ErrProcesoNotFound):
			jsonError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrYaInscrito), errors.Is(err, ErrSinConfigPago):
			jsonError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrNivelNoPermitido):
			jsonError(w, http.StatusForbidden, err.Error())
		default:
			internalError(w, "inscribir", err, "Error al procesar la inscripción.")
		}
		return
	}
	jsonOK(w, http.StatusCreated, ins)
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
func (h *Handler) UpdateInscripcionEstado(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req struct {
		Estado      string `json:"estado"`
		Observacion string `json:"observacion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Estado) == "" {
		jsonError(w, http.StatusBadRequest, "estado es requerido.")
		return
	}
	auditFn := func(ctx context.Context, tx sqlx.ExecerContext) error {
		return h.auditLog.LogTx(ctx, tx, &claims.UserID, "cambiar_estado_inscripcion", "inscripcion", &id,
			map[string]any{"estado": req.Estado, "observacion": req.Observacion},
			audit.ClientIP(r), r.UserAgent())
	}
	if err := h.svc.UpdateEstado(r.Context(), id, req.Estado, req.Observacion, claims.UserID, auditFn); err != nil {
		switch {
		case errors.Is(err, ErrInscripcionNotFound):
			jsonError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrTransicionInvalida):
			jsonError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrEstadoInvalido):
			jsonError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			internalError(w, "update_estado", err, "Error al actualizar la inscripción.")
		}
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Inscripción actualizada."})
}
