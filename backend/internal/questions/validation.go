package questions

import "fmt"

// MaxPreguntasSet caps the total number of questions one generate-set call can return.
const MaxPreguntasSet = 200

// ValidateRespuestas enforces business rules on answer options:
//   - 2 ≤ len(respuestas) ≤ 6
//   - tipo=unica: exactly 1 es_correcta=true
//   - tipo=multiple: at least 2 es_correcta=true
func ValidateRespuestas(tipo string, respuestas []CreateRespuestaItem) error {
	if len(respuestas) < 2 || len(respuestas) > 6 {
		return fmt.Errorf("%w: la pregunta debe tener entre 2 y 6 respuestas", ErrValidation)
	}

	correctas := 0
	for _, r := range respuestas {
		if r.EsCorrecta {
			correctas++
		}
	}

	switch tipo {
	case "unica":
		if correctas != 1 {
			return fmt.Errorf("%w: tipo 'unica' debe tener exactamente 1 respuesta correcta", ErrValidation)
		}
	case "multiple":
		if correctas < 2 {
			return fmt.Errorf("%w: tipo 'multiple' debe tener al menos 2 respuestas correctas", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: tipo debe ser 'unica' o 'multiple'", ErrValidation)
	}

	return nil
}
