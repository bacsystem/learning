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
	minMotivo         = 5 // retirar and desmarcar (contract S5, S6)
	maxMotivo         = 500
	minEntidad        = 3 // oficial mark (contract S6)
	maxEntidad        = 100
	minNota           = 10
	maxNota           = 500
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
	d := DatosFuente{Tipo: strings.TrimSpace(f.Tipo), EspecialidadIDs: []int64{}}
	var regla string
	if d.Titulo, regla = longitud(f.Titulo, minTitulo, maxTitulo); regla != "" {
		campos["titulo"] = regla
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

// parsearIDs parses a repeated form field of ids with the rule of validarIDs.
func parsearIDs(valores []string, ids *[]int64) string {
	if len(valores) > maxEspecialidades {
		return reglaMax
	}
	parseados := make([]int64, 0, len(valores))
	for _, v := range valores {
		id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return reglaInvalido
		}
		parseados = append(parseados, id)
	}
	if regla := validarIDs(parseados); regla != "" {
		return regla
	}
	*ids = append(*ids, parseados...)
	return ""
}

// validarIDs: ≤ 50 especialidad ids, each > 0, without repeats.
func validarIDs(ids []int64) string {
	if len(ids) > maxEspecialidades {
		return reglaMax
	}
	vistos := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return reglaInvalido
		}
		if vistos[id] {
			return reglaRepetido
		}
		vistos[id] = true
	}
	return ""
}

// longitud trims s (README §6) and returns it with the rule its length in
// runes breaks: requerido when empty, min or max; "" when it fits.
func longitud(s string, min, max int) (string, string) {
	s = strings.TrimSpace(s)
	switch n := utf8.RuneCountInString(s); {
	case n == 0:
		return s, reglaRequerido
	case n < min:
		return s, reglaMin
	case n > max:
		return s, reglaMax
	}
	return s, ""
}

// opcional is longitud over an optional field; nil is requerido.
func opcional(s *string, min, max int) (string, string) {
	if s == nil {
		return "", reglaRequerido
	}
	return longitud(*s, min, max)
}

// errorDeCampos is nil without broken fields, a ValidacionError otherwise.
func errorDeCampos(campos map[string]string) error {
	if len(campos) == 0 {
		return nil
	}
	return &ValidacionError{Campos: campos}
}

// ── S5–S7 ─────────────────────────────────────────────────────────────────────

// transicion is a validated S5 change: the target state and, to retire, the
// trimmed motivo.
type transicion struct {
	Destino string
	Motivo  string
}

// destinosS5 are the target states of S5: retirar, restaurar, reintentar.
var destinosS5 = map[string]bool{EstadoRetirada: true, EstadoActiva: true, EstadoProcesando: true}

// validarCambioEstado applies the shape rules of S5; a motivo for another
// target is ignored.
func validarCambioEstado(req CambioEstadoRequest) (transicion, error) {
	t := transicion{Destino: req.Estado}
	campos := map[string]string{}
	switch {
	case t.Destino == "":
		campos["estado"] = reglaRequerido
	case !destinosS5[t.Destino]:
		campos["estado"] = reglaInvalido
	case t.Destino == EstadoRetirada:
		var regla string
		if t.Motivo, regla = opcional(req.Motivo, minMotivo, maxMotivo); regla != "" {
			campos["motivo"] = regla
		}
	}
	return t, errorDeCampos(campos)
}

// marca is a validated S6 change; the fields of the other direction are ignored.
type marca struct {
	Marcar         bool
	EntidadEmisora string
	Nota           string
	Motivo         string
}

// validarOficial applies the shape rules of S6 (contract S6, README §6).
func validarOficial(req OficialRequest) (marca, error) {
	campos := map[string]string{}
	if req.OficialVerificada == nil {
		campos["oficial_verificada"] = reglaRequerido
		return marca{}, errorDeCampos(campos)
	}
	m := marca{Marcar: *req.OficialVerificada}
	var regla string
	if m.Marcar {
		if m.EntidadEmisora, regla = opcional(req.EntidadEmisora, minEntidad, maxEntidad); regla != "" {
			campos["entidad_emisora"] = regla
		}
		if m.Nota, regla = opcional(req.Nota, minNota, maxNota); regla != "" {
			campos["nota"] = regla
		}
	} else if m.Motivo, regla = opcional(req.Motivo, minMotivo, maxMotivo); regla != "" {
		campos["motivo"] = regla
	}
	return m, errorDeCampos(campos)
}

// edicion is a validated S7 change.
type edicion struct {
	Titulo          string
	EspecialidadIDs []int64
}

// validarEdicion applies rule 1 of S7: titulo 3–150 after trimming and
// especialidad_ids present, ≤ 50, > 0, without repeats (empty = all).
func validarEdicion(req EditarRequest) (edicion, error) {
	campos := map[string]string{}
	var d edicion
	var regla string
	if d.Titulo, regla = longitud(req.Titulo, minTitulo, maxTitulo); regla != "" {
		campos["titulo"] = regla
	}
	if req.EspecialidadIDs == nil {
		campos["especialidad_ids"] = reglaRequerido
	} else if regla = validarIDs(*req.EspecialidadIDs); regla != "" {
		campos["especialidad_ids"] = regla
	} else {
		d.EspecialidadIDs = *req.EspecialidadIDs
	}
	return d, errorDeCampos(campos)
}

// parsearFuenteID parses the fuente_id of S2 (architect, README §4): a
// non-numeric value is syntax (ErrCuerpoInvalido, 400); absent or ≤ 0 is
// validation (422, campos.fuente_id).
func parsearFuenteID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, &ValidacionError{Campos: map[string]string{"fuente_id": reglaRequerido}}
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, ErrCuerpoInvalido
	}
	if id <= 0 {
		return 0, &ValidacionError{Campos: map[string]string{"fuente_id": reglaInvalido}}
	}
	return id, nil
}
