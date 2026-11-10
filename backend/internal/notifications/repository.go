package notifications

import (
	"context"
	"errors"

	"github.com/cbaciliod/learning/internal/db/pgerr"

	"github.com/jmoiron/sqlx"
)

// ErrNotFound: the notification does not exist (or is not addressed to the user).
var ErrNotFound = errors.New("Notificación no encontrada.")

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new notification.
func (r *Repository) Create(ctx context.Context, req CreateRequest, adminID int64) (Notificacion, error) {
	var n Notificacion
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO notificaciones (titulo, cuerpo, tipo, destinatario, creado_por)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, titulo, cuerpo, tipo, destinatario, creado_por, created_at`,
		req.Titulo, req.Cuerpo, req.Tipo, req.Destinatario, adminID,
	).Scan(&n.ID, &n.Titulo, &n.Cuerpo, &n.Tipo, &n.Destinatario, &n.CreadoPor, &n.CreatedAt)
	return n, err
}

// ListAll returns all notifications for the admin panel (newest first).
func (r *Repository) ListAll(ctx context.Context) ([]Notificacion, error) {
	var rows []Notificacion
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, titulo, cuerpo, tipo, destinatario, creado_por, created_at,
		       false AS leida
		FROM notificaciones
		ORDER BY created_at DESC, id DESC
		LIMIT 200`)
	if rows == nil {
		rows = []Notificacion{}
	}
	return rows, err
}

// Delete removes a notification. Returns ErrNotFound for an unknown id.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM notificaciones WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListForUser returns notifications visible to a given user (by rol), newest first,
// with the leida flag for that specific user.
func (r *Repository) ListForUser(ctx context.Context, userID int64, rol string) ([]Notificacion, error) {
	var rows []Notificacion
	err := r.db.SelectContext(ctx, &rows, `
		SELECT n.id, n.titulo, n.cuerpo, n.tipo, n.destinatario, n.creado_por, n.created_at,
		       (nl.usuario_id IS NOT NULL) AS leida
		FROM notificaciones n
		LEFT JOIN notificacion_leidas nl
		       ON nl.notificacion_id = n.id AND nl.usuario_id = $1
		WHERE n.destinatario = 'todos'
		   OR n.destinatario = $2
		ORDER BY n.created_at DESC, n.id DESC
		LIMIT 100`,
		userID, "rol:"+rol,
	)
	if rows == nil {
		rows = []Notificacion{}
	}
	return rows, err
}

// UnreadCount returns the number of unread notifications for a user.
func (r *Repository) UnreadCount(ctx context.Context, userID int64, rol string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM notificaciones n
		WHERE (n.destinatario = 'todos' OR n.destinatario = $2)
		  AND NOT EXISTS (
		      SELECT 1 FROM notificacion_leidas nl
		      WHERE nl.notificacion_id = n.id AND nl.usuario_id = $1
		  )`,
		userID, "rol:"+rol,
	).Scan(&count)
	return count, err
}

// MarkRead marks a single notification as read for a user (idempotent).
// Only notifications visible to the user (todos or their role) can be marked;
// otherwise — or if the id does not exist — it returns ErrNotFound.
func (r *Repository) MarkRead(ctx context.Context, notifID, userID int64, rol string) error {
	var visible bool
	err := r.db.QueryRowContext(ctx, `
		WITH n AS (
			SELECT id FROM notificaciones
			WHERE id = $1 AND (destinatario = 'todos' OR destinatario = $3)
		), ins AS (
			INSERT INTO notificacion_leidas (notificacion_id, usuario_id)
			SELECT id, $2 FROM n
			ON CONFLICT DO NOTHING
		)
		SELECT EXISTS (SELECT 1 FROM n)`,
		notifID, userID, "rol:"+rol).Scan(&visible)
	if pgerr.IsFKViolation(err) { // deleted concurrently
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !visible {
		return ErrNotFound
	}
	return nil
}

// MarkAllRead marks all visible notifications as read for a user. The NOT
// EXISTS only trims the insert; ON CONFLICT DO NOTHING is what makes it safe
// against a concurrent MarkRead/MarkAllRead of the same user (H139), which
// would otherwise fail with a unique violation (23505) and a 500.
func (r *Repository) MarkAllRead(ctx context.Context, userID int64, rol string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notificacion_leidas (notificacion_id, usuario_id)
		SELECT n.id, $1
		FROM notificaciones n
		WHERE (n.destinatario = 'todos' OR n.destinatario = $2)
		  AND NOT EXISTS (
		      SELECT 1 FROM notificacion_leidas nl
		      WHERE nl.notificacion_id = n.id AND nl.usuario_id = $1
		  )
		ON CONFLICT DO NOTHING`,
		userID, "rol:"+rol)
	return err
}
