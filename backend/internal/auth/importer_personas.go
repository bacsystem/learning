package auth

import (
	"encoding/csv"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

const maxPersonaImportRows = 1000

type personaImportRow struct {
	DNI             string
	Nombres         string
	ApellidoPaterno string
	ApellidoMaterno string
	GradoID         int64
	EspecialidadID  int64
	InstitucionID   int64
	Telefono        string
	Email           string
}

// PersonaPreviewRow is one row returned in the preview response.
type PersonaPreviewRow struct {
	Row             int    `json:"row"`
	DNI             string `json:"dni"`
	Nombres         string `json:"nombres"`
	ApellidoPaterno string `json:"apellido_paterno"`
	ApellidoMaterno string `json:"apellido_materno"`
	GradoID         int64  `json:"grado_id"`
	EspecialidadID  int64  `json:"especialidad_id"`
	InstitucionID   int64  `json:"institucion_id"`
	Telefono        string `json:"telefono"`
	Email           string `json:"email"`
	Estado          string `json:"estado"` // "valido" | "error"
	Error           string `json:"error,omitempty"`
}

// PersonaPreviewResponse is the response body for POST /admin/personas/preview.
type PersonaPreviewResponse struct {
	Rows        []PersonaPreviewRow `json:"rows"`
	TotalValido int                 `json:"total_valido"`
	TotalError  int                 `json:"total_error"`
}

func isPersonaXLSX(filename, contentType string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".xlsx") ||
		strings.Contains(contentType, "spreadsheetml") ||
		strings.Contains(contentType, "openxmlformats")
}

// parsePersonaColumns converts a row of strings into a personaImportRow.
// Expected column order: dni, nombres, apellido_paterno, apellido_materno,
// grado_id, especialidad_id, institucion_id, telefono, email
func parsePersonaColumns(cols []string) personaImportRow {
	get := func(i int) string {
		if i < len(cols) {
			return strings.TrimSpace(cols[i])
		}
		return ""
	}
	gradoID, _ := strconv.ParseInt(get(4), 10, 64)
	espID, _ := strconv.ParseInt(get(5), 10, 64)
	instID, _ := strconv.ParseInt(get(6), 10, 64)
	return personaImportRow{
		DNI:             get(0),
		Nombres:         get(1),
		ApellidoPaterno: get(2),
		ApellidoMaterno: get(3),
		GradoID:         gradoID,
		EspecialidadID:  espID,
		InstitucionID:   instID,
		Telefono:        get(7),
		Email:           get(8),
	}
}

func parsePersonasCSVFile(file multipart.File) ([]personaImportRow, error) {
	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error al leer CSV: %w", err)
	}
	if len(records) > 0 {
		records = records[1:] // skip header
	}
	if len(records) > maxPersonaImportRows {
		return nil, fmt.Errorf("el archivo excede el máximo de %d filas", maxPersonaImportRows)
	}
	rows := make([]personaImportRow, 0, len(records))
	for _, cols := range records {
		rows = append(rows, parsePersonaColumns(cols))
	}
	return rows, nil
}

func parsePersonasXLSXFile(file multipart.File) ([]personaImportRow, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("error al leer Excel: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("el archivo Excel no tiene hojas")
	}
	allRows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("error al leer filas: %w", err)
	}
	if len(allRows) > 0 {
		allRows = allRows[1:] // skip header
	}
	if len(allRows) > maxPersonaImportRows {
		return nil, fmt.Errorf("el archivo excede el máximo de %d filas", maxPersonaImportRows)
	}
	rows := make([]personaImportRow, 0, len(allRows))
	for _, cols := range allRows {
		rows = append(rows, parsePersonaColumns(cols))
	}
	return rows, nil
}

func validatePersonaImportRow(r personaImportRow) string {
	if len(r.DNI) != 8 || !isAllDigits(r.DNI) {
		return "DNI debe tener exactamente 8 dígitos numéricos"
	}
	if len(strings.TrimSpace(r.Nombres)) < 2 {
		return "nombres inválido (mínimo 2 caracteres)"
	}
	if len(strings.TrimSpace(r.ApellidoPaterno)) < 2 {
		return "apellido_paterno inválido (mínimo 2 caracteres)"
	}
	if r.GradoID <= 0 {
		return "grado_id inválido (debe ser un número positivo)"
	}
	if r.EspecialidadID <= 0 {
		return "especialidad_id inválido (debe ser un número positivo)"
	}
	if r.InstitucionID <= 0 {
		return "institucion_id inválido (debe ser un número positivo)"
	}
	if !strings.Contains(r.Email, "@") || !strings.Contains(r.Email, ".") {
		return "email inválido"
	}
	return ""
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func previewPersonasRows(rows []personaImportRow) PersonaPreviewResponse {
	result := PersonaPreviewResponse{Rows: make([]PersonaPreviewRow, 0, len(rows))}
	for i, r := range rows {
		pr := PersonaPreviewRow{
			Row:             i + 2, // 1-based, accounting for header
			DNI:             r.DNI,
			Nombres:         r.Nombres,
			ApellidoPaterno: r.ApellidoPaterno,
			ApellidoMaterno: r.ApellidoMaterno,
			GradoID:         r.GradoID,
			EspecialidadID:  r.EspecialidadID,
			InstitucionID:   r.InstitucionID,
			Telefono:        r.Telefono,
			Email:           r.Email,
		}
		if errMsg := validatePersonaImportRow(r); errMsg != "" {
			pr.Estado = "error"
			pr.Error = errMsg
			result.TotalError++
		} else {
			pr.Estado = "valido"
			result.TotalValido++
		}
		result.Rows = append(result.Rows, pr)
	}
	return result
}

func parsePersonasFile(file multipart.File, filename, contentType string) ([]personaImportRow, error) {
	if isPersonaXLSX(filename, contentType) {
		return parsePersonasXLSXFile(file)
	}
	return parsePersonasCSVFile(file)
}
