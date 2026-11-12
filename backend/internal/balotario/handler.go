package balotario

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

const (
	codigoParametro = "parametro_invalido"
	msgParametro    = "Parámetro inválido."
	msgNoEncontrado = "Registro no encontrado."

	msgNoAutenticado = "No autenticado."
	// accionImportar is the action of the audit row of §4.4.
	accionImportar = "balotario.importar"
)

// reglasHTTP maps each rule code to its status and fallback message (§7).
var reglasHTTP = map[string]struct {
	status  int
	mensaje string
}{
	CodigoTipoFuenteInvalido:        {http.StatusUnprocessableEntity, "La fuente no es un balotario."},
	CodigoFuenteNoActiva:            {http.StatusUnprocessableEntity, "La fuente no está activa."},
	CodigoLimitePreguntas:           {http.StatusUnprocessableEntity, "El balotario supera el máximo de preguntas por archivo."},
	CodigoVersionDesactualizada:     {http.StatusConflict, "La fuente tiene una versión más reciente; recarga la vista previa."},
	CodigoYaImportado:               {http.StatusConflict, "Esta versión del balotario ya se importó."},
	CodigoEspecialidadInvalida:      {http.StatusUnprocessableEntity, "La especialidad no existe, está inactiva o no es de la institución del proceso."},
	CodigoEspecialidadNoCorresponde: {http.StatusUnprocessableEntity, "La especialidad no está entre las profesiones de la fuente."},
	CodigoReferenciaInvalida:        {http.StatusUnprocessableEntity, "El proceso o el área no existe o no corresponde."},
	CodigoSinValidas:                {http.StatusUnprocessableEntity, "El balotario no tiene ninguna pregunta válida para importar."},
	CodigoValidacion:                {http.StatusUnprocessableEntity, "Datos inválidos."},
}

// Handler is the transport of the balotario import.
type Handler struct {
	svc    *Service
	audLog *audit.Logger
}

func NewHandler(svc *Service, audLog *audit.Logger) *Handler {
	return &Handler{svc: svc, audLog: audLog}
}

// VistaPrevia handles GET /admin/fuentes/{id}/balotario (§3).
func (h *Handler) VistaPrevia(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(w, r)
	if !ok {
		return
	}
	page, err := httpx.ParsePage(r.URL.Query().Get("page"), MaxPagina)
	if err != nil {
		parametroInvalido(w)
		return
	}
	perPage, err := httpx.ParsePerPage(r.URL.Query().Get("per_page"), PerPageDefecto, PerPageMax)
	if err != nil {
		parametroInvalido(w)
		return
	}
	resp, err := h.svc.VistaPrevia(r.Context(), id, page, perPage)
	if err != nil {
		responderError(w, "balotario.vista_previa", err)
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// Confirmar handles POST /admin/fuentes/{id}/balotario/importaciones (§4).
func (h *Handler) Confirmar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(w, r)
	if !ok {
		return
	}
	var req ConfirmarRequest
	if !httpx.DecodeJSONStrict(w, r, &req) {
		return
	}
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.JSONError(w, http.StatusUnauthorized, msgNoAutenticado)
		return
	}
	resp, err := h.svc.Confirmar(r.Context(), id, req, claims.UserID, h.auditor(claims.UserID, id, audit.ClientIP(r), r.UserAgent()))
	if err != nil {
		responderError(w, "balotario.confirmar", err)
		return
	}
	jsonOK(w, http.StatusCreated, resp)
}

// auditor binds the admin, the IP and the user agent to the audit row of the import (§4.4).
func (h *Handler) auditor(actor, fuenteID int64, ip, ua string) Auditor {
	return func(ctx context.Context, tx sqlx.ExecerContext, detalle map[string]any) error {
		return h.audLog.LogTx(ctx, tx, &actor, accionImportar, "fuente", &fuenteID, detalle, ip, ua)
	}
}

func idDeRuta(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		parametroInvalido(w)
		return 0, false
	}
	return id, true
}

func parametroInvalido(w http.ResponseWriter) {
	httpx.JSONErrorCode(w, http.StatusBadRequest, codigoParametro, msgParametro, nil)
}

// responderError answers a rule or a not-found in the coded format (format 2).
func responderError(w http.ResponseWriter, op string, err error) {
	var re *ReglaError
	switch {
	case errors.As(err, &re):
		resp, ok := reglasHTTP[re.Codigo]
		if !ok {
			httpx.InternalErrorCode(w, op, err)
			return
		}
		httpx.JSONErrorCode(w, resp.status, re.Codigo, resp.mensaje, re.Detalle)
	case errors.Is(err, ErrNoEncontrado):
		httpx.JSONErrorCode(w, http.StatusNotFound, CodigoNoEncontrado, msgNoEncontrado, nil)
	default:
		httpx.InternalErrorCode(w, op, err)
	}
}

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
