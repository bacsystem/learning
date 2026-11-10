package notifications

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cbaciliod/learning/internal/httpx"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type Handler struct {
	repo     *Repository
	auditLog *audit.Logger
}

func NewHandler(repo *Repository, auditLog *audit.Logger) *Handler {
	return &Handler{repo: repo, auditLog: auditLog}
}

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// internalError logs the underlying cause (no PII) and answers a generic 500.
func internalError(w http.ResponseWriter, op string, err error) {
	httpx.InternalError(w, "notifications."+op, err)
}

// ── User endpoints ─────────────────────────────────────────────────────────────

// List returns notifications visible to the current user.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "no autorizado")
		return
	}
	rows, err := h.repo.ListForUser(r.Context(), claims.UserID, claims.Rol)
	if err != nil {
		internalError(w, "list", err)
		return
	}
	jsonOK(w, http.StatusOK, rows)
}

// UnreadCount returns the count of unread notifications for the current user.
func (h *Handler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "no autorizado")
		return
	}
	count, err := h.repo.UnreadCount(r.Context(), claims.UserID, claims.Rol)
	if err != nil {
		internalError(w, "unread_count", err)
		return
	}
	jsonOK(w, http.StatusOK, UnreadCountResponse{Count: count})
}

// MarkRead marks a single notification as read.
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "no autorizado")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	if err := h.repo.MarkRead(r.Context(), id, claims.UserID, claims.Rol); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
			return
		}
		internalError(w, "mark_read", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead marks all notifications as read for the current user.
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "no autorizado")
		return
	}
	if err := h.repo.MarkAllRead(r.Context(), claims.UserID, claims.Rol); err != nil {
		internalError(w, "mark_all_read", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Admin endpoints ────────────────────────────────────────────────────────────

// AdminList returns all notifications for the admin panel.
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.ListAll(r.Context())
	if err != nil {
		internalError(w, "admin_list", err)
		return
	}
	jsonOK(w, http.StatusOK, rows)
}

// AdminCreate creates and sends a new notification.
func (h *Handler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "no autorizado")
		return
	}
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	req.Destinatario = NormalizeDestinatario(req.Destinatario)
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	n, err := h.repo.Create(r.Context(), req, claims.UserID)
	if err != nil {
		internalError(w, "create", err)
		return
	}
	h.auditLog.Log(r.Context(), &claims.UserID, "crear_notificacion", "notificacion", &n.ID,
		map[string]any{"titulo": req.Titulo, "destinatario": req.Destinatario},
		audit.ClientIP(r), r.UserAgent())
	jsonOK(w, http.StatusCreated, n)
}

// AdminDelete deletes a notification.
func (h *Handler) AdminDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
			return
		}
		internalError(w, "delete", err)
		return
	}
	var adminID *int64
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		adminID = &claims.UserID
	}
	h.auditLog.Log(r.Context(), adminID, "eliminar_notificacion", "notificacion", &id, nil,
		audit.ClientIP(r), r.UserAgent())
	w.WriteHeader(http.StatusNoContent)
}
