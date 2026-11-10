package catalogs

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/cbaciliod/learning/internal/httpx"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
)

var validate = newValidator()

// newValidator registers dec2: a float with at most 2 decimals, so a value
// such as 0.004 is rejected instead of being rounded by NUMERIC(5,2) (H87).
func newValidator() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("dec2", func(fl validator.FieldLevel) bool {
		return hasAtMost2Decimals(fl.Field().Float())
	})
	return v
}

// hasAtMost2Decimals tolerates binary representation noise (0.29*100 =
// 28.999999999999996) but not a real third decimal.
func hasAtMost2Decimals(f float64) bool {
	c := f * 100
	return math.Abs(c-math.Round(c)) < 1e-6
}

// Handler exposes the catalog HTTP handlers.
type Handler struct {
	svc      *Service
	auditLog *audit.Logger
}

// NewHandler creates a new Handler. Every successful mutation is written to
// the audit log (auditLog may be nil in tests).
func NewHandler(svc *Service, auditLog *audit.Logger) *Handler {
	return &Handler{svc: svc, auditLog: auditLog}
}

// audit records a successful catalog mutation by the authenticated admin.
func (h *Handler) audit(r *http.Request, accion, entidad string, id int64, detalle map[string]any) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		return
	}
	h.auditLog.Log(r.Context(), &claims.UserID, accion, entidad, &id, detalle, audit.ClientIP(r), r.UserAgent())
}

// auditTx is audit for mutations that run in a transaction: the repository
// writes the row in that transaction, so a failed audit rolls the change back.
func (h *Handler) auditTx(r *http.Request, accion, entidad string, id int64, detalle map[string]any) audit.TxFunc {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		return nil
	}
	return func(ctx context.Context, tx sqlx.ExecerContext) error {
		return h.auditLog.LogTx(ctx, tx, &claims.UserID, accion, entidad, &id, detalle, audit.ClientIP(r), r.UserAgent())
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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

func pathID(r *http.Request, segment string) (int64, bool) {
	val := r.PathValue(segment)
	id, err := strconv.ParseInt(val, 10, 64)
	return id, err == nil && id > 0
}

// mapErr maps sentinel errors to HTTP status codes; anything else is logged
// with the operation name and answered with a generic 500.
func mapErr(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		jsonError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrValidation):
		jsonError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrInvalidRef):
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		httpx.InternalError(w, "catalogs."+op, err)
	}
}

var errBadQuery = errors.New("Parámetros de consulta inválidos.")

// parseListQuery builds a ListQuery from URL query parameters.
// limit: "all"/empty → whole catalog (bounded by maxAll); N → 1..maxLimit.
// page: 1-based, only meaningful with a numeric limit, capped at maxPage.
// Malformed numbers are rejected instead of silently treated as 0.
func parseListQuery(r *http.Request) (ListQuery, error) {
	v := r.URL.Query()
	q := ListQuery{Estado: v.Get("estado"), Tipo: v.Get("tipo"), Nivel: v.Get("nivel")}
	if s := v.Get("institucion_id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id < 0 {
			return q, errBadQuery
		}
		q.InstitucionID = id
	}
	if s := v.Get("anio"); s != "" {
		anio, err := strconv.Atoi(s)
		if err != nil || anio < 0 {
			return q, errBadQuery
		}
		q.Anio = anio
	}
	// Same clamping as every other list (httpx); catalogs are strict and
	// answer 400 on malformed numbers instead of falling back to a default.
	if s := v.Get("limit"); s != "" && s != "all" {
		n, err := httpx.ParsePerPage(s, maxLimit, maxLimit)
		if err != nil {
			return q, errBadQuery
		}
		q.Limit = n
	}
	if s := v.Get("page"); s != "" {
		n, err := httpx.ParsePage(s, maxPage)
		if err != nil {
			return q, errBadQuery
		}
		q.Page = n
	}
	return q, nil
}

// ── Instituciones ─────────────────────────────────────────────────────────────

func (h *Handler) ListInstituciones(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := h.svc.ListInstituciones(r.Context(), q)
	if err != nil {
		mapErr(w, "list_instituciones", err)
		return
	}
	if list == nil {
		list = []InstitucionResponse{}
	}
	jsonOK(w, http.StatusOK, list)
}

func (h *Handler) GetInstitucion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	row, err := h.svc.GetInstitucion(r.Context(), id)
	if err != nil {
		mapErr(w, "get_institucion", err)
		return
	}
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) CreateInstitucion(w http.ResponseWriter, r *http.Request) {
	var req CreateInstitucionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.CreateInstitucion(r.Context(), req)
	if err != nil {
		mapErr(w, "create_institucion", err)
		return
	}
	h.audit(r, "crear_institucion", "institucion", row.ID, map[string]any{"datos": req})
	jsonOK(w, http.StatusCreated, row)
}

func (h *Handler) UpdateInstitucion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req UpdateInstitucionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.UpdateInstitucion(r.Context(), id, req)
	if err != nil {
		mapErr(w, "update_institucion", err)
		return
	}
	h.audit(r, "actualizar_institucion", "institucion", id, map[string]any{"datos": req})
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) UpdateInstitucionEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req EstadoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := h.svc.UpdateInstitucionEstado(r.Context(), id, req.Estado,
		h.auditTx(r, "cambiar_estado_institucion", "institucion", id, map[string]any{"estado": req.Estado})); err != nil {
		mapErr(w, "update_institucion_estado", err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Estado actualizado."})
}

// ── Grados ────────────────────────────────────────────────────────────────────

func (h *Handler) ListGrados(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := h.svc.ListGrados(r.Context(), q)
	if err != nil {
		mapErr(w, "list_grados", err)
		return
	}
	if list == nil {
		list = []GradoResponse{}
	}
	jsonOK(w, http.StatusOK, list)
}

func (h *Handler) GetGrado(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	row, err := h.svc.GetGrado(r.Context(), id)
	if err != nil {
		mapErr(w, "get_grado", err)
		return
	}
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) CreateGrado(w http.ResponseWriter, r *http.Request) {
	var req CreateGradoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.CreateGrado(r.Context(), req)
	if err != nil {
		mapErr(w, "create_grado", err)
		return
	}
	h.audit(r, "crear_grado", "grado", row.ID, map[string]any{"datos": req})
	jsonOK(w, http.StatusCreated, row)
}

func (h *Handler) UpdateGrado(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req CreateGradoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.UpdateGrado(r.Context(), id, req)
	if err != nil {
		mapErr(w, "update_grado", err)
		return
	}
	h.audit(r, "actualizar_grado", "grado", id, map[string]any{"datos": req})
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) UpdateGradoEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req EstadoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := h.svc.UpdateGradoEstado(r.Context(), id, req.Estado); err != nil {
		mapErr(w, "update_grado_estado", err)
		return
	}
	h.audit(r, "cambiar_estado_grado", "grado", id, map[string]any{"estado": req.Estado})
	jsonOK(w, http.StatusOK, map[string]string{"message": "Estado actualizado."})
}

// ── Especialidades ────────────────────────────────────────────────────────────

func (h *Handler) ListEspecialidades(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := h.svc.ListEspecialidades(r.Context(), q)
	if err != nil {
		mapErr(w, "list_especialidades", err)
		return
	}
	if list == nil {
		list = []EspecialidadResponse{}
	}
	jsonOK(w, http.StatusOK, list)
}

func (h *Handler) GetEspecialidad(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	row, err := h.svc.GetEspecialidad(r.Context(), id)
	if err != nil {
		mapErr(w, "get_especialidad", err)
		return
	}
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) CreateEspecialidad(w http.ResponseWriter, r *http.Request) {
	var req CreateEspecialidadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.CreateEspecialidad(r.Context(), req)
	if err != nil {
		mapErr(w, "create_especialidad", err)
		return
	}
	h.audit(r, "crear_especialidad", "especialidad", row.ID, map[string]any{"datos": req})
	jsonOK(w, http.StatusCreated, row)
}

func (h *Handler) UpdateEspecialidad(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req CreateEspecialidadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.UpdateEspecialidad(r.Context(), id, req)
	if err != nil {
		mapErr(w, "update_especialidad", err)
		return
	}
	h.audit(r, "actualizar_especialidad", "especialidad", id, map[string]any{"datos": req})
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) UpdateEspecialidadEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req EstadoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := h.svc.UpdateEspecialidadEstado(r.Context(), id, req.Estado); err != nil {
		mapErr(w, "update_especialidad_estado", err)
		return
	}
	h.audit(r, "cambiar_estado_especialidad", "especialidad", id, map[string]any{"estado": req.Estado})
	jsonOK(w, http.StatusOK, map[string]string{"message": "Estado actualizado."})
}

// ── Asignaturas ───────────────────────────────────────────────────────────────

func (h *Handler) ListAsignaturas(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := h.svc.ListAsignaturas(r.Context(), q)
	if err != nil {
		mapErr(w, "list_asignaturas", err)
		return
	}
	if list == nil {
		list = []AsignaturaResponse{}
	}
	jsonOK(w, http.StatusOK, list)
}

func (h *Handler) GetAsignatura(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	row, err := h.svc.GetAsignatura(r.Context(), id)
	if err != nil {
		mapErr(w, "get_asignatura", err)
		return
	}
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) CreateAsignatura(w http.ResponseWriter, r *http.Request) {
	var req CreateAsignaturaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.CreateAsignatura(r.Context(), req)
	if err != nil {
		mapErr(w, "create_asignatura", err)
		return
	}
	h.audit(r, "crear_asignatura", "asignatura", row.ID, map[string]any{"datos": req})
	jsonOK(w, http.StatusCreated, row)
}

func (h *Handler) UpdateAsignatura(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req CreateAsignaturaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.UpdateAsignatura(r.Context(), id, req)
	if err != nil {
		mapErr(w, "update_asignatura", err)
		return
	}
	h.audit(r, "actualizar_asignatura", "asignatura", id, map[string]any{"datos": req})
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) UpdateAsignaturaEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req EstadoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := h.svc.UpdateAsignaturaEstado(r.Context(), id, req.Estado); err != nil {
		mapErr(w, "update_asignatura_estado", err)
		return
	}
	h.audit(r, "cambiar_estado_asignatura", "asignatura", id, map[string]any{"estado": req.Estado})
	jsonOK(w, http.StatusOK, map[string]string{"message": "Estado actualizado."})
}

// ── Procesos ──────────────────────────────────────────────────────────────────

func (h *Handler) ListProcesos(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := h.svc.ListProcesos(r.Context(), q)
	if err != nil {
		mapErr(w, "list_procesos", err)
		return
	}
	if list == nil {
		list = []ProcesoResponse{}
	}
	jsonOK(w, http.StatusOK, list)
}

func (h *Handler) GetProceso(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	row, err := h.svc.GetProceso(r.Context(), id)
	if err != nil {
		mapErr(w, "get_proceso", err)
		return
	}
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) CreateProceso(w http.ResponseWriter, r *http.Request) {
	var req CreateProcesoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.CreateProceso(r.Context(), req)
	if err != nil {
		mapErr(w, "create_proceso", err)
		return
	}
	h.audit(r, "crear_proceso", "proceso", row.ID, map[string]any{"datos": req})
	jsonOK(w, http.StatusCreated, row)
}

func (h *Handler) UpdateProceso(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req CreateProcesoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, err := h.svc.UpdateProceso(r.Context(), id, req)
	if err != nil {
		mapErr(w, "update_proceso", err)
		return
	}
	h.audit(r, "actualizar_proceso", "proceso", id, map[string]any{"datos": req})
	jsonOK(w, http.StatusOK, row)
}

func (h *Handler) UpdateProcesoEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req EstadoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := h.svc.UpdateProcesoEstado(r.Context(), id, req.Estado,
		h.auditTx(r, "cambiar_estado_proceso", "proceso", id, map[string]any{"estado": req.Estado})); err != nil {
		mapErr(w, "update_proceso_estado", err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Estado actualizado."})
}

// ── Proceso-Asignaturas ───────────────────────────────────────────────────────

func (h *Handler) ListProcesoAsignaturas(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var gradoID, especialidadID int64
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		gradoID = claims.GradoID
		especialidadID = claims.EspecialidadID
	}
	list, err := h.svc.ListProcesoAsignaturas(r.Context(), id, gradoID, especialidadID)
	if err != nil {
		mapErr(w, "list_proceso_asignaturas", err)
		return
	}
	if list == nil {
		list = []ProcesoAsignaturaItem{}
	}
	jsonOK(w, http.StatusOK, list)
}

func (h *Handler) AsignarAsignatura(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req AsignarAsignaturaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	item, err := h.svc.AsignarAsignatura(r.Context(), id, req,
		h.auditTx(r, "asignar_asignatura_proceso", "proceso", id, map[string]any{"datos": req}))
	if err != nil {
		mapErr(w, "asignar_asignatura", err)
		return
	}
	jsonOK(w, http.StatusCreated, item)
}

func (h *Handler) UpdateProcesoAsignatura(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID de proceso inválido.")
		return
	}
	asignaturaIDStr := r.PathValue("asignaturaId")
	asignaturaID, err := strconv.ParseInt(asignaturaIDStr, 10, 64)
	if err != nil || asignaturaID <= 0 {
		jsonError(w, http.StatusBadRequest, "ID de asignatura inválido.")
		return
	}
	var req UpdateProcesoAsignaturaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := h.svc.UpdateProcesoAsignatura(r.Context(), id, asignaturaID, req,
		h.auditTx(r, "actualizar_asignatura_proceso", "proceso", id,
			map[string]any{"asignatura_id": asignaturaID, "datos": req})); err != nil {
		mapErr(w, "update_proceso_asignatura", err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Asignatura actualizada."})
}

func (h *Handler) DeleteProcesoAsignatura(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		jsonError(w, http.StatusBadRequest, "ID de proceso inválido.")
		return
	}
	asignaturaIDStr := r.PathValue("asignaturaId")
	asignaturaID, err := strconv.ParseInt(asignaturaIDStr, 10, 64)
	if err != nil || asignaturaID <= 0 {
		jsonError(w, http.StatusBadRequest, "ID de asignatura inválido.")
		return
	}
	if err := h.svc.DeleteProcesoAsignatura(r.Context(), id, asignaturaID,
		h.auditTx(r, "quitar_asignatura_proceso", "proceso", id, map[string]any{"asignatura_id": asignaturaID})); err != nil {
		mapErr(w, "delete_proceso_asignatura", err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Asignatura eliminada del proceso."})
}
