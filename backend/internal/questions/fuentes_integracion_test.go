package questions

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/jmoiron/sqlx"
)

// suscriptorQueFalla is a second subscriber that fails after the real one ran.
type suscriptorQueFalla struct{}

func (suscriptorQueFalla) Nombre() string { return "falla" }
func (suscriptorQueFalla) FuenteRetirada(context.Context, sqlx.ExtContext, int64, int64) (int, error) {
	return 0, errors.New("provocado")
}

// retiroFx is a fuente cited by N preguntas in given states.
type retiroFx struct {
	*cicloFx
	db       *sqlx.DB
	repo     *fuentes.Repository
	fuenteID int64
	ids      map[string][]int64 // state → preguntas
	actor    int64
}

func newRetiroFx(t *testing.T, estados map[string]int) *retiroFx {
	t.Helper()
	c := newCicloFx(t)
	fuenteID, pasaje := fuenteDePrueba(t, c.f.DB)
	fx := &retiroFx{cicloFx: c, db: c.f.DB, repo: fuentes.NewRepository(c.f.DB), fuenteID: fuenteID, ids: map[string][]int64{}, actor: c.f.UsuarioID}
	// The retirement audits with the actor of the fixture: remove those rows before it is deleted.
	t.Cleanup(func() {
		if _, err := fx.db.Exec(`DELETE FROM audit_log WHERE (detalle->>'fuente_id')::bigint = $1`, fx.fuenteID); err != nil {
			t.Errorf("cleanup audit_log: %v", err)
		}
	})
	for estado, n := range estados {
		var motivo *string // the CHECK of 065 requires one when rechazada
		if estado == EstadoRechazada {
			m := "Clave incorrecta"
			motivo = &m
		}
		for i := 0; i < n; i++ {
			var id int64
			if err := c.f.DB.QueryRow(`INSERT INTO preguntas (institucion_id, proceso_id, asignatura_id, enunciado, estado, motivo_rechazo)
				VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, c.f.InstitucionID, c.f.ProcesoID, c.p.AsignaturaID,
				fmt.Sprintf("Retiro %s %d %s", estado, i, c.f.Suffix), estado, motivo).Scan(&id); err != nil {
				t.Fatal(err)
			}
			if _, err := c.f.DB.Exec(`INSERT INTO pregunta_citas (pregunta_id, pasaje_id) VALUES ($1, $2)`, id, pasaje); err != nil {
				t.Fatal(err)
			}
			fx.ids[estado] = append(fx.ids[estado], id)
		}
	}
	return fx
}

func (fx *retiroFx) estadoDe(t *testing.T, id int64) string {
	t.Helper()
	var e string
	if err := fx.db.QueryRow(`SELECT estado FROM preguntas WHERE id = $1`, id).Scan(&e); err != nil {
		t.Fatal(err)
	}
	return e
}

func (fx *retiroFx) retirar(extra ...fuentes.SuscriptorRetiro) (int, error) {
	return fx.repo.Retirar(context.Background(), fx.fuenteID, "Obsoleta", fx.actor,
		append([]fuentes.SuscriptorRetiro{SuscriptorRetiro{}}, extra...), nil)
}

// Escenario «Retirar una fuente citada»: 12 aprobadas, 3 en borrador, 2 activas heredadas.
func TestSuscriptorRetiro_EnviaARevision(t *testing.T) {
	fx := newRetiroFx(t, map[string]int{EstadoAprobada: 12, EstadoBorrador: 3, EstadoActivo: 2, EstadoRechazada: 1, EstadoEnRevision: 1})

	n, err := fx.retirar()
	if err != nil || n != 14 {
		t.Fatalf("marcadas = %d (%v), want 14", n, err)
	}
	for estado, want := range map[string]string{
		EstadoAprobada: EstadoEnRevision, EstadoActivo: EstadoEnRevision, EstadoBorrador: EstadoBorrador,
		EstadoRechazada: EstadoRechazada, EstadoEnRevision: EstadoEnRevision,
	} {
		for _, id := range fx.ids[estado] {
			if got := fx.estadoDe(t, id); got != want {
				t.Errorf("%s → %s, want %s", estado, got, want)
			}
		}
	}

	var filas int
	var anteriores []string
	rows, err := fx.db.Query(`SELECT detalle->>'estado_anterior' FROM audit_log
		WHERE accion = 'pregunta.estado' AND usuario_id = $1 AND detalle->>'motivo' = 'fuente retirada' AND (detalle->>'fuente_id')::bigint = $2`,
		fx.actor, fx.fuenteID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		anteriores = append(anteriores, a)
		filas++
	}
	count := map[string]int{}
	for _, a := range anteriores {
		count[a]++
	}
	if filas != 14 || count[EstadoAprobada] != 12 || count[EstadoActivo] != 2 {
		t.Errorf("auditoría: %d filas, por estado anterior %v", filas, count)
	}
}

// Escenario «Restaurar no reaprueba».
func TestSuscriptorRetiro_RestaurarNoReaprueba(t *testing.T) {
	fx := newRetiroFx(t, map[string]int{EstadoAprobada: 2})
	if _, err := fx.retirar(); err != nil {
		t.Fatal(err)
	}
	if err := fx.repo.Restaurar(context.Background(), fx.fuenteID, nil); err != nil {
		t.Fatal(err)
	}
	for _, id := range fx.ids[EstadoAprobada] {
		if got := fx.estadoDe(t, id); got != EstadoEnRevision {
			t.Errorf("tras restaurar: %s, want en_revision", got)
		}
	}
}

// Todo o nada: if a later subscriber fails, nothing of the retirement stays.
func TestSuscriptorRetiro_TodoONada(t *testing.T) {
	fx := newRetiroFx(t, map[string]int{EstadoAprobada: 3, EstadoActivo: 1})
	_, err := fx.retirar(suscriptorQueFalla{})
	var se *fuentes.SuscriptorError
	if !errors.As(err, &se) || se.Nombre != "falla" {
		t.Fatalf("err = %v, want SuscriptorError de falla", err)
	}
	for estado, ids := range fx.ids {
		for _, id := range ids {
			if got := fx.estadoDe(t, id); got != estado {
				t.Errorf("%s cambió a %s", estado, got)
			}
		}
	}
	var estadoFuente string
	if err := fx.db.QueryRow(`SELECT estado FROM fuentes WHERE id = $1`, fx.fuenteID).Scan(&estadoFuente); err != nil || estadoFuente != "activa" {
		t.Errorf("la fuente sigue activa: %s (%v)", estadoFuente, err)
	}
	var filas int
	if err := fx.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE (detalle->>'fuente_id')::bigint = $1`, fx.fuenteID).Scan(&filas); err != nil || filas != 0 {
		t.Errorf("sin auditoría huérfana: %d (%v)", filas, err)
	}
}

func TestCitantes(t *testing.T) {
	fx := newRetiroFx(t, map[string]int{EstadoAprobada: 2, EstadoActivo: 1, EstadoBorrador: 4, EstadoRechazada: 1})
	ctx := context.Background()
	n, err := Citantes{}.ContarPorFuente(ctx, fx.db, fx.fuenteID)
	if err != nil || n != 3 {
		t.Errorf("ContarPorFuente = %d (%v), want 3: solo aprobada y activo", n, err)
	}
	var version int64
	if err := fx.db.Get(&version, `SELECT id FROM fuente_versiones WHERE fuente_id = $1`, fx.fuenteID); err != nil {
		t.Fatal(err)
	}
	if citada, err := (Citantes{}).VersionCitada(ctx, fx.db, version); err != nil || !citada {
		t.Errorf("VersionCitada = %v (%v), want true", citada, err)
	}
	otra, _ := fuenteDePrueba(t, fx.db)
	var otraVersion int64
	if err := fx.db.Get(&otraVersion, `SELECT id FROM fuente_versiones WHERE fuente_id = $1`, otra); err != nil {
		t.Fatal(err)
	}
	if citada, err := (Citantes{}).VersionCitada(ctx, fx.db, otraVersion); err != nil || citada {
		t.Errorf("versión sin citas: %v (%v)", citada, err)
	}
	if n, err := (Citantes{}).ContarPorFuente(ctx, fx.db, otra); err != nil || n != 0 {
		t.Errorf("fuente sin citas: %d (%v)", n, err)
	}
}
