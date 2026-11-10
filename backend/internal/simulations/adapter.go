package simulations

import (
	"context"
	"errors"

	"github.com/cbaciliod/learning/internal/grading"
	"github.com/cbaciliod/learning/internal/questions"
)

// questionsAdapter bridges *questions.Service to the local QuestionsService interface.
type questionsAdapter struct {
	svc *questions.Service
}

// NewQuestionsAdapter wraps a *questions.Service so it satisfies QuestionsService.
func NewQuestionsAdapter(svc *questions.Service) QuestionsService {
	return &questionsAdapter{svc: svc}
}

func (a *questionsAdapter) GetProcesoInstitucionID(ctx context.Context, procesoID int64) (int64, error) {
	return a.svc.GetProcesoInstitucionID(ctx, procesoID)
}

func (a *questionsAdapter) GenerateSet(ctx context.Context, req generateSetRequest) ([]preguntaExamenItem, error) {
	qReq := questions.GenerateSetRequest{
		ProcesoID:      req.ProcesoID,
		GradoID:        req.GradoID,
		EspecialidadID: req.EspecialidadID,
	}
	for _, a := range req.Asignaturas {
		qReq.Asignaturas = append(qReq.Asignaturas, questions.AsignaturaSetItem{
			AsignaturaID:    a.AsignaturaID,
			NumeroPreguntas: a.NumeroPreguntas,
		})
	}

	resp, err := a.svc.GenerateSet(ctx, qReq)
	if err != nil {
		if errors.Is(err, questions.ErrInsufficientQuestions) {
			return nil, ErrInsufficientQuestions
		}
		return nil, err
	}

	items := make([]preguntaExamenItem, len(resp.Preguntas))
	for i, p := range resp.Preguntas {
		resps := make([]respuestaExamen, len(p.Respuestas))
		for j, r := range p.Respuestas {
			resps[j] = respuestaExamen{ID: r.ID, Texto: r.Texto, Orden: r.Orden}
		}
		items[i] = preguntaExamenItem{
			ID:           p.ID,
			Enunciado:    p.Enunciado,
			Tipo:         p.Tipo,
			Asignatura:   p.Asignatura,
			AsignaturaID: p.AsignaturaID,
			Respuestas:   resps,
			Orden:        p.Orden,
		}
	}
	return items, nil
}

func (a *questionsAdapter) RecordIntentos(ctx context.Context, intentos []grading.Intento) error {
	return a.svc.RecordIntentos(ctx, intentos)
}
