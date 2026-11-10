package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/cbaciliod/learning/internal/httpx"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
)

var validate = validator.New()

type Handler struct {
	repo     *Repository
	auditLog *audit.Logger
}

func NewHandler(repo *Repository, auditLog *audit.Logger) *Handler {
	return &Handler{repo: repo, auditLog: auditLog}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// internalError logs the underlying cause (no PII) and answers a generic 500.
func internalError(w http.ResponseWriter, op string, err error) {
	httpx.InternalError(w, "admin."+op, err)
}

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetDashboardStats(r.Context())
	if err != nil {
		internalError(w, "dashboard", err)
		return
	}
	jsonOK(w, http.StatusOK, stats)
}

func (h *Handler) handleDashboardActividad(w http.ResponseWriter, r *http.Request) {
	practicas, simulacros, top, err := h.repo.GetDashboardActividad(r.Context())
	if err != nil {
		internalError(w, "dashboard_actividad", err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{
		"practicas_ultima_semana":  practicas,
		"simulacros_ultima_semana": simulacros,
		"top_asignaturas":          top,
	})
}

// ── Usuario Detalle ───────────────────────────────────────────────────────────

func (h *Handler) handleGetUsuarioDetalle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "ID de usuario inválido.")
		return
	}
	detalle, err := h.repo.GetUsuarioDetalle(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "Usuario no encontrado.")
		return
	}
	if err != nil {
		internalError(w, "usuario_detalle", err)
		return
	}
	jsonOK(w, http.StatusOK, detalle)
}

// handlePatchUsuarioRol changes a user's role. 404 unknown id, 409 last admin.
// The target's sessions are revoked; the change is audited only on success.
func (h *Handler) handlePatchUsuarioRol(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "ID de usuario inválido.")
		return
	}

	var req UpdateRolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	var auditFn audit.TxFunc
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		auditFn = func(ctx context.Context, tx sqlx.ExecerContext) error {
			return h.auditLog.LogTx(ctx, tx, &claims.UserID, "cambiar_rol", "usuario", &id,
				map[string]any{"rol": req.Rol},
				audit.ClientIP(r), r.UserAgent())
		}
	}
	if err := h.repo.UpdateUsuarioRol(r.Context(), id, req.Rol, auditFn); err != nil {
		switch {
		case errors.Is(err, errUsuarioNotFound):
			jsonError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, errLastAdmin):
			jsonError(w, http.StatusConflict, err.Error())
		default:
			internalError(w, "cambiar_rol", err)
		}
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Rol actualizado."})
}

// ── Audit Log ─────────────────────────────────────────────────────────────────

// maxAuditPage bounds ?page so (page-1)*per_page never overflows the OFFSET.
const maxAuditPage = 1_000_000

func (h *Handler) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, _ := httpx.ParsePage(q.Get("page"), maxAuditPage)
	perPage, _ := httpx.ParsePerPage(q.Get("per_page"), 20, 100)

	from := q.Get("from")
	to := q.Get("to")
	for _, d := range []string{from, to} {
		if d == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d); err != nil {
			jsonError(w, http.StatusBadRequest, "Fecha inválida. Use el formato AAAA-MM-DD.")
			return
		}
	}
	search := q.Get("q")

	resp, err := h.repo.GetAuditLog(r.Context(), page, perPage, from, to, search)
	if err != nil {
		internalError(w, "audit_log", err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// ── Ingresos ──────────────────────────────────────────────────────────────────

func (h *Handler) handleGetIngresos(w http.ResponseWriter, r *http.Request) {
	resp, err := h.repo.GetIngresosSummary(r.Context())
	if err != nil {
		internalError(w, "ingresos", err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

func (h *Handler) handleGetIngresosConfig(w http.ResponseWriter, r *http.Request) {
	resp, err := h.repo.GetIngresosConfig(r.Context())
	if err != nil {
		internalError(w, "ingresos_config", err)
		return
	}
	if resp == nil {
		resp = []ConfigPago{}
	}
	jsonOK(w, http.StatusOK, resp)
}

// montoRe accepts a non-negative decimal with at most 8 integer digits and 2
// decimals, i.e. exactly the range of DECIMAL(10,2): 0 … 99,999,999.99.
// Exponents, signs and extra decimals are rejected instead of rounded.
var montoRe = regexp.MustCompile(`^(0|[1-9][0-9]{0,7})(\.[0-9]{1,2})?$`)

// parseMonto validates the literal sent by the client and returns it as an
// exact decimal string for the DB.
func parseMonto(n *json.Number) (string, bool) {
	if n == nil || !montoRe.MatchString(n.String()) {
		return "", false
	}
	return n.String(), true
}

func (h *Handler) handlePutIngresosConfig(w http.ResponseWriter, r *http.Request) {
	anioStr := r.PathValue("anio")
	anio, err := strconv.Atoi(anioStr)
	if err != nil || anio < 2000 || anio > 2099 {
		jsonError(w, http.StatusBadRequest, "Año inválido. Debe estar entre 2000 y 2099.")
		return
	}

	var req UpdateConfigPagoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	monto, ok := parseMonto(req.Monto)
	if !ok {
		jsonError(w, http.StatusUnprocessableEntity,
			"monto es requerido: número entre 0 y 99999999.99 con hasta 2 decimales.")
		return
	}

	var auditFn func(context.Context, sqlx.ExecerContext, *ConfigPago, *string) error
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		auditFn = func(ctx context.Context, tx sqlx.ExecerContext, c *ConfigPago, anterior *string) error {
			anio64 := int64(anio)
			return h.auditLog.LogTx(ctx, tx, &claims.UserID, "actualizar_config_pago", "configuracion_pago", &anio64,
				map[string]any{"anio": anio, "monto": c.Monto.String(), "monto_anterior": anterior},
				audit.ClientIP(r), r.UserAgent())
		}
	}
	result, _, err := h.repo.UpsertConfigPago(r.Context(), anio, monto, auditFn)
	if err != nil {
		internalError(w, "upsert_config_pago", err)
		return
	}
	jsonOK(w, http.StatusOK, result)
}
