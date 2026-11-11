package fuentes

import (
	"bytes"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Upload limits (SIA-E2-03 regla 1; contract S1).
const (
	MaxArchivoBytes   = 2 * 1024 * 1024 // 2 MiB
	MaxLineasArchivo  = 20000
	maxNombreArchivo  = 255
	minTitulo         = 3
	maxTitulo         = 150
	minAnio           = 1900
	maxAnio           = 2100
	maxEspecialidades = 50
	extensionMD       = ".md"
	reglaRequerido    = "requerido"
	reglaMin          = "min"
	reglaMax          = "max"
	reglaInvalido     = "invalido"
	reglaRepetido     = "repetido"
)

var (
	tiposFuente        = map[string]bool{"balotario": true, "guia": true, "norma": true, "otro": true}
	contentTypesMD     = map[string]bool{"text/markdown": true, "text/plain": true}
	separadoresDeRutas = strings.NewReplacer(`\`, "/")
)

// validarArchivo applies steps 3–6 of S1, in order, and returns the sanitised
// file name. Size (step 1) is enforced by the handler while reading.
func validarArchivo(a ArchivoSubido) (string, error) {
	nombre := sanearNombre(a.Nombre)
	if nombre == "" || utf8.RuneCountInString(nombre) > maxNombreArchivo || !strings.EqualFold(filepath.Ext(nombre), extensionMD) {
		return "", &ArchivoInvalidoError{Motivo: MotivoExtension}
	}
	if tipo, _, err := mime.ParseMediaType(a.ContentType); err != nil || !contentTypesMD[strings.ToLower(tipo)] {
		return "", &ArchivoInvalidoError{Motivo: MotivoTipo}
	}
	if !utf8.Valid(a.Contenido) || bytes.IndexByte(a.Contenido, 0) >= 0 {
		return "", &ArchivoInvalidoError{Motivo: MotivoCodificacion}
	}
	if ContarLineas(a.Contenido) > MaxLineasArchivo {
		return "", &ArchivoInvalidoError{Motivo: MotivoLineas}
	}
	return nombre, nil
}

// sanearNombre keeps the base name (also for Windows paths) without control
// characters; nothing of the client's path is kept (E2-03, Seguridad).
func sanearNombre(nombre string) string {
	base := filepath.Base(separadoresDeRutas.Replace(nombre))
	if base == "." || base == "/" {
		return ""
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, base)
}

// DatosFuente are the validated metadata of S1.
type DatosFuente struct {
	Titulo          string
	Tipo            string
	Anio            *int
	ProcesoID       *int64
	EspecialidadIDs []int64
}

// validarFormulario applies step 7 of S1 (shape only; existence is checked
// under the lock). Every broken field is reported.
func validarFormulario(f FormularioFuente) (DatosFuente, error) {
	campos := map[string]string{}
	d := DatosFuente{Titulo: strings.TrimSpace(f.Titulo), Tipo: strings.TrimSpace(f.Tipo), EspecialidadIDs: []int64{}}
	switch n := utf8.RuneCountInString(d.Titulo); {
	case n == 0:
		campos["titulo"] = reglaRequerido
	case n < minTitulo:
		campos["titulo"] = reglaMin
	case n > maxTitulo:
		campos["titulo"] = reglaMax
	}
	if d.Tipo == "" {
		campos["tipo"] = reglaRequerido
	} else if !tiposFuente[d.Tipo] {
		campos["tipo"] = reglaInvalido
	}
	if s := strings.TrimSpace(f.Anio); s != "" {
		anio, err := strconv.Atoi(s)
		if err != nil || anio < minAnio || anio > maxAnio {
			campos["anio"] = reglaInvalido
		} else {
			d.Anio = &anio
		}
	}
	if s := strings.TrimSpace(f.ProcesoID); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			campos["proceso_id"] = reglaInvalido
		} else {
			d.ProcesoID = &id
		}
	}
	if regla := parsearIDs(f.EspecialidadIDs, &d.EspecialidadIDs); regla != "" {
		campos["especialidad_ids"] = regla
	}
	if len(campos) > 0 {
		return DatosFuente{}, &ValidacionError{Campos: campos}
	}
	return d, nil
}

// parsearIDs parses a repeated id field: ≤ 50 ids > 0 without repeats.
func parsearIDs(valores []string, ids *[]int64) string {
	if len(valores) > maxEspecialidades {
		return reglaMax
	}
	vistos := make(map[int64]bool, len(valores))
	for _, v := range valores {
		id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil || id <= 0 {
			return reglaInvalido
		}
		if vistos[id] {
			return reglaRepetido
		}
		vistos[id] = true
		*ids = append(*ids, id)
	}
	return ""
}

// parsearFuenteID parses the fuente_id of S2.
func parsearFuenteID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || id <= 0 {
		return 0, &ValidacionError{Campos: map[string]string{"fuente_id": reglaRequerido}}
	}
	return id, nil
}
