package fuentes

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// Citantes reads the citations of fuentes, which SIA-E3-02, E6-06 and E6-08
// register when they create them (contract E2 § Puntos de extensión). It is
// used to count citing contents and to archive versions nobody cites.
type Citantes interface {
	ContarPorFuente(ctx context.Context, q sqlx.QueryerContext, fuenteID int64) (int, error)
	VersionCitada(ctx context.Context, q sqlx.QueryerContext, versionID int64) (bool, error)
}

// SuscriptorRetiro receives the «fuente retirada» notice (SIA-E2-06 regla 3):
// once per subscriber, inside the retirement's transaction, with the actor.
// An error rolls the whole retirement back. It returns how many contents it
// marked (contenidos_marcados). SIA-E3-02, E6-06 and E6-08 register one.
type SuscriptorRetiro interface {
	Nombre() string // for the logs
	FuenteRetirada(ctx context.Context, tx sqlx.ExtContext, fuenteID, actorID int64) (afectados int, err error)
}

// sinCitantes is the MVP Citantes: nothing cites a fuente yet.
type sinCitantes struct{}

func (sinCitantes) ContarPorFuente(context.Context, sqlx.QueryerContext, int64) (int, error) {
	return 0, nil
}

func (sinCitantes) VersionCitada(context.Context, sqlx.QueryerContext, int64) (bool, error) {
	return false, nil
}
