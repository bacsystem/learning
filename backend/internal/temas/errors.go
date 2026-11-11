package temas

import (
	"errors"
	"sort"
	"strings"
)

// Domain errors. The handler translates them into HTTP status and the stable
// codes of docs/api/serums-ia-e1-taxonomia.md §1; the texts are the Spanish
// fallback message.
var (
	ErrNoEncontrado        = errors.New("Tema no encontrado.")
	ErrValidacion          = errors.New("Datos inválidos.")
	ErrReferenciaInvalida  = errors.New("El área o alguna especialidad no existe o está inactiva.")
	ErrTemaDuplicado       = errors.New("Ya existe un tema con ese nombre en esta área.")
	ErrLimiteTemas         = errors.New("El área ya tiene el máximo de temas activos.")
	ErrEstadoInvalido      = errors.New("El tema ya está en ese estado.")
	ErrOrdenInvalido       = errors.New("La lista de orden es inválida.")
	ErrOrdenDesactualizado = errors.New("El orden está desactualizado; recarga la lista.")
)

// Rule names used in ValidacionError.Campos (detalle.campos of the 422).
const (
	reglaRequerido = "requerido"
	reglaMin       = "min"
	reglaMax       = "max"
	reglaInvalido  = "invalido"
	reglaRepetido  = "repetido"
)

// ValidacionError lists the fields that broke a rule (field → rule). It
// matches ErrValidacion with errors.Is.
type ValidacionError struct {
	Campos map[string]string
}

func (e *ValidacionError) Error() string {
	nombres := make([]string, 0, len(e.Campos))
	for campo := range e.Campos {
		nombres = append(nombres, campo)
	}
	sort.Strings(nombres)
	return ErrValidacion.Error() + " Campos: " + strings.Join(nombres, ", ")
}

func (e *ValidacionError) Unwrap() error { return ErrValidacion }
