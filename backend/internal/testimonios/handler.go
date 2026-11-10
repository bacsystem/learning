package testimonios

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cbaciliod/learning/internal/httpx"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

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
	httpx.InternalErrorMsg(w, "testimonios."+op, err, msg)
}

// maxPage bounds ?page so the OFFSET can never overflow.
const maxPage = 1_000_000

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func claimsFrom(r *http.Request) *jwtpkg.Claims {
	return auth.ClaimsFromContext(r.Context())
}

// GET /testimonios/publico
func (h *Handler) GetPublicos(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.GetPublicos(r.Context())
	if err != nil {
		internalError(w, "publicos", err, "Error al obtener testimonios.")
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// GET /testimonios/me
func (h *Handler) GetMio(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	t, err := h.svc.GetMiTestimonio(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, "mio", err, "Error al obtener tu testimonio.")
		return
	}
	jsonOK(w, http.StatusOK, t)
}

// POST /testimonios
func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, "El comentario debe tener entre 10 y 300 caracteres y la calificación entre 1 y 5.")
		return
	}
	claims := claimsFrom(r)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	if err := h.svc.Submit(r.Context(), claims.UserID, req.Contenido, req.Calificacion); err != nil {
		internalError(w, "submit", err, "Error al guardar tu testimonio.")
		return
	}
	jsonOK(w, http.StatusCreated, map[string]string{"message": "Testimonio enviado. Pendiente de aprobación."})
}

// GET /admin/testimonios
func (h *Handler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	estado := q.Get("estado")
	page, _ := httpx.ParsePage(q.Get("page"), maxPage)
	perPage, _ := httpx.ParsePerPage(q.Get("per_page"), 20, 100)
	resp, err := h.svc.ListAdmin(r.Context(), estado, page, perPage)
	if err != nil {
		internalError(w, "list_admin", err, "Error al obtener testimonios.")
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

// PATCH /admin/testimonios/{id}/estado
func (h *Handler) UpdateEstado(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	var req UpdateEstadoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, "Estado inválido. Use: aprobado o rechazado.")
		return
	}
	if err := h.svc.UpdateEstado(r.Context(), id, req.Estado); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "Testimonio no encontrado.")
			return
		}
		internalError(w, "update_estado", err, "Error al actualizar estado.")
		return
	}
	if claims := claimsFrom(r); claims != nil {
		h.auditLog.Log(r.Context(), &claims.UserID, "actualizar_estado_testimonio", "testimonio", &id,
			map[string]any{"estado": req.Estado},
			audit.ClientIP(r), r.UserAgent())
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Estado actualizado."})
}
