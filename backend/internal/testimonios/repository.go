package testimonios

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"strings"
	"unicode"

	"github.com/cbaciliod/learning/internal/db/pii"
	"github.com/jmoiron/sqlx"
)

// Repository handles all DB access for testimonios.
type Repository struct {
	db  *sqlx.DB
	gcm cipher.AEAD
}

func NewRepository(db *sqlx.DB, gcm cipher.AEAD) *Repository {
	return &Repository{db: db, gcm: gcm}
}

func (r *Repository) decrypt(s, field string) string { return pii.Decrypt(r.gcm, s, field) }

type testimonioRow struct {
	ID             int64  `db:"id"`
	UsuarioID      int64  `db:"usuario_id"`
	Username       string `db:"username"`
	NombreCompleto string `db:"nombre_completo"`
	NombrePublico  string `db:"-"`
	Nombres        string `db:"nombres"`
	ApellidoP      string `db:"apellido_paterno"`
	Grado          string `db:"grado"`
	Calificacion   int    `db:"calificacion"`
	Contenido      string `db:"contenido"`
	Estado         string `db:"estado"`
	CreatedAt      string `db:"created_at"`
}

const publicJoin = `
	SELECT t.id, t.usuario_id, u.username,
	       p.nombres, p.apellido_paterno,
	       COALESCE(g.nombre, '') AS grado,
	       t.calificacion, t.contenido, t.estado,
	       TO_CHAR(t.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
	FROM testimonios t
	JOIN usuarios u ON u.id = t.usuario_id
	JOIN personas p ON p.id = u.persona_id
	LEFT JOIN grados g ON g.id = p.grado_id`

// buildNombreCompleto restores the pre-encryption display name
// "nombres apellido_paterno" (the PII-encryption refactor dropped the surname).
// Parts that cannot be decrypted are omitted, never shown as ciphertext.
func (r *Repository) buildNombreCompleto(row *testimonioRow) {
	row.NombreCompleto = strings.TrimSpace(
		r.decrypt(row.Nombres, "nombres") + " " + r.decrypt(row.ApellidoP, "apellido_paterno"))
}

// nombrePublico is the display name on the PUBLIC endpoint (H80): first given
// name plus the initial of the apellido paterno, e.g. "Juan P.". Without a
// decryptable given name nothing is shown.
func nombrePublico(nombres, paterno string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(nombres), " ")
	if first == "" {
		return ""
	}
	for _, c := range strings.TrimSpace(paterno) {
		return first + " " + string(unicode.ToUpper(c)) + "."
	}
	return first
}

// UpsertTestimonio inserts or updates a user's testimony, resetting estado to 'pendiente'.
func (r *Repository) UpsertTestimonio(ctx context.Context, usuarioID int64, contenido string, calificacion int) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO testimonios (usuario_id, contenido, calificacion)
		VALUES ($1, $2, $3)
		ON CONFLICT (usuario_id) DO UPDATE
		  SET contenido=$2, calificacion=$3, estado='pendiente', updated_at=NOW()
	`, usuarioID, contenido, calificacion)
	return err
}

// GetPublicos returns up to 20 approved testimonials ordered by newest first.
func (r *Repository) GetPublicos(ctx context.Context) ([]testimonioRow, error) {
	var rows []testimonioRow
	err := r.db.SelectContext(ctx, &rows, publicJoin+`
		WHERE t.estado = 'aprobado'
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT 20`)
	for i := range rows {
		rows[i].NombrePublico = nombrePublico(
			r.decrypt(rows[i].Nombres, "nombres"), r.decrypt(rows[i].ApellidoP, "apellido_paterno"))
	}
	return rows, err
}

// GetMiTestimonio returns the testimony for a given user, or sql.ErrNoRows if none.
func (r *Repository) GetMiTestimonio(ctx context.Context, usuarioID int64) (*testimonioRow, error) {
	var row testimonioRow
	err := r.db.GetContext(ctx, &row, `
		SELECT t.id, t.usuario_id, u.username,
		       p.nombres, p.apellido_paterno,
		       COALESCE(g.nombre, '') AS grado,
		       t.calificacion, t.contenido, t.estado,
		       TO_CHAR(t.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		FROM testimonios t
		JOIN usuarios u ON u.id = t.usuario_id
		JOIN personas p ON p.id = u.persona_id
		LEFT JOIN grados g ON g.id = p.grado_id
		WHERE t.usuario_id = $1`, usuarioID)
	if err != nil {
		return nil, err
	}
	r.buildNombreCompleto(&row)
	return &row, nil
}

// ListAdmin returns a paginated list, optionally filtered by estado.
func (r *Repository) ListAdmin(ctx context.Context, estado string, page, perPage int) ([]testimonioRow, int64, error) {
	offset := int64(page-1) * int64(perPage)
	var total int64
	var rows []testimonioRow

	if estado != "" {
		if err := r.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM testimonios WHERE estado=$1`, estado).Scan(&total); err != nil {
			return nil, 0, err
		}
		err := r.db.SelectContext(ctx, &rows, publicJoin+`
			WHERE t.estado = $1
			ORDER BY t.created_at DESC, t.id DESC
			LIMIT $2 OFFSET $3`, estado, perPage, offset)
		for i := range rows {
			r.buildNombreCompleto(&rows[i])
		}
		return rows, total, err
	}

	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM testimonios`).Scan(&total); err != nil {
		return nil, 0, err
	}
	err := r.db.SelectContext(ctx, &rows, publicJoin+`
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT $1 OFFSET $2`, perPage, offset)
	for i := range rows {
		r.buildNombreCompleto(&rows[i])
	}
	return rows, total, err
}

// UpdateEstado sets estado on a testimony. Returns sql.ErrNoRows if not found.
func (r *Repository) UpdateEstado(ctx context.Context, id int64, estado string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE testimonios SET estado=$1, updated_at=NOW() WHERE id=$2`, estado, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
