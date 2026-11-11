package fuentes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"

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
	codigoArchivoGrande   = "archivo_demasiado_grande"
	codigoCuerpoInvalido  = "cuerpo_invalido"
	codigoArchivoInvalido = "archivo_invalido"
	codigoValidacion      = "validacion"
	codigoReferencia      = "referencia_invalida"
	codigoDuplicada       = "fuente_duplicada"
	codigoLimiteFuentes   = "limite_fuentes"
	codigoNoEncontrado    = "no_encontrado"
	codigoEstadoInvalido  = "estado_invalido"
	codigoLimiteVersiones = "limite_versiones"
	msgCuerpoInvalido     = "El formulario multipart con el archivo es obligatorio."
	msgNoEncontrado       = "Fuente no encontrada."
	msgNoAutenticado      = "No autenticado."
	campoArchivo          = "file"
)

// servicio is what the handler needs from the Service.
type servicio interface {
	Crear(ctx context.Context, archivo ArchivoSubido, formulario FormularioFuente, actor *int64, aud Auditor) (FuenteDetalle, error)
	NuevaVersion(ctx context.Context, fuenteID string, archivo ArchivoSubido, actor *int64, aud Auditor) (FuenteDetalle, error)
}

// Handler exposes S1 and S2.
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
	httpx.JSONErrorCode(w, http.StatusRequestEntityTooLarge, codigoArchivoGrande, ErrArchivoDemasiadoGrande.Error(), nil)
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
