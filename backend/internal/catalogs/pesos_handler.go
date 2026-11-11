package catalogs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/httpx"
	"github.com/jmoiron/sqlx"
)

// Stable codes of the weights endpoints (docs/api/README.md §4).
const (
	codigoCuerpoInvalido     = "cuerpo_invalido"
	codigoParametroInvalido  = "parametro_invalido"
	codigoNoEncontrado       = "no_encontrado"
	codigoValidacion         = "validacion"
	codigoReferenciaInvalida = "referencia_invalida"
	codigoLimitePreguntas    = "limite_preguntas"
	codigoPesosNoSuman100    = "pesos_no_suman_100"

	accionPesosGuardar = "pesos.guardar"
	entidadProceso     = "proceso"

	msgProcesoNoEncontrado = "Proceso no encontrado."
	msgCuerpoInvalido      = "Cuerpo de solicitud inválido."
	msgParametroInvalido   = "Parámetro inválido."
)

// ResumenPesos handles GET /admin/procesos/{id}/pesos (PE1).
func (h *Handler) ResumenPesos(w http.ResponseWriter, r *http.Request) {
	procesoID, ok := pathID(r, "id")
	if !ok {
		parametroInvalido(w)
		return
	}
	res, err := h.svc.ResumenPesos(r.Context(), procesoID)
	if err != nil {
		responderErrorPesos(w, "resumen_pesos", err)
		return
	}
	jsonOK(w, http.StatusOK, res)
}

// ConfiguracionPesos handles GET /admin/procesos/{id}/pesos/{especialidadId} (PE2).
func (h *Handler) ConfiguracionPesos(w http.ResponseWriter, r *http.Request) {
	procesoID, especialidadID, ok := idsPesos(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	c, err := h.svc.ConfiguracionPesos(r.Context(), procesoID, especialidadID)
	if err != nil {
		responderErrorPesos(w, "configuracion_pesos", err)
		return
	}
	jsonOK(w, http.StatusOK, c)
}

// GuardarPesos handles PUT /admin/procesos/{id}/pesos/{especialidadId} (PE3).
func (h *Handler) GuardarPesos(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.JSONError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	procesoID, especialidadID, ok := idsPesos(r)
	if !ok {
		parametroInvalido(w)
		return
	}
	var req GuardarPesosRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.JSONErrorCode(w, http.StatusBadRequest, codigoCuerpoInvalido, msgCuerpoInvalido, nil)
		return
	}
	actor, ip, ua := claims.UserID, audit.ClientIP(r), r.UserAgent()
	aud := func(ctx context.Context, tx sqlx.ExecerContext, detalle map[string]any) error {
		return h.auditLog.LogTx(ctx, tx, &actor, accionPesosGuardar, entidadProceso, &procesoID, detalle, ip, ua)
	}
	c, err := h.svc.GuardarPesos(r.Context(), procesoID, especialidadID, req, aud)
	if err != nil {
		responderErrorPesos(w, "guardar_pesos", err)
		return
	}
	jsonOK(w, http.StatusOK, c)
}

// responderErrorPesos translates the weights errors into the coded format;
// anything else is logged under op and answered as error_interno.
func responderErrorPesos(w http.ResponseWriter, op string, err error) {
	var detalle map[string]any
	msg := err.Error()
	var ep *errorPesos
	if errors.As(err, &ep) {
		detalle, msg = ep.detalle, ep.msg
	}
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.JSONErrorCode(w, http.StatusNotFound, codigoNoEncontrado, msgProcesoNoEncontrado, nil)
	case errors.Is(err, ErrPesosValidacion):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoValidacion, msg, detalle)
	case errors.Is(err, ErrInvalidRef):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoReferenciaInvalida, msg, nil)
	case errors.Is(err, ErrPesoDuplicado):
		httpx.JSONErrorCode(w, http.StatusConflict, codigoPesoDuplicado, msg, detalle)
	case errors.Is(err, ErrLimitePreguntas):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoLimitePreguntas, msg, detalle)
	case errors.Is(err, ErrPesosNoSuman100):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoPesosNoSuman100, msg, detalle)
	default:
		httpx.InternalErrorCode(w, "catalogs."+op, err)
	}
}

func idsPesos(r *http.Request) (procesoID, especialidadID int64, ok bool) {
	procesoID, okP := pathID(r, "id")
	especialidadID, okE := pathID(r, "especialidadId")
	return procesoID, especialidadID, okP && okE
}

func parametroInvalido(w http.ResponseWriter) {
	httpx.JSONErrorCode(w, http.StatusBadRequest, codigoParametroInvalido, msgParametroInvalido, nil)
}
