package fuentes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/httpx"
	"github.com/jmoiron/sqlx"
)

// MaxCuerpoSubida is the reader limit of S1/S2: the 2 MiB file plus room for
// the form fields (contract S1 step 1). ParseMultipartForm keeps everything
// below it in memory, so nothing is written to disk (E2-07, Seguridad).
const MaxCuerpoSubida = MaxArchivoBytes + 64*1024

// Stable codes of the upload endpoints (docs/api/README.md §4).
const (
	codigoCuerpoInvalido  = "cuerpo_invalido"
	codigoArchivoInvalido = "archivo_invalido"
	codigoValidacion      = "validacion"
	codigoReferencia      = "referencia_invalida"
	codigoDuplicada       = "fuente_duplicada"
	codigoLimiteFuentes   = "limite_fuentes"
	codigoNoEncontrado    = "no_encontrado"
	codigoEstadoInvalido  = "estado_invalido"
	codigoLimiteVersiones = "limite_versiones"
	codigoParametro       = "parametro_invalido"
	codigoConsulta        = "consulta_invalida"
	msgCuerpoInvalido     = "El formulario multipart con el archivo es obligatorio."
	msgParametroInvalido  = "Parámetro inválido."
	msgNoEncontrado       = "Fuente no encontrada."
	msgNoAutenticado      = "No autenticado."
	campoArchivo          = "file"
	// maxPagina bounds ?page so the OFFSET can never overflow (as in temas).
	maxPagina = 1_000_000
)

// estadosFuente are the values of the S3 estado filter.
var estadosFuente = map[string]bool{EstadoProcesando: true, EstadoActiva: true, EstadoFallida: true, EstadoRetirada: true}

// servicio is what the handler needs from the Service.
type servicio interface {
	Crear(ctx context.Context, archivo ArchivoSubido, formulario FormularioFuente, actor *int64, aud Auditor) (FuenteDetalle, error)
	NuevaVersion(ctx context.Context, fuenteID string, archivo ArchivoSubido, actor *int64, aud Auditor) (FuenteDetalle, error)
	Listar(ctx context.Context, f FiltroListado) (ListaFuentes, error)
	Obtener(ctx context.Context, id int64) (FuenteDetalle, error)
	CambiarEstado(ctx context.Context, id int64, req CambioEstadoRequest, actor int64, aud Auditor) (FuenteDetalle, error)
	Oficial(ctx context.Context, id int64, req OficialRequest, actor int64, aud Auditor) (FuenteDetalle, error)
	Editar(ctx context.Context, id int64, req EditarRequest, aud Auditor) (FuenteDetalle, error)
	BuscarPasajes(ctx context.Context, q string) (ListaPasajes, error)
}

// Handler exposes S1–S7.
type Handler struct {
	svc    servicio
	audLog *audit.Logger
}

// NewHandler creates a Handler; audLog writes the audit rows.
func NewHandler(svc servicio, audLog *audit.Logger) *Handler {
	return &Handler{svc: svc, audLog: audLog}
}

// Crear handles POST /admin/fuentes (S1): 202 with the FuenteDetalle.
func (h *Handler) Crear(w http.ResponseWriter, r *http.Request) {
	actor, aud, ok := h.auditor(w, r)
	if !ok {
		return
	}
	archivo, ok := leerArchivo(w, r)
	if !ok {
		return
	}
	formulario := FormularioFuente{Titulo: r.FormValue("titulo"), Tipo: r.FormValue("tipo"), Anio: r.FormValue("anio"),
		ProcesoID: r.FormValue("proceso_id"), EspecialidadIDs: r.MultipartForm.Value["especialidad_ids"]}
	d, err := h.svc.Crear(r.Context(), archivo, formulario, &actor, aud)
	if err != nil {
		responderError(w, "crear", err)
		return
	}
	jsonStatus(w, http.StatusAccepted, d)
}

// NuevaVersion handles POST /admin/fuentes/versiones (S2): 202 with the
// FuenteDetalle. The route has no {id} so the upload body limit applies.
func (h *Handler) NuevaVersion(w http.ResponseWriter, r *http.Request) {
	actor, aud, ok := h.auditor(w, r)
	if !ok {
		return
	}
	archivo, ok := leerArchivo(w, r)
	if !ok {
		return
	}
	d, err := h.svc.NuevaVersion(r.Context(), r.FormValue("fuente_id"), archivo, &actor, aud)
	if err != nil {
		responderError(w, "nueva_version", err)
		return
	}
	jsonStatus(w, http.StatusAccepted, d)
}

// Listar handles GET /admin/fuentes (S3). A malformed filter is 400
// parametro_invalido; a title search over 100 characters, 422.
func (h *Handler) Listar(w http.ResponseWriter, r *http.Request) {
	f, ok := parsearListado(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	lista, err := h.svc.Listar(r.Context(), f)
	if err != nil {
		responderError(w, "listar", err)
		return
	}
	jsonStatus(w, http.StatusOK, lista)
}

// BuscarPasajes handles GET /admin/pasajes?q= (E3-07 §2). q is never logged.
func (h *Handler) BuscarPasajes(w http.ResponseWriter, r *http.Request) {
	lista, err := h.svc.BuscarPasajes(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		responderError(w, "buscar_pasajes", err)
		return
	}
	jsonStatus(w, http.StatusOK, lista)
}

// Obtener handles GET /admin/fuentes/{id} (S4).
func (h *Handler) Obtener(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.Obtener(r.Context(), id)
	if err != nil {
		responderError(w, "obtener", err)
		return
	}
	jsonStatus(w, http.StatusOK, d)
}

// CambiarEstado handles PATCH /admin/fuentes/{id}/estado (S5).
func (h *Handler) CambiarEstado(w http.ResponseWriter, r *http.Request) {
	var req CambioEstadoRequest
	id, actor, aud, ok := h.cambio(w, r, &req)
	if !ok {
		return
	}
	d, err := h.svc.CambiarEstado(r.Context(), id, req, actor, aud)
	if err != nil {
		responderError(w, "cambiar_estado", err)
		return
	}
	jsonStatus(w, http.StatusOK, d)
}

// Oficial handles PATCH /admin/fuentes/{id}/oficial (S6).
func (h *Handler) Oficial(w http.ResponseWriter, r *http.Request) {
	var req OficialRequest
	id, actor, aud, ok := h.cambio(w, r, &req)
	if !ok {
		return
	}
	d, err := h.svc.Oficial(r.Context(), id, req, actor, aud)
	if err != nil {
		responderError(w, "oficial", err)
		return
	}
	jsonStatus(w, http.StatusOK, d)
}

// Editar handles PUT /admin/fuentes/{id} (S7).
func (h *Handler) Editar(w http.ResponseWriter, r *http.Request) {
	var req EditarRequest
	id, _, aud, ok := h.cambio(w, r, &req)
	if !ok {
		return
	}
	d, err := h.svc.Editar(r.Context(), id, req, aud)
	if err != nil {
		responderError(w, "editar", err)
		return
	}
	jsonStatus(w, http.StatusOK, d)
}

// cambio reads what every JSON change of S5–S7 needs: the admin and its
// Auditor, the path id (400 parametro_invalido) and the body, strict (400
// cuerpo_invalido for unknown fields, 413 when oversized).
func (h *Handler) cambio(w http.ResponseWriter, r *http.Request, req any) (int64, int64, Auditor, bool) {
	actor, aud, ok := h.auditor(w, r)
	if !ok {
		return 0, 0, nil, false
	}
	id, ok := pathID(w, r)
	if !ok || !httpx.DecodeJSONStrict(w, r, req) {
		return 0, 0, nil, false
	}
	return id, actor, aud, true
}

// pathID reads {id} > 0, answering 400 parametro_invalido otherwise.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		parametroInvalido(w)
		return 0, false
	}
	return id, true
}

// parsearListado reads the S3 query strictly: an unknown estado or tipo, a
// non-positive id, an oficial other than true/false, a bad page or a q that
// PostgreSQL cannot take as text (invalid UTF-8 or a NUL byte) is false.
func parsearListado(r *http.Request) (FiltroListado, bool) {
	q := r.URL.Query()
	f := FiltroListado{Estado: q.Get("estado"), Tipo: q.Get("tipo"), Q: q.Get("q")}
	if !utf8.ValidString(f.Q) || strings.ContainsRune(f.Q, 0) {
		return FiltroListado{}, false
	}
	if (f.Estado != "" && !estadosFuente[f.Estado]) || (f.Tipo != "" && !tiposFuente[f.Tipo]) {
		return FiltroListado{}, false
	}
	if s := q.Get("especialidad_id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			return FiltroListado{}, false
		}
		f.EspecialidadID = id
	}
	if s := q.Get("oficial"); s != "" {
		if s != "true" && s != "false" {
			return FiltroListado{}, false
		}
		oficial := s == "true"
		f.Oficial = &oficial
	}
	var err error
	if f.Page, err = httpx.ParsePage(q.Get("page"), maxPagina); err != nil {
		return FiltroListado{}, false
	}
	if f.PerPage, err = httpx.ParsePerPage(q.Get("per_page"), perPageDefecto, perPageMax); err != nil {
		return FiltroListado{}, false
	}
	return f, true
}

func parametroInvalido(w http.ResponseWriter) {
	httpx.JSONErrorCode(w, http.StatusBadRequest, codigoParametro, msgParametroInvalido, nil)
}

// leerArchivo applies steps 1–2 of S1: the reader limit and the file part,
// read in memory. A body past the limit or a file over 2 MiB is 413.
func leerArchivo(w http.ResponseWriter, r *http.Request) (ArchivoSubido, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxCuerpoSubida)
	if err := r.ParseMultipartForm(MaxCuerpoSubida); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			archivoDemasiadoGrande(w)
		} else {
			httpx.JSONErrorCode(w, http.StatusBadRequest, codigoCuerpoInvalido, msgCuerpoInvalido, nil)
		}
		return ArchivoSubido{}, false
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	file, header, err := r.FormFile(campoArchivo)
	if err != nil {
		httpx.JSONErrorCode(w, http.StatusBadRequest, codigoCuerpoInvalido, msgCuerpoInvalido, nil)
		return ArchivoSubido{}, false
	}
	defer func(f multipart.File) { _ = f.Close() }(file)
	if header.Size > MaxArchivoBytes {
		archivoDemasiadoGrande(w)
		return ArchivoSubido{}, false
	}
	contenido, err := io.ReadAll(io.LimitReader(file, MaxArchivoBytes+1))
	if err != nil || len(contenido) > MaxArchivoBytes {
		archivoDemasiadoGrande(w)
		return ArchivoSubido{}, false
	}
	return ArchivoSubido{Nombre: header.Filename, ContentType: header.Header.Get("Content-Type"), Contenido: contenido}, true
}

func archivoDemasiadoGrande(w http.ResponseWriter) {
	httpx.JSONErrorCode(w, http.StatusRequestEntityTooLarge, httpx.CodeFileTooLarge, ErrArchivoDemasiadoGrande.Error(), nil)
}

// auditor binds the authenticated admin, IP and user agent to an Auditor.
func (h *Handler) auditor(w http.ResponseWriter, r *http.Request) (int64, Auditor, bool) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.JSONError(w, http.StatusUnauthorized, msgNoAutenticado)
		return 0, nil, false
	}
	actor, ip, ua := claims.UserID, audit.ClientIP(r), r.UserAgent()
	return actor, func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error {
		return h.audLog.LogTx(ctx, tx, &actor, ev.Accion, ev.Entidad, &ev.EntidadID, ev.Detalle, ip, ua)
	}, true
}

// responderError maps the upload errors to their codes; anything else is
// logged under op (ids only, never content) and answered as error_interno.
func responderError(w http.ResponseWriter, op string, err error) {
	var (
		archivo *ArchivoInvalidoError
		verr    *ValidacionError
		dup     *DuplicadaError
		limite  *LimiteError
	)
	switch {
	case errors.As(err, &archivo):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoArchivoInvalido, ErrArchivoInvalido.Error(), map[string]any{"motivo": archivo.Motivo})
	case errors.As(err, &verr):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoValidacion, ErrValidacion.Error(), map[string]any{"campos": verr.Campos})
	case errors.Is(err, ErrCuerpoInvalido):
		httpx.JSONErrorCode(w, http.StatusBadRequest, codigoCuerpoInvalido, err.Error(), nil)
	case errors.Is(err, ErrReferenciaInvalida):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoReferencia, err.Error(), nil)
	case errors.As(err, &dup):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoDuplicada, "Ya existe una fuente con este mismo archivo.", map[string]any{
			"fuente_id": dup.FuenteID, "titulo": dup.Titulo, "estado": dup.Estado, "version": dup.Version})
	case errors.Is(err, ErrFuenteDuplicada):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoDuplicada, "Ya existe una fuente con este mismo archivo.", nil)
	case errors.As(err, &limite):
		detalle := map[string]any{"limite": limite.Limite}
		if limite.EspecialidadID != nil {
			detalle["especialidad_id"] = *limite.EspecialidadID
		}
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoLimiteFuentes, ErrLimiteFuentes.Error(), detalle)
	case errors.Is(err, ErrConsultaInvalida):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoConsulta, err.Error(), map[string]any{"min": MinConsulta, "max": MaxConsulta})
	case errors.Is(err, ErrNoEncontrado):
		httpx.JSONErrorCode(w, http.StatusNotFound, codigoNoEncontrado, msgNoEncontrado, nil)
	case errors.Is(err, ErrEstadoInvalido):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoEstadoInvalido, err.Error(), nil)
	case errors.Is(err, ErrLimiteVersiones):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoLimiteVersiones, err.Error(), nil)
	default:
		httpx.InternalErrorCode(w, "fuentes."+op, err)
	}
}

func jsonStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
