package balotario

import (
	"context"
	"errors"

	"github.com/cbaciliod/learning/internal/fuentes"
)

// Lector is what the preview needs from the data layer.
type Lector interface {
	Leer(ctx context.Context, fuenteID int64) (lectura, error)
}

// Service is the business logic of the balotario import: it knows neither HTTP
// nor SQL.
type Service struct {
	lector Lector
}

func NewService(lector Lector) *Service { return &Service{lector: lector} }

// VistaPrevia parses the current version of a balotario without saving
// anything (contract §3). page and perPage paginate only the problems.
func (s *Service) VistaPrevia(ctx context.Context, fuenteID int64, page, perPage int) (VistaPreviaResponse, error) {
	l, err := s.lector.Leer(ctx, fuenteID)
	if errors.Is(err, fuentes.ErrNoEncontrado) {
		return VistaPreviaResponse{}, ErrNoEncontrado
	}
	if err != nil {
		return VistaPreviaResponse{}, err
	}
	if err := comprobarImportable(l.Version); err != nil {
		return VistaPreviaResponse{}, err
	}
	a, err := analizar(l.Version.Contenido, l.Rangos)
	if err != nil {
		return VistaPreviaResponse{}, traducirLimite(err)
	}
	return armarVistaPrevia(l, a, page, perPage), nil
}

// comprobarImportable is steps 3 and 4 of §3 and 5 of §4.1: only an active
// balotario with a current version is imported.
func comprobarImportable(v fuentes.VersionBalotario) error {
	if v.Tipo != "balotario" {
		return regla(CodigoTipoFuenteInvalido, nil)
	}
	if v.Estado != "activa" || v.VersionID == 0 {
		return regla(CodigoFuenteNoActiva, map[string]any{"estado": v.Estado})
	}
	return nil
}

// traducirLimite turns the parser's limit into its rule with the detail.
func traducirLimite(err error) error {
	var limite *LimiteError
	if errors.As(err, &limite) {
		return regla(CodigoLimitePreguntas, map[string]any{"maximo": MaxPreguntas, "detectadas": limite.Detectadas})
	}
	return err
}

func armarVistaPrevia(l lectura, a analisis, page, perPage int) VistaPreviaResponse {
	v := l.Version
	resp := VistaPreviaResponse{
		Fuente: FuenteVista{ID: v.FuenteID, Titulo: v.Titulo, Version: v.Numero, OficialVerificada: v.OficialVerificada,
			OficialEntidad: v.OficialEntidad},
		RutaRevision: RutaVerificacionClave,
		Preguntas:    primerasPreguntas(a.Validas),
		Problemas:    paginarProblemas(a.Problemas, page, perPage),
	}
	if v.OficialVerificada {
		resp.RutaRevision = RutaEstructural
	}
	t := a.totales()
	resp.Totales = TotalesVista{Validas: t.Validas, ConProblemas: t.ConProblemas, Unica: t.Unica, Multiple: t.Multiple, ConAlertas: t.ConAlertas}

	resp.OtrasImportaciones = []OtraImportacion{}
	for _, reg := range l.Importaciones {
		if reg.Version == v.Numero {
			resp.Importacion = &ImportacionVista{ID: reg.ID, Creadas: reg.Creadas, CreatedAt: reg.CreatedAt}
			continue
		}
		resp.OtrasImportaciones = append(resp.OtrasImportaciones, OtraImportacion{Version: reg.Version, Creadas: reg.Creadas, CreatedAt: reg.CreatedAt})
	}
	resp.PuedeConfirmar, resp.MotivoNoConfirmable = puedeConfirmar(resp.Importacion != nil, len(a.Validas))
	return resp
}

// puedeConfirmar: false with sin_validas or ya_importado (the second wins when
// both apply: the version is already imported whatever it holds).
func puedeConfirmar(yaImportada bool, validas int) (bool, *string) {
	motivo := func(m string) (bool, *string) { return false, &m }
	switch {
	case yaImportada:
		return motivo(NoConfirmableYaImportado)
	case validas == 0:
		return motivo(NoConfirmableSinValidas)
	}
	return true, nil
}

func primerasPreguntas(validas []importable) []PreguntaVista {
	n := min(len(validas), PreguntasVistaPrevia)
	out := make([]PreguntaVista, 0, n)
	for _, v := range validas[:n] {
		alts := make([]AlternativaVista, len(v.Alternativas))
		for i, a := range v.Alternativas {
			alts[i] = AlternativaVista{Letra: a.Letra, Texto: a.Texto, Correcta: a.Correcta}
		}
		out = append(out, PreguntaVista{Numero: v.Numero, LineaInicio: v.LineaInicio, LineaFin: v.LineaFin, Tipo: v.Tipo,
			Enunciado: v.Enunciado, Alternativas: alts, AlertasEstructura: v.Alertas})
	}
	return out
}

// paginarProblemas pages the problems (D11); a page past the end is empty.
func paginarProblemas(problemas []Problema, page, perPage int) PaginaProblemas {
	out := PaginaProblemas{Data: []ProblemaVista{}, Total: len(problemas), Page: page, PerPage: perPage}
	desde := (page - 1) * perPage
	if desde < 0 || desde >= len(problemas) {
		return out
	}
	for _, p := range problemas[desde:min(desde+perPage, len(problemas))] {
		out.Data = append(out.Data, ProblemaVista{Numero: p.Numero, LineaInicio: p.LineaInicio, LineaFin: p.LineaFin, Motivos: p.Motivos})
	}
	return out
}
