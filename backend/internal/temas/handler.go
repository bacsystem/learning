package temas

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/httpx"
	"github.com/jmoiron/sqlx"
)

// Stable error codes of the tema endpoints (docs/api/README.md §4).
const (
	codigoCuerpoInvalido      = "cuerpo_invalido"
	codigoParametroInvalido   = "parametro_invalido"
	codigoNoEncontrado        = "no_encontrado"
	codigoValidacion          = "validacion"
	codigoReferenciaInvalida  = "referencia_invalida"
	codigoTemaDuplicado       = "tema_duplicado"
	codigoLimiteTemas         = "limite_temas"
	codigoEstadoInvalido      = "estado_invalido"
	codigoOrdenInvalido       = "orden_invalido"
	codigoOrdenDesactualizado = "orden_desactualizado"

	msgCuerpoInvalido    = "Cuerpo de solicitud inválido."
	msgParametroInvalido = "Parámetro inválido."
	msgNoAutenticado     = "No autenticado."
)

// Pagination of T1 (docs/api/README.md §5).
const (
	perPageDefecto = 20
	perPageMax     = 100
	// maxPage bounds ?page so the OFFSET can never overflow.
	maxPage = 1_000_000
)

// servicio is what the handler needs from the Service.
type servicio interface {
	Listar(ctx context.Context, f Filtro) (ListaTemas, error)
	Obtener(ctx context.Context, id int64) (Tema, error)
	ListarActivos(ctx context.Context, asignaturaID, especialidadID int64) ([]TemaActivo, error)
	Crear(ctx context.Context, req CrearTemaRequest, aud Auditor) (Tema, error)
	Editar(ctx context.Context, id int64, req EditarTemaRequest, aud Auditor) (Tema, error)
	CambiarEstado(ctx context.Context, id int64, req CambiarEstadoRequest, aud Auditor) (Tema, error)
	Reordenar(ctx context.Context, asignaturaID int64, ids []int64, aud Auditor) ([]PosicionTema, error)
}

// Handler exposes the tema endpoints T1–T7.
type Handler struct {
	svc    servicio
	audLog *audit.Logger
}

// NewHandler creates a Handler. audLog writes the audit rows of mutations.
func NewHandler(svc servicio, audLog *audit.Logger) *Handler {
	return &Handler{svc: svc, audLog: audLog}
}

// ── Admin (T1–T6) ─────────────────────────────────────────────────────────────

// Listar handles GET /admin/temas.
func (h *Handler) Listar(w http.ResponseWriter, r *http.Request) {
	f, ok := parseFiltro(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	lista, err := h.svc.Listar(r.Context(), f)
	if err != nil {
		responderError(w, "listar", err)
		return
	}
	jsonOK(w, http.StatusOK, lista)
}

// Obtener handles GET /admin/temas/{id}.
func (h *Handler) Obtener(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	t, err := h.svc.Obtener(r.Context(), id)
	if err != nil {
		responderError(w, "obtener", err)
		return
	}
	jsonOK(w, http.StatusOK, t)
}

// Crear handles POST /admin/temas.
func (h *Handler) Crear(w http.ResponseWriter, r *http.Request) {
	aud, ok := h.auditor(w, r)
	if !ok {
		return
	}
	var req CrearTemaRequest
	if !decodificar(w, r, &req) {
		return
	}
	t, err := h.svc.Crear(r.Context(), req, aud)
	if err != nil {
		responderError(w, "crear", err)
		return
	}
	jsonOK(w, http.StatusCreated, t)
}

// Editar handles PUT /admin/temas/{id}.
func (h *Handler) Editar(w http.ResponseWriter, r *http.Request) {
	aud, ok := h.auditor(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	var req EditarTemaRequest
	if !decodificar(w, r, &req) {
		return
	}
	t, err := h.svc.Editar(r.Context(), id, req, aud)
	if err != nil {
		responderError(w, "editar", err)
		return
	}
	jsonOK(w, http.StatusOK, t)
}

// CambiarEstado handles PATCH /admin/temas/{id}/estado.
func (h *Handler) CambiarEstado(w http.ResponseWriter, r *http.Request) {
	aud, ok := h.auditor(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	var req CambiarEstadoRequest
	if !decodificar(w, r, &req) {
		return
	}
	t, err := h.svc.CambiarEstado(r.Context(), id, req, aud)
	if err != nil {
		responderError(w, "cambiar_estado", err)
		return
	}
	jsonOK(w, http.StatusOK, t)
}

// Reordenar handles PUT /admin/asignaturas/{id}/temas/orden.
func (h *Handler) Reordenar(w http.ResponseWriter, r *http.Request) {
	aud, ok := h.auditor(w, r)
	if !ok {
		return
	}
	asignaturaID, ok := pathID(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	var req ReordenarRequest
	if !decodificar(w, r, &req) {
		return
	}
	if req.TemaIDs == nil {
		httpx.JSONErrorCode(w, http.StatusBadRequest, codigoCuerpoInvalido, msgCuerpoInvalido, nil)
		return
	}
	posiciones, err := h.svc.Reordenar(r.Context(), asignaturaID, *req.TemaIDs, aud)
	if err != nil {
		responderError(w, "reordenar", err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"data": posiciones})
}

// ── Autenticado (T7) ──────────────────────────────────────────────────────────

// ListarActivos handles GET /asignaturas/{id}/temas.
func (h *Handler) ListarActivos(w http.ResponseWriter, r *http.Request) {
	asignaturaID, ok := pathID(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	especialidadID, ok := queryIDOpcional(r, "especialidad_id")
	if !ok {
		parametroInvalido(w)
		return
	}
	temas, err := h.svc.ListarActivos(r.Context(), asignaturaID, especialidadID)
	if err != nil {
		responderError(w, "listar_activos", err)
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"data": temas})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// auditor binds the authenticated admin, IP and user agent to an Auditor.
// The routes run behind RequireAuth, so missing claims only happen if the
// handler is mounted without it: answer 401 instead of an anonymous audit row.
func (h *Handler) auditor(w http.ResponseWriter, r *http.Request) (Auditor, bool) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.JSONError(w, http.StatusUnauthorized, msgNoAutenticado)
		return nil, false
	}
	actor, ip, ua := claims.UserID, audit.ClientIP(r), r.UserAgent()
	return func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error {
		return h.audLog.LogTx(ctx, tx, &actor, ev.Accion, ev.Entidad, &ev.EntidadID, ev.Detalle, ip, ua)
	}, true
}

// responderError translates a domain error into its status and code; anything
// else is logged under op and answered as error_interno.
func responderError(w http.ResponseWriter, op string, err error) {
	var verr *ValidacionError
	switch {
	case errors.As(err, &verr):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoValidacion, ErrValidacion.Error(),
			map[string]any{"campos": verr.Campos})
	case errors.Is(err, ErrNoEncontrado), errors.Is(err, ErrAsignaturaNoEncontrada):
		httpx.JSONErrorCode(w, http.StatusNotFound, codigoNoEncontrado, err.Error(), nil)
	case errors.Is(err, ErrReferenciaInvalida):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoReferenciaInvalida, err.Error(), nil)
	case errors.Is(err, ErrTemaDuplicado):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoTemaDuplicado, err.Error(),
			map[string]any{"campo": campoNombre})
	case errors.Is(err, ErrLimiteTemas):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoLimiteTemas, err.Error(),
			map[string]any{"maximo": MaxTemasActivos})
	case errors.Is(err, ErrEstadoInvalido):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoEstadoInvalido, err.Error(), nil)
	case errors.Is(err, ErrOrdenInvalido):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoOrdenInvalido, err.Error(), nil)
	case errors.Is(err, ErrOrdenDesactualizado):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoOrdenDesactualizado, err.Error(), nil)
	default:
		httpx.InternalErrorCode(w, "temas."+op, err)
	}
}

// decodificar reads a JSON body, rejecting unknown fields (400 cuerpo_invalido).
func decodificar(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		httpx.JSONErrorCode(w, http.StatusBadRequest, codigoCuerpoInvalido, msgCuerpoInvalido, nil)
		return false
	}
	return true
}

// parseFiltro reads the T1 query in strict mode: asignatura_id is required and
// every malformed value is rejected.
func parseFiltro(r *http.Request) (Filtro, bool) {
	q := r.URL.Query()
	asignaturaID, err := strconv.ParseInt(q.Get("asignatura_id"), 10, 64)
	if err != nil || asignaturaID <= 0 {
		return Filtro{}, false
	}
	especialidadID, ok := queryIDOpcional(r, "especialidad_id")
	if !ok {
		return Filtro{}, false
	}
	estado := q.Get("estado")
	if estado != "" && estado != EstadoActivo && estado != EstadoArchivado {
		return Filtro{}, false
	}
	page, err := httpx.ParsePage(q.Get("page"), maxPage)
	if err != nil {
		return Filtro{}, false
	}
	perPage, err := httpx.ParsePerPage(q.Get("per_page"), perPageDefecto, perPageMax)
	if err != nil {
		return Filtro{}, false
	}
	return Filtro{AsignaturaID: asignaturaID, EspecialidadID: especialidadID, Estado: estado,
		Page: page, PerPage: perPage}, true
}

// queryIDOpcional reads an optional id > 0 from the query; "" is 0 (no filter).
func queryIDOpcional(r *http.Request, nombre string) (int64, bool) {
	s := r.URL.Query().Get(nombre)
	if s == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil && id > 0
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func parametroInvalido(w http.ResponseWriter) {
	httpx.JSONErrorCode(w, http.StatusBadRequest, codigoParametroInvalido, msgParametroInvalido, nil)
}

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
