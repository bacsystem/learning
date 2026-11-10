package questions

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"

	"github.com/lib/pq"

	"github.com/cbaciliod/learning/internal/grading"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// TestUpsertEstadisticas_ConcurrentShuffled_NoDeadlock: attempts that share
// questions finish at the same time, each listing them in its own order. The
// GROUP BY's hash output order follows the input order when keys collide, so
// without an explicit ORDER BY the upserts lock pregunta_estadisticas rows in
// different orders, deadlock (40P01) and lose one attempt's stats (H134).
func TestUpsertEstadisticas_ConcurrentShuffled_NoDeadlock(t *testing.T) {
	db := qt.DB(t)
	f := qt.Seed(t, db, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 120, Peso: 100, Unicas: 120}}})
	repo := NewRepository(db)
	base := make([]grading.Intento, len(f.Preguntas))
	for i, p := range f.Preguntas {
		base[i] = grading.Intento{PreguntaID: p.ID, Correcta: i%2 == 0}
	}
	rng := rand.New(rand.NewSource(134))
	shuffled := func() []grading.Intento {
		s := append([]grading.Intento(nil), base...)
		rng.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
		return s[:60+rng.Intn(61)]
	}
	const rounds, workers = 150, 8
	want := 0
	for r := 0; r < rounds; r++ {
		var wg sync.WaitGroup
		errs := make([]error, workers)
		sets := make([][]grading.Intento, workers)
		for g := range sets {
			sets[g] = shuffled()
			want += len(sets[g])
		}
		for g, set := range sets {
			wg.Add(1)
			go func(g int, set []grading.Intento) {
				defer wg.Done()
				errs[g] = repo.UpsertEstadisticas(context.Background(), set)
			}(g, set)
		}
		wg.Wait()
		for _, err := range errs {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "40P01" {
				t.Fatalf("round %d: deadlock: %v", r, err)
			}
			if err != nil {
				t.Fatalf("round %d: %v", r, err)
			}
		}
	}
	var total int
	if err := db.QueryRow(`SELECT COALESCE(SUM(total_intentos),0) FROM pregunta_estadisticas WHERE pregunta_id = ANY($1)`,
		pq.Array(preguntaIDs(f))).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != want {
		t.Fatalf("total_intentos = %d, want %d", total, want)
	}
}

func preguntaIDs(f *qt.Fixture) []int64 {
	ids := make([]int64, len(f.Preguntas))
	for i, p := range f.Preguntas {
		ids[i] = p.ID
	}
	return ids
}
