package balotario

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cbaciliod/learning/internal/fuentes"
)

// Operaciones are the steps of the confirmation inside its transaction
// (contract §4.1). The repository implements them over one *sqlx.Tx.
type Operaciones interface {
	BloquearFuente(ctx context.Context, fuenteID int64) error
	VersionVigente(ctx context.Context, fuenteID int64) (fuentes.VersionBalotario, error)
	Rangos(ctx context.Context, versionID int64) ([]fuentes.RangoPasaje, error)
	ImportacionDeVersion(ctx context.Context, versionID int64) (*registro, error)
	InstitucionDelProceso(ctx context.Context, procesoID int64) (int64, bool, error)
	EspecialidadActiva(ctx context.Context, especialidadID, institucionID int64) (bool, error)
	AreaAsignada(ctx context.Context, procesoID, asignaturaID int64) (bool, error)
	Guardar(ctx context.Context, g Guardado, aud Auditor) (int64, error)
}

// Confirmar imports the current version of a balotario as drafts (contract
// §4): everything happens in one transaction with the fuente locked FOR SHARE,
// so a failure leaves nothing and a concurrent confirmation of the same
// version loses with ya_importado.
func (s *Service) Confirmar(ctx context.Context, fuenteID int64, req ConfirmarRequest, actorID int64, aud Auditor) (ConfirmarResponse, error) {
	if err := validarSolicitud(req); err != nil {
		return ConfirmarResponse{}, err
	}
	var resp ConfirmarResponse
	err := s.datos.EnTransaccion(ctx, func(ops Operaciones) error {
		var err error
		resp, err = confirmar(ctx, ops, fuenteID, req, actorID, aud)
		return err
	})
	if errors.Is(err, ErrYaImportadoConcurrente) {
		return ConfirmarResponse{}, s.yaImportado(ctx, fuenteID, req.Version)
	}
	if err != nil {
		return ConfirmarResponse{}, traducirLimite(err)
	}
	return resp, nil
}

// validarSolicitud is step 2: the four fields are present and positive.
func validarSolicitud(req ConfirmarRequest) error {
	campos := map[string]string{}
	if req.Version < 1 {
		campos["version"] = "debe ser un entero mayor que 0"
	}
	if req.ProcesoID < 1 {
		campos["proceso_id"] = "debe ser un entero mayor que 0"
	}
	if req.EspecialidadID < 1 {
		campos["especialidad_id"] = "debe ser un entero mayor que 0"
	}
	if req.AsignaturaID < 1 {
		campos["asignatura_id"] = "debe ser un entero mayor que 0"
	}
	if len(campos) > 0 {
		return regla(CodigoValidacion, map[string]any{"campos": campos})
	}
	return nil
}

// confirmar is steps 3–13 of §4.1.
func confirmar(ctx context.Context, ops Operaciones, fuenteID int64, req ConfirmarRequest, actorID int64, aud Auditor) (ConfirmarResponse, error) {
	if err := ops.BloquearFuente(ctx, fuenteID); err != nil { // steps 3 and 4
		return ConfirmarResponse{}, err
	}
	v, err := ops.VersionVigente(ctx, fuenteID)
	if err != nil {
		return ConfirmarResponse{}, err
	}
	if err := comprobarImportable(v); err != nil { // step 5
		return ConfirmarResponse{}, err
	}
	if req.Version != v.Numero { // step 6
		return ConfirmarResponse{}, regla(CodigoVersionDesactualizada, map[string]any{"version_vigente": v.Numero})
	}
	if previa, err := ops.ImportacionDeVersion(ctx, v.VersionID); err != nil { // step 7
		return ConfirmarResponse{}, err
	} else if previa != nil {
		return ConfirmarResponse{}, reglaYaImportado(previa)
	}
	institucionID, err := comprobarReferencias(ctx, ops, v, req) // steps 8–10
	if err != nil {
		return ConfirmarResponse{}, err
	}
	rangos, err := ops.Rangos(ctx, v.VersionID)
	if err != nil {
		return ConfirmarResponse{}, err
	}
	a, err := analizar(v.Contenido, rangos) // step 11
	if err != nil {
		return ConfirmarResponse{}, err
	}
	if len(a.Validas) == 0 { // step 12
		return ConfirmarResponse{}, regla(CodigoSinValidas, nil)
	}
	t := a.totales()
	importacionID, err := ops.Guardar(ctx, Guardado{
		FuenteID: fuenteID, FuenteVersionID: v.VersionID, Version: v.Numero, InstitucionID: institucionID, Req: req, ActorID: actorID,
		Validas: a.Validas, ConProblemas: len(a.Problemas), Detalle: detalleAuditoria(fuenteID, v, req, t),
	}, aud)
	if err != nil { // step 13
		return ConfirmarResponse{}, err
	}
	return ConfirmarResponse{ImportacionID: importacionID, FuenteID: fuenteID, Version: v.Numero, Creadas: t.Validas,
		ConProblemas: t.ConProblemas, Unica: t.Unica, Multiple: t.Multiple, ConAlertas: t.ConAlertas}, nil
}

// comprobarReferencias is steps 8a, 8b, 9 and 10; it returns the institution of the proceso.
func comprobarReferencias(ctx context.Context, ops Operaciones, v fuentes.VersionBalotario, req ConfirmarRequest) (int64, error) {
	institucionID, existe, err := ops.InstitucionDelProceso(ctx, req.ProcesoID)
	if err != nil {
		return 0, err
	}
	if !existe {
		return 0, referenciaInvalida("proceso_id")
	}
	activa, err := ops.EspecialidadActiva(ctx, req.EspecialidadID, institucionID)
	if err != nil {
		return 0, err
	}
	if !activa {
		return 0, regla(CodigoEspecialidadInvalida, nil)
	}
	if len(v.EspecialidadIDs) > 0 && !slices.Contains(v.EspecialidadIDs, req.EspecialidadID) {
		return 0, regla(CodigoEspecialidadNoCorresponde, map[string]any{"especialidad_ids": v.EspecialidadIDs})
	}
	asignada, err := ops.AreaAsignada(ctx, req.ProcesoID, req.AsignaturaID)
	if err != nil {
		return 0, err
	}
	if !asignada {
		return 0, referenciaInvalida("asignatura_id")
	}
	return institucionID, nil
}

func referenciaInvalida(campo string) *ReglaError {
	return regla(CodigoReferenciaInvalida, map[string]any{"campos": map[string]string{campo: "no existe o no corresponde"}})
}

func reglaYaImportado(reg *registro) *ReglaError {
	return regla(CodigoYaImportado, map[string]any{"importacion_id": reg.ID, "created_at": reg.CreatedAt})
}

// yaImportado answers the loser of a concurrent confirmation, with the detail
// of the winner (read after the failed transaction).
func (s *Service) yaImportado(ctx context.Context, fuenteID int64, version int) error {
	l, err := s.datos.Leer(ctx, fuenteID)
	if err != nil {
		return fmt.Errorf("leer la importación ganadora: %w", err)
	}
	for _, reg := range l.Importaciones {
		if reg.Version == version {
			return reglaYaImportado(&reg)
		}
	}
	return regla(CodigoYaImportado, nil)
}

// detalleAuditoria is the audit detail of §4.4: ids and totals, never text.
func detalleAuditoria(fuenteID int64, v fuentes.VersionBalotario, req ConfirmarRequest, t Totales) map[string]any {
	return map[string]any{
		"version": v.Numero, "fuente_version_id": v.VersionID, "proceso_id": req.ProcesoID,
		"especialidad_id": req.EspecialidadID, "asignatura_id": req.AsignaturaID,
		"creadas": t.Validas, "con_problemas": t.ConProblemas, "unica": t.Unica, "multiple": t.Multiple, "con_alertas": t.ConAlertas,
	}
}
