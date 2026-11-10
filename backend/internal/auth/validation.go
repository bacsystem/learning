package auth

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// validationMessages is the generic Spanish message returned (422) when a
// field fails validation, keyed by the JSON field name. It never includes
// the raw validator text, which is English and exposes struct names.
var validationMessages = map[string]string{
	"dni":              "El DNI debe tener exactamente 8 dígitos.",
	"email":            "Ingresa un correo electrónico válido.",
	"username":         "El usuario es obligatorio: de 4 a 30 caracteres, solo letras y números.",
	"password":         "La contraseña es obligatoria: al menos 8 caracteres y como máximo 72 bytes.",
	"confirm_password": "Las contraseñas no coinciden.",
	"token":            "El enlace no es válido.",
	"nombres":          "Los nombres son obligatorios (2 a 100 caracteres).",
	"apellido_paterno": "El apellido paterno es obligatorio (2 a 100 caracteres).",
	"apellido_materno": "El apellido materno no es válido (hasta 100 caracteres).",
	"telefono":         "El teléfono no es válido.",
	"grado_id":         "El grado es obligatorio.",
	"especialidad_id":  "La especialidad es obligatoria.",
	"institucion_id":   "La institución es obligatoria.",
	"estado":           "El estado no es válido.",
	"observacion":      "La observación admite hasta 500 caracteres.",
	"personas":         "La lista de personas es obligatoria.",
}

// msgValidationGeneric is used for fields without a specific message.
const msgValidationGeneric = "Datos inválidos. Revisa los campos del formulario."

// validationMessage maps a validator error to its Spanish message, using the
// first failing field (validator field names are the JSON tag names, see
// newValidator).
func validationMessage(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) && len(ve) > 0 {
		if msg, ok := validationMessages[ve[0].Field()]; ok {
			return msg
		}
	}
	return msgValidationGeneric
}

// jsonValidationError writes the 422 response for a validator error.
func jsonValidationError(w http.ResponseWriter, err error) {
	jsonError(w, http.StatusUnprocessableEntity, validationMessage(err))
}
