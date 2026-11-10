package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type AdminHandler struct {
	svc *Service
}

func NewAdminHandler(svc *Service) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) ListUsuarios(w http.ResponseWriter, r *http.Request) {
	estado := r.URL.Query().Get("estado")
	list, err := h.svc.repo.ListUsuarios(r.Context(), estado)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	if list == nil {
		list = []UsuarioPendiente{}
	}
	jsonOK(w, http.StatusOK, list)
}

func (h *AdminHandler) BatchImport(w http.ResponseWriter, r *http.Request) {
	var req BatchPersonasRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}

	claims := ClaimsFromContext(r.Context())
	resp, err := h.svc.BatchImport(r.Context(), req.Personas, claims.UserID, clientIP(r))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}

	jsonOK(w, http.StatusOK, resp)
}

func (h *AdminHandler) UpdateEmail(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "ID de usuario inválido.")
		return
	}

	var req struct {
		Email string `json:"email" validate:"required,email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, "El correo no es válido.")
		return
	}

	claims := ClaimsFromContext(r.Context())
	if err := h.svc.AdminUpdateUsuarioEmail(r.Context(), targetID, req.Email, claims.UserID, clientIP(r)); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			jsonError(w, http.StatusNotFound, "Usuario no encontrado.")
		case errors.Is(err, ErrEmailTaken):
			jsonError(w, http.StatusConflict, "El correo ya está asociado a otra persona.")
		default:
			jsonError(w, http.StatusInternalServerError, "Error al actualizar el correo.")
		}
		return
	}

	jsonOK(w, http.StatusOK, map[string]string{"message": "Correo actualizado correctamente."})
}

func (h *AdminHandler) GetConfigSesiones(w http.ResponseWriter, r *http.Request) {
	max, err := h.svc.repo.GetMaxSesiones(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]int{"max_sesiones": max})
}

func (h *AdminHandler) SetConfigSesiones(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaxSesiones int `json:"max_sesiones" validate:"required,min=1,max=10"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, "max_sesiones debe ser entre 1 y 10.")
		return
	}
	claims := ClaimsFromContext(r.Context())
	if err := h.svc.repo.SetMaxSesiones(r.Context(), req.MaxSesiones, &AuditEntry{
		UsuarioID: &claims.UserID,
		Accion:    "config_sesiones",
		Entidad:   "configuracion_sistema",
		Detalle:   auditJSON(map[string]any{"max_sesiones": req.MaxSesiones}),
		IP:        clientIP(r),
	}); err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]int{"max_sesiones": req.MaxSesiones})
}

func (h *AdminHandler) GetConfigRecursos(w http.ResponseWriter, r *http.Request) {
	raw, err := h.svc.repo.GetRecursos(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	items := []Recurso{}
	if e := json.Unmarshal([]byte(raw), &items); e != nil || items == nil {
		items = []Recurso{} // JSON corrupto → lista vacía
	}
	jsonOK(w, http.StatusOK, items)
}

func (h *AdminHandler) SetConfigRecursos(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recursos []Recurso `json:"recursos" validate:"dive"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Recursos == nil {
		req.Recursos = []Recurso{}
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, "Datos inválidos: revise el título y la URL de cada recurso.")
		return
	}
	data, err := json.Marshal(req.Recursos)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	claims := ClaimsFromContext(r.Context())
	if err := h.svc.repo.SetRecursos(r.Context(), string(data), &AuditEntry{
		UsuarioID: &claims.UserID,
		Accion:    "config_recursos",
		Entidad:   "configuracion_sistema",
		Detalle:   auditJSON(map[string]any{"recursos": len(req.Recursos)}),
		IP:        clientIP(r),
	}); err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, req.Recursos)
}

func (h *AdminHandler) ListPersonas(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.repo.ListPersonas(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	if list == nil {
		list = []PersonaListItem{}
	}
	jsonOK(w, http.StatusOK, list)
}

func (h *AdminHandler) GetPersona(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "ID de persona inválido.")
		return
	}
	p, err := h.svc.repo.GetPersona(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "Persona no encontrada.")
		return
	}
	jsonOK(w, http.StatusOK, p)
}

func (h *AdminHandler) UpdatePersona(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "ID de persona inválido.")
		return
	}
	var req UpdatePersonaRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}
	claims := ClaimsFromContext(r.Context())
	if err := h.svc.AdminUpdatePersona(r.Context(), id, req, claims.UserID, clientIP(r)); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			jsonError(w, http.StatusNotFound, "Persona no encontrada.")
		case errors.Is(err, ErrEmailTaken):
			jsonError(w, http.StatusConflict, "El correo ya está asociado a otra persona.")
		default:
			jsonError(w, http.StatusInternalServerError, "Error al actualizar persona.")
		}
		return
	}
	p, err := h.svc.repo.GetPersona(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, p)
}

func (h *AdminHandler) PreviewPersonasCSV(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		jsonError(w, http.StatusBadRequest, "Error al procesar el formulario.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Archivo requerido (campo 'file').")
		return
	}
	defer file.Close()

	rows, err := parsePersonasFile(file, header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	jsonOK(w, http.StatusOK, previewPersonasRows(rows))
}

func (h *AdminHandler) ImportPersonasCSV(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		jsonError(w, http.StatusBadRequest, "Error al procesar el formulario.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Archivo requerido (campo 'file').")
		return
	}
	defer file.Close()

	rows, err := parsePersonasFile(file, header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	var items []BatchPersonaItem
	var parseErrors []string
	for i, row := range rows {
		if errMsg := validatePersonaImportRow(row); errMsg != "" {
			parseErrors = append(parseErrors, fmt.Sprintf("fila %d: %s", i+2, errMsg))
			continue
		}
		items = append(items, BatchPersonaItem{
			DNI:             row.DNI,
			Nombres:         row.Nombres,
			ApellidoPaterno: row.ApellidoPaterno,
			ApellidoMaterno: row.ApellidoMaterno,
			GradoID:         row.GradoID,
			EspecialidadID:  row.EspecialidadID,
			InstitucionID:   row.InstitucionID,
			Telefono:        row.Telefono,
			Email:           row.Email,
		})
	}

	claims := ClaimsFromContext(r.Context())
	resp, err := h.svc.BatchImport(r.Context(), items, claims.UserID, clientIP(r))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error al importar personas.")
		return
	}
	resp.Errors = append(parseErrors, resp.Errors...)
	jsonOK(w, http.StatusOK, resp)
}

func (h *AdminHandler) UpdateEstado(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "ID de usuario inválido.")
		return
	}

	var req UpdateUsuarioEstadoRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}

	claims := ClaimsFromContext(r.Context())
	resp, err := h.svc.UpdateEstado(r.Context(), targetID, req, claims.UserID, clientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			jsonError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrCannotChangeOwnEstado), errors.Is(err, ErrLastAdmin):
			jsonError(w, http.StatusConflict, err.Error())
		default:
			jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		}
		return
	}

	jsonOK(w, http.StatusOK, resp)
}

// ResetRegistroLimites (DELETE /admin/registro-limites, body {"dni"}) clears
// the persisted registration and check-dni counters of the persona of dni
// and its accepted-registrations cap. The DNI travels in the body, not the
// path, so it does not end up in access logs; the audit row names the
// persona id only.
func (h *AdminHandler) ResetRegistroLimites(w http.ResponseWriter, r *http.Request) {
	var req ResetRegistroLimitesRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}
	claims := ClaimsFromContext(r.Context())
	if err := h.svc.ResetRegistroLimites(r.Context(), req.DNI, claims.UserID, clientIP(r)); err != nil {
		if errors.Is(err, ErrDNINotFound) {
			jsonError(w, http.StatusNotFound, "Persona no encontrada.")
			return
		}
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"message": "Límites de registro restablecidos."})
}

func (h *AdminHandler) ResetTourUsuario(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "ID de usuario inválido.")
		return
	}
	claims := ClaimsFromContext(r.Context())
	if err := h.svc.repo.ResetTourByAdmin(r.Context(), targetID, AuditEntry{
		UsuarioID: &claims.UserID,
		Accion:    "reset_tour",
		Entidad:   "usuarios",
		EntidadID: &targetID,
		IP:        clientIP(r),
	}); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			jsonError(w, http.StatusNotFound, "Usuario no encontrado.")
			return
		}
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]bool{"tour_completado": false})
}

func (h *AdminHandler) ListUsuariosOnline(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListUsuariosOnline(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"data": items, "total": len(items)})
}

func (h *AdminHandler) ListSesionesUsuario(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "ID inválido.")
		return
	}
	items, err := h.svc.ListSesionesUsuario(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"data": items, "total": len(items)})
}
