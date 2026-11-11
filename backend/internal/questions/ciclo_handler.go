package questions

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/httpx"
	"github.com/jmoiron/sqlx"
)

const (
	codigoNoEncontrado = "no_encontrado"
	codigoParametro    = "parametro_invalido"
	msgNoEncontrado    = "Registro no encontrado."
	msgParametro       = "Parámetro inválido."
	msgNoAutenticado   = "No autenticado."
)

// respuestaRegla is the HTTP face of one life-cycle rule.
type respuestaRegla struct {
	status  int
	mensaje string
}

// reglasHTTP maps each rule code to its status and fallback message
// (contract E3-02 §7, E3-07 §7).
var reglasHTTP = map[string]respuestaRegla{
	CodigoEstadoInvalido:        {http.StatusConflict, "La pregunta cambió o la operación no aplica a su estado; recárgala."},
	CodigoRevisionEspecializada: {http.StatusConflict, "Esta pregunta se revisa en su cola especializada."},
	CodigoUltimaCitaActiva:      {http.StatusConflict, "Una pregunta aprobada debe conservar al menos una cita a una fuente activa."},
	CodigoFaltaCita:             {http.StatusUnprocessableEntity, "La pregunta necesita al menos una cita a una fuente activa."},
	CodigoNotaObligatoria:       {http.StatusUnprocessableEntity, "Revalidar esta pregunta exige una nota."},
	CodigoVerificacionRequerida: {http.StatusUnprocessableEntity, "Confirma que verificaste la pregunta y su clave contra la cita."},
	CodigoLimiteCitas:           {http.StatusUnprocessableEntity, "Se superó el máximo de citas por pregunta."},
	CodigoFuenteNoActiva:        {http.StatusUnprocessableEntity, "Alguna cita nueva es de una fuente que no está activa."},
	CodigoReferenciaInvalida:    {http.StatusUnprocessableEntity, "Algún pasaje no existe."},
}

// CambiarEstado handles PATCH /preguntas/{id}/estado (E3-07 §3).
func (h *Handler) CambiarEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(w, r)
	if !ok {
		return
	}
	var req CambiarEstadoRequest
	if !httpx.DecodeJSONStrict(w, r, &req) {
		return
	}
	aud, ok := h.auditor(w, r)
	if !ok {
		return
	}
	p, err := h.svc.CambiarEstado(r.Context(), id, req, aud)
	if err != nil {
		responderRegla(w, "questions.cambiar_estado", err)
		return
	}
	jsonOK(w, http.StatusOK, p)
}

// idDeRuta reads {id} for the coded endpoints (400 parametro_invalido).
func idDeRuta(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.JSONErrorCode(w, http.StatusBadRequest, codigoParametro, msgParametro, nil)
		return 0, false
	}
	return id, true
}

// auditor binds the authenticated admin, IP and user agent to an Auditor.
func (h *Handler) auditor(w http.ResponseWriter, r *http.Request) (Auditor, bool) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.JSONError(w, http.StatusUnauthorized, msgNoAutenticado)
		return nil, false
	}
	actor, ip, ua := claims.UserID, audit.ClientIP(r), r.UserAgent()
	return func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error {
		return h.audLog.LogTx(ctx, tx, &actor, ev.Accion, "pregunta", &ev.EntidadID, ev.Detalle, ip, ua)
	}, true
}

// responderRegla answers a life-cycle error in the coded format.
func responderRegla(w http.ResponseWriter, op string, err error) {
	var re *ReglaError
	var ve *ValidacionPreguntaError
	switch {
	case errors.As(err, &re):
		resp, ok := reglasHTTP[re.Codigo]
		if !ok {
			httpx.InternalErrorCode(w, op, err)
			return
		}
		httpx.JSONErrorCode(w, resp.status, re.Codigo, resp.mensaje, re.Detalle)
	case errors.As(err, &ve):
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, CodigoValidacion, "Datos inválidos.", map[string]any{"campos": ve.Campos})
	case errors.Is(err, ErrNotFound):
		httpx.JSONErrorCode(w, http.StatusNotFound, codigoNoEncontrado, msgNoEncontrado, nil)
	default:
		httpx.InternalErrorCode(w, op, err)
	}
}
