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

// sinCitantes is the MVP Citantes: nothing cites a fuente yet.
type sinCitantes struct{}

func (sinCitantes) ContarPorFuente(context.Context, sqlx.QueryerContext, int64) (int, error) {
	return 0, nil
}

func (sinCitantes) VersionCitada(context.Context, sqlx.QueryerContext, int64) (bool, error) {
	return false, nil
}
