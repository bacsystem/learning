package temas

import (
	"strings"
	"unicode/utf8"

	"github.com/cbaciliod/learning/internal/texto"
)

// Business limits (SIA-E1-01 reglas 2, 6; docs/api/README.md §6).
const (
	MaxTemasActivos     = 60
	minNombre           = 3
	maxNombre           = 120
	maxDescripcion      = 500
	maxEspecialidadIDs  = 50
	campoNombre         = "nombre"
	campoDescripcion    = "descripcion"
	campoEspecialidades = "especialidad_ids"
	campoAsignatura     = "asignatura_id"
	campoEstado         = "estado"
)

// normalizarNombre trims the name and collapses its inner spaces.
func normalizarNombre(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// validarDatos normalises and validates the editable fields; every broken rule
// is reported in campos. An empty descripcion is stored as NULL and a missing
// especialidad list means «aplica a todas».
func validarDatos(nombre string, descripcion *string, especialidadIDs []int64, campos map[string]string) DatosTema {
	d := DatosTema{Nombre: normalizarNombre(nombre)}
	switch n := utf8.RuneCountInString(d.Nombre); {
	case n == 0:
		campos[campoNombre] = reglaRequerido
	case n < minNombre:
		campos[campoNombre] = reglaMin
	case n > maxNombre:
		campos[campoNombre] = reglaMax
	}
	d.NombreClave = texto.Plegar(d.Nombre)

	if descripcion != nil {
		if s := strings.TrimSpace(*descripcion); s != "" {
			if utf8.RuneCountInString(s) > maxDescripcion {
				campos[campoDescripcion] = reglaMax
			}
			d.Descripcion = &s
		}
	}

	if regla := validarIDs(especialidadIDs, maxEspecialidadIDs); regla != "" {
		campos[campoEspecialidades] = regla
	}
	d.EspecialidadIDs = especialidadIDs
	if d.EspecialidadIDs == nil {
		d.EspecialidadIDs = []int64{}
	}
	return d
}

// validarIDs returns the broken rule of an id list, or "": at most max
// elements, every id > 0 and no repeats.
func validarIDs(ids []int64, max int) string {
	if len(ids) > max {
		return reglaMax
	}
	vistos := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return reglaInvalido
		}
		if _, ok := vistos[id]; ok {
			return reglaRepetido
		}
		vistos[id] = struct{}{}
	}
	return ""
}

func validarCrear(req CrearTemaRequest) (DatosTema, error) {
	campos := map[string]string{}
	if req.AsignaturaID <= 0 {
		campos[campoAsignatura] = reglaRequerido
	}
	d := validarDatos(req.Nombre, req.Descripcion, req.EspecialidadIDs, campos)
	return d, errorDeCampos(campos)
}

func validarEditar(req EditarTemaRequest) (DatosTema, error) {
	campos := map[string]string{}
	d := validarDatos(req.Nombre, req.Descripcion, req.EspecialidadIDs, campos)
	return d, errorDeCampos(campos)
}

func validarEstado(estado string) error {
	if estado != EstadoActivo && estado != EstadoArchivado {
		return &ValidacionError{Campos: map[string]string{campoEstado: reglaInvalido}}
	}
	return nil
}

// validarOrden checks the shape of a reorder list (escenario «Lista de orden
// inválida»); whether it matches the active temas is checked under the lock.
func validarOrden(ids []int64) error {
	if validarIDs(ids, MaxTemasActivos) != "" {
		return ErrOrdenInvalido
	}
	return nil
}

func errorDeCampos(campos map[string]string) error {
	if len(campos) == 0 {
		return nil
	}
	return &ValidacionError{Campos: campos}
}
