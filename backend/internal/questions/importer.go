package questions

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	maxImportRows = 500
)

// importRow maps one Excel/CSV row into a create request.
type importRow struct {
	Enunciado         string
	Tipo              string
	Dificultad        string
	GradoID           *int64
	EspecialidadID    *int64
	Respuestas        []CreateRespuestaItem // up to 6
	Retroalimentacion *CreateRetroRequest
	Referencia        string
}

// isXLSX returns true when the filename or content-type indicates an xlsx file.
func isXLSX(filename, contentType string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".xlsx") ||
		strings.Contains(contentType, "spreadsheetml") ||
		strings.Contains(contentType, "openxmlformats")
}

// ParseExcel parses an xlsx file and returns importRows.
// Expected column order:
//
//	A: enunciado | B: tipo | C: dificultad | D: grado_id | E: especialidad_id
//	F: resp_1 | G: correcta_1 | H: resp_2 | I: correcta_2 | J: resp_3 | K: correcta_3
//	L: resp_4 | M: correcta_4 | N: resp_5 | O: correcta_5 | P: resp_6 | Q: correcta_6
//	R: retroalimentacion | S: referencia
func ParseExcel(file multipart.File) ([]importRow, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("error al leer archivo Excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("el archivo Excel no tiene hojas")
	}

	allRows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("error al leer filas: %w", err)
	}

	// Skip header row if it exists
	if len(allRows) > 0 {
		allRows = allRows[1:]
	}

	if len(allRows) > maxImportRows {
		return nil, fmt.Errorf("el archivo excede el máximo de %d filas", maxImportRows)
	}

	var result []importRow
	for _, cols := range allRows {
		row, err := parseColumns(cols)
		if err != nil {
			// Return raw rows even with parse errors; validation per row done in ImportPreguntas
			result = append(result, importRow{})
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

// ParseCSV parses a CSV file and returns importRows.
// Expected column order matches ParseExcel.
func ParseCSV(file multipart.File) ([]importRow, error) {
	reader := csv.NewReader(file)
	reader.LazyQuotes = true

	allRecords, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error al leer CSV: %w", err)
	}

	// Skip header row
	if len(allRecords) > 0 {
		allRecords = allRecords[1:]
	}

	if len(allRecords) > maxImportRows {
		return nil, fmt.Errorf("el archivo excede el máximo de %d filas", maxImportRows)
	}

	var result []importRow
	for _, cols := range allRecords {
		row, err := parseColumns(cols)
		if err != nil {
			result = append(result, importRow{})
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

// parseColumns converts a slice of string columns into an importRow.
func parseColumns(cols []string) (importRow, error) {
	get := func(i int) string {
		if i < len(cols) {
			return strings.TrimSpace(cols[i])
		}
		return ""
	}

	row := importRow{
		Enunciado:  get(0),
		Tipo:       get(1),
		Dificultad: get(2),
	}

	// Optional grado_id
	if s := get(3); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err == nil && id > 0 {
			row.GradoID = &id
		}
	}

	// Optional especialidad_id
	if s := get(4); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err == nil && id > 0 {
			row.EspecialidadID = &id
		}
	}

	// Respuestas: columns 5–16 (resp_1/correcta_1 through resp_6/correcta_6)
	for i := 0; i < 6; i++ {
		respCol := 5 + (i * 2)
		correctaCol := respCol + 1
		texto := get(respCol)
		if texto == "" {
			break
		}
		correctaStr := strings.ToLower(get(correctaCol))
		esCorrecta := correctaStr == "true" || correctaStr == "1" || correctaStr == "si" || correctaStr == "yes"
		row.Respuestas = append(row.Respuestas, CreateRespuestaItem{
			Texto:      texto,
			EsCorrecta: esCorrecta,
			Orden:      i + 1,
		})
	}

	// Retroalimentacion at column 17 (R), referencia at 18 (S)
	if retro := get(17); retro != "" {
		ref := get(18)
		var refPtr *string
		if ref != "" {
			refPtr = &ref
		}
		row.Retroalimentacion = &CreateRetroRequest{
			Explicacion: retro,
			Referencia:  refPtr,
		}
	}

	return row, nil
}

// PreviewPreguntas validates parsed rows without creating them.
func PreviewPreguntas(rows []importRow) PreviewResponse {
	result := PreviewResponse{Rows: []PreviewRowResponse{}}
	for i, row := range rows {
		rowNum := i + 2
		pr := PreviewRowResponse{
			Row:            rowNum,
			Enunciado:      row.Enunciado,
			Tipo:           row.Tipo,
			Dificultad:     row.Dificultad,
			GradoID:        row.GradoID,
			EspecialidadID: row.EspecialidadID,
			NumRespuestas:  len(row.Respuestas),
			HasRetro:       row.Retroalimentacion != nil,
		}
		if pr.Tipo == "" {
			pr.Tipo = "unica"
		}
		if pr.Dificultad == "" {
			pr.Dificultad = "medio"
		}
		if pr.Enunciado == "" {
			pr.Estado = "error"
			pr.Error = "enunciado vacío"
		} else if err := ValidateRespuestas(pr.Tipo, row.Respuestas); err != nil {
			pr.Estado = "error"
			pr.Error = err.Error()
		} else {
			pr.Estado = "valido"
			result.TotalValido++
		}
		if pr.Estado == "error" {
			result.TotalError++
		}
		result.Rows = append(result.Rows, pr)
	}
	return result
}

// ImportPreguntas processes parsed rows and creates valid questions.
// Invalid rows are skipped and reported in the result. Repeated statements are
// not detected here: preguntas has no unique key on enunciado (the
// deduplication belongs to SIA-E3-06).
func ImportPreguntas(ctx context.Context, rows []importRow, procesoID, asignaturaID int64, svc *Service) ImportResultResponse {
	result := ImportResultResponse{
		Errors: []ImportErrorRow{},
	}

	for i, row := range rows {
		rowNum := i + 2 // 1-based, skipping header

		if row.Enunciado == "" {
			result.Errors = append(result.Errors, ImportErrorRow{
				Row:     rowNum,
				Message: "enunciado vacío",
			})
			continue
		}

		req := CreatePreguntaRequest{
			ProcesoID:         procesoID,
			AsignaturaID:      asignaturaID,
			GradoID:           row.GradoID,
			EspecialidadID:    row.EspecialidadID,
			Enunciado:         row.Enunciado,
			Tipo:              row.Tipo,
			Dificultad:        row.Dificultad,
			Respuestas:        row.Respuestas,
			EtiquetaIDs:       []int64{},
			Retroalimentacion: row.Retroalimentacion,
		}

		// Default values if missing
		if req.Tipo == "" {
			req.Tipo = "unica"
		}
		if req.Dificultad == "" {
			req.Dificultad = "medio"
		}

		_, err := svc.CreatePregunta(ctx, req)
		if err != nil {
			result.Errors = append(result.Errors, ImportErrorRow{
				Row:     rowNum,
				Message: err.Error(),
			})
			continue
		}

		result.Imported++
	}

	return result
}
