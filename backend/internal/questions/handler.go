package questions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cbaciliod/learning/internal/httpx"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

var validate = validator.New()

// Handler exposes the questions HTTP handlers.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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

func pathID(r *http.Request, key string) (int64, error) {
	val := r.PathValue(key)
	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("ID inválido")
	}
	return id, nil
}

func mapErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrTemaInvalido):
		// Only this error uses the coded format (docs/api/README.md §3, T8).
		httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoTemaInvalido, err.Error(), nil)
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		jsonError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrValidation):
		jsonError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrInsufficientQuestions), errors.Is(err, ErrLimiteExcedido):
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		log.Error().Err(err).Msg("questions: internal error")
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
	}
}

// ── Preguntas ─────────────────────────────────────────────────────────────────

func (h *Handler) ListPreguntas(w http.ResponseWriter, r *http.Request) {
	q := ListQuery{}
	params := r.URL.Query()

	parseInt := func(key string) (int64, bool) {
		s := params.Get(key)
		if s == "" {
			return 0, true
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	var ok bool
	if q.ProcesoID, ok = parseInt("proceso_id"); !ok {
		jsonError(w, http.StatusBadRequest, "proceso_id inválido.")
		return
	}
	if q.AsignaturaID, ok = parseInt("asignatura_id"); !ok {
		jsonError(w, http.StatusBadRequest, "asignatura_id inválido.")
		return
	}
	if q.GradoID, ok = parseInt("grado_id"); !ok {
		jsonError(w, http.StatusBadRequest, "grado_id inválido.")
		return
	}
	if q.EspecialidadID, ok = parseInt("especialidad_id"); !ok {
		jsonError(w, http.StatusBadRequest, "especialidad_id inválido.")
		return
	}
	q.Dificultad = params.Get("dificultad")
	if q.EtiquetaID, ok = parseInt("etiqueta_id"); !ok {
		jsonError(w, http.StatusBadRequest, "etiqueta_id inválido.")
		return
	}
	if q.TemaID, ok = parseInt("tema_id"); !ok {
		jsonError(w, http.StatusBadRequest, "tema_id inválido.")
		return
	}
	q.Estado = params.Get("estado")
	q.Search = params.Get("search")
	if s := params.Get("page"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			q.Page = v
		}
	}
	if s := params.Get("per_page"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			q.PerPage = v
		}
	}

	result, err := h.svc.ListPreguntas(r.Context(), q)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, result)
}

func (h *Handler) GetPregunta(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.svc.GetPregunta(r.Context(), id)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, result)
}

func (h *Handler) CreatePregunta(w http.ResponseWriter, r *http.Request) {
	var req CreatePreguntaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.svc.CreatePregunta(r.Context(), req)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusCreated, result)
}

func (h *Handler) UpdatePregunta(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req UpdatePreguntaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.svc.UpdatePregunta(r.Context(), id, req)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, result)
}

func (h *Handler) DeletePregunta(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.DeletePregunta(r.Context(), id); err != nil {
		mapErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Generate Set ──────────────────────────────────────────────────────────────

func (h *Handler) GenerateSet(w http.ResponseWriter, r *http.Request) {
	var req GenerateSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	total := 0
	for _, a := range req.Asignaturas {
		total += a.NumeroPreguntas
	}
	if total > MaxPreguntasSet {
		mapErr(w, fmt.Errorf("%w: máximo %d preguntas por conjunto", ErrLimiteExcedido, MaxPreguntasSet))
		return
	}
	result, err := h.svc.GenerateSet(r.Context(), req)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, result)
}

// ── Import ────────────────────────────────────────────────────────────────────

func (h *Handler) ImportPreguntas(w http.ResponseWriter, r *http.Request) {
	const maxSize = 5 << 20 // 5 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	if err := r.ParseMultipartForm(maxSize); err != nil {
		jsonError(w, http.StatusBadRequest, "Archivo demasiado grande o formulario inválido.")
		return
	}

	procesoIDStr := r.FormValue("proceso_id")
	asignaturaIDStr := r.FormValue("asignatura_id")
	procesoID, err := strconv.ParseInt(procesoIDStr, 10, 64)
	if err != nil || procesoID <= 0 {
		jsonError(w, http.StatusBadRequest, "proceso_id inválido.")
		return
	}
	asignaturaID, err := strconv.ParseInt(asignaturaIDStr, 10, 64)
	if err != nil || asignaturaID <= 0 {
		jsonError(w, http.StatusBadRequest, "asignatura_id inválido.")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Archivo no encontrado en la solicitud.")
		return
	}
	defer file.Close()

	var rows []importRow
	filename := header.Filename
	if isXLSX(filename, header.Header.Get("Content-Type")) {
		rows, err = ParseExcel(file)
	} else {
		rows, err = ParseCSV(file)
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// H19: reject a foreign asignatura once, instead of failing every row.
	if err := h.svc.ValidarAsignatura(r.Context(), procesoID, asignaturaID); err != nil {
		mapErr(w, err)
		return
	}

	result := ImportPreguntas(r.Context(), rows, procesoID, asignaturaID, h.svc)
	jsonOK(w, http.StatusOK, result)
}

// PreviewPreguntas handles POST /preguntas/preview.
func (h *Handler) PreviewPreguntas(w http.ResponseWriter, r *http.Request) {
	const maxSize = 5 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	if err := r.ParseMultipartForm(maxSize); err != nil {
		jsonError(w, http.StatusBadRequest, "Archivo demasiado grande o formulario inválido.")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Archivo no encontrado en la solicitud.")
		return
	}
	defer file.Close()

	var rows []importRow
	filename := header.Filename
	if isXLSX(filename, header.Header.Get("Content-Type")) {
		rows, err = ParseExcel(file)
	} else {
		rows, err = ParseCSV(file)
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	result := PreviewPreguntas(rows)
	jsonOK(w, http.StatusOK, result)
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.GetStats(r.Context())
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, result)
}

// ── Etiquetas ─────────────────────────────────────────────────────────────────

func (h *Handler) ListEtiquetas(w http.ResponseWriter, r *http.Request) {
	var asignaturaID int64
	if s := r.URL.Query().Get("asignatura_id"); s != "" {
		asignaturaID, _ = strconv.ParseInt(s, 10, 64)
	}
	estado := r.URL.Query().Get("estado")

	result, err := h.svc.ListEtiquetas(r.Context(), asignaturaID, estado)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, result)
}

func (h *Handler) CreateEtiqueta(w http.ResponseWriter, r *http.Request) {
	var req CreateEtiquetaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.svc.CreateEtiqueta(r.Context(), req)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusCreated, result)
}

func (h *Handler) UpdateEtiqueta(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req UpdateEtiquetaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.svc.UpdateEtiqueta(r.Context(), id, req)
	if err != nil {
		mapErr(w, err)
		return
	}
	jsonOK(w, http.StatusOK, result)
}

func (h *Handler) DeleteEtiqueta(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.DeleteEtiqueta(r.Context(), id); err != nil {
		mapErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
