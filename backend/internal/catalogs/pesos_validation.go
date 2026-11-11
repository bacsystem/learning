package catalogs

import (
	"errors"
	"fmt"
	"math"
)

// Limits of a weights configuration (contract PE3; E1-03 reglas 4–5).
const (
	maxAreasPesos         = 50
	maxPreguntasPorArea   = 200
	maxPreguntasConjunto  = 200
	maxPesoCentesimas     = 10000 // 100.00 %
	maxCoeficienteCentes  = 99999 // 999.99
	totalPesoCentesimas   = 10000
	toleranciaCentesimas  = 1 // ± 0.01, the same as pesoValido
	coeficienteEstimado   = 1.0
	reglaPesoRequerido    = "requerido"
	reglaPesoMin          = "min"
	reglaPesoMax          = "max"
	reglaPesoDecimales    = "decimales"
	campoAreas            = "areas"
	msgPesosValidacion    = "Datos de pesos inválidos."
	msgAreaRepetida       = "El área se repite en la configuración."
	msgLimitePreguntas    = "La suma de preguntas supera el máximo de 200."
	msgPesosNoSuman100    = "Los pesos deben sumar 100."
	msgFormatoNoConfigura = "El formato del examen no está configurado para esta especialidad."
)

// Coded errors of the weights endpoints (contract PE3, PE6).
var (
	ErrPesosValidacion      = errors.New(msgPesosValidacion)
	ErrLimitePreguntas      = errors.New(msgLimitePreguntas)
	ErrPesosNoSuman100      = errors.New(msgPesosNoSuman100)
	ErrFormatoNoConfigurado = errors.New(msgFormatoNoConfigura)
)

// errorPesos is a coded error with its own message and detalle; errors.Is
// matches its base sentinel.
type errorPesos struct {
	base    error
	msg     string
	detalle map[string]any
}

func (e *errorPesos) Error() string { return e.msg }
func (e *errorPesos) Unwrap() error { return e.base }

// centesimas converts a value already checked by hasAtMost2Decimals to an
// exact integer number of hundredths, so sums never drift (33.33 × 3).
func centesimas(f float64) int64 { return int64(math.Round(f * 100)) }

// validarAreasPesos checks the shape of PE3 (validation 1): 1–50 areas and,
// per row, the same rules as the existing rows. Every broken rule is listed.
func validarAreasPesos(areas []AreaPesoRequest) error {
	campos := map[string]string{}
	switch {
	case len(areas) == 0:
		campos[campoAreas] = reglaPesoMin
	case len(areas) > maxAreasPesos:
		campos[campoAreas] = reglaPesoMax
	}
	for i, a := range areas {
		campo := func(nombre string) string { return fmt.Sprintf("%s[%d].%s", campoAreas, i, nombre) }
		if a.AsignaturaID <= 0 {
			campos[campo("asignatura_id")] = reglaPesoRequerido
		}
		if regla := reglaRango(float64(a.NumeroPreguntas), 1, maxPreguntasPorArea); regla != "" {
			campos[campo("numero_preguntas")] = regla
		}
		if regla := reglaDecimal(a.PesoPorcentual, maxPesoCentesimas); regla != "" {
			campos[campo("peso_porcentual")] = regla
		}
		if regla := reglaDecimal(a.Coeficiente, maxCoeficienteCentes); regla != "" {
			campos[campo("coeficiente")] = regla
		}
	}
	if len(campos) == 0 {
		return nil
	}
	return &errorPesos{base: ErrPesosValidacion, msg: msgPesosValidacion, detalle: map[string]any{"campos": campos}}
}

// reglaDecimal: 0.01 ≤ f ≤ maxCentesimas/100 with at most two decimals.
func reglaDecimal(f float64, maxCentesimas int64) string {
	if !hasAtMost2Decimals(f) {
		return reglaPesoDecimales
	}
	return reglaRango(float64(centesimas(f)), 1, float64(maxCentesimas))
}

func reglaRango(v, min, max float64) string {
	switch {
	case v < min:
		return reglaPesoMin
	case v > max:
		return reglaPesoMax
	}
	return ""
}

// reglasDeConjunto are PE3 validations 3–5, in order, over a list that already
// passed validarAreasPesos: no repeated area (409), Σ preguntas ≤ 200 and
// Σ pesos = 100 ± 0.01.
func reglasDeConjunto(areas []AreaPesoRequest) error {
	vistas := make(map[int64]struct{}, len(areas))
	var preguntas int
	var pesos int64
	for _, a := range areas {
		if _, ok := vistas[a.AsignaturaID]; ok {
			return &errorPesos{base: ErrPesoDuplicado, msg: msgAreaRepetida,
				detalle: map[string]any{"asignatura_id": a.AsignaturaID}}
		}
		vistas[a.AsignaturaID] = struct{}{}
		preguntas += a.NumeroPreguntas
		pesos += centesimas(a.PesoPorcentual)
	}
	if preguntas > maxPreguntasConjunto {
		return &errorPesos{base: ErrLimitePreguntas, msg: msgLimitePreguntas,
			detalle: map[string]any{"suma_preguntas": preguntas}}
	}
	if diff := pesos - totalPesoCentesimas; diff < -toleranciaCentesimas || diff > toleranciaCentesimas {
		return &errorPesos{base: ErrPesosNoSuman100, msg: msgPesosNoSuman100,
			detalle: map[string]any{"suma_pesos": float64(pesos) / 100}}
	}
	return nil
}

// pesosIguales splits 100.00 % among the asignaturas (sorted by id): each gets
// floor(10000/n) hundredths and the leftover hundredths go one by one to the
// first ones, so the total is exactly 100.00 (contract PE6).
func pesosIguales(asignaturaIDs []int64) []PesoArea {
	n := int64(len(asignaturaIDs))
	areas := make([]PesoArea, 0, n)
	if n == 0 {
		return areas
	}
	base, resto := totalPesoCentesimas/n, totalPesoCentesimas%n
	for i, id := range asignaturaIDs {
		c := base
		if int64(i) < resto {
			c++
		}
		areas = append(areas, PesoArea{AsignaturaID: id, PesoPorcentual: float64(c) / 100, Coeficiente: coeficienteEstimado})
	}
	return areas
}
