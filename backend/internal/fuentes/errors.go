package fuentes

import (
	"errors"
	"sort"
	"strings"
)

// Errors of the admin upload (S1, S2; contract E2). The handler maps them to
// status and code; the texts are the Spanish fallback message.
var (
	ErrArchivoDemasiadoGrande = errors.New("El archivo supera el máximo de 2 MiB.")
	ErrArchivoInvalido        = errors.New("El archivo no es un .md de texto UTF-8 válido.")
	ErrValidacion             = errors.New("Datos de la fuente inválidos.")
	ErrReferenciaInvalida     = errors.New("Alguna especialidad o el proceso no existe o está inactivo.")
	ErrLimiteFuentes          = errors.New("Se alcanzó el máximo de fuentes.")
	ErrEstadoInvalido         = errors.New("La fuente no admite una versión nueva en su estado actual.")
	ErrLimiteVersiones        = errors.New("La fuente ya tiene 20 versiones y ninguna se puede archivar.")
)

// Motivos of archivo_invalido (detalle.motivo; contract S1 steps 3–6).
const (
	MotivoExtension    = "extension"
	MotivoTipo         = "tipo"
	MotivoCodificacion = "codificacion"
	MotivoLineas       = "lineas"
)

// ArchivoInvalidoError says which file rule failed; it matches ErrArchivoInvalido.
type ArchivoInvalidoError struct {
	Motivo string
}

func (e *ArchivoInvalidoError) Error() string {
	return ErrArchivoInvalido.Error() + " (" + e.Motivo + ")"
}
func (e *ArchivoInvalidoError) Unwrap() error { return ErrArchivoInvalido }

// ValidacionError lists the broken metadata fields (field → rule).
type ValidacionError struct {
	Campos map[string]string
}

func (e *ValidacionError) Error() string {
	nombres := make([]string, 0, len(e.Campos))
	for c := range e.Campos {
		nombres = append(nombres, c)
	}
	sort.Strings(nombres)
	return ErrValidacion.Error() + " Campos: " + strings.Join(nombres, ", ")
}

func (e *ValidacionError) Unwrap() error { return ErrValidacion }

// DuplicadaError is ErrFuenteDuplicada with the fuente that already holds the
// hash, which the UI links to (contract S1 step 8).
type DuplicadaError struct {
	FuenteID int64
	Titulo   string
	Estado   string
	Version  int
}

func (e *DuplicadaError) Error() string { return ErrFuenteDuplicada.Error() }
func (e *DuplicadaError) Unwrap() error { return ErrFuenteDuplicada }

// LimiteError is ErrLimiteFuentes with the limit that was reached: per
// profession (EspecialidadID set) or the total.
type LimiteError struct {
	Limite         int
	EspecialidadID *int64
}

func (e *LimiteError) Error() string { return ErrLimiteFuentes.Error() }
func (e *LimiteError) Unwrap() error { return ErrLimiteFuentes }
