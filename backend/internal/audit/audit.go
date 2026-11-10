package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cbaciliod/learning/internal/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// WriteTimeout bounds each audit INSERT (it runs detached from the request context).
const WriteTimeout = 5 * time.Second

type Logger struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Logger {
	return &Logger{db: db}
}

// Log inserts one audit record. Failures never block the main request flow,
// but they are logged so a missing audit trail is visible to operators.
// Use it only for mutations that do not run in a transaction; otherwise use
// LogTx so the change and its audit row commit (or roll back) together.
func (l *Logger) Log(
	ctx context.Context,
	usuarioID *int64,
	accion, entidad string,
	entidadID *int64,
	detalle map[string]any,
	ip, ua string,
) {
	if l == nil {
		return
	}
	// Detach from the request context: the mutation being audited has already
	// happened, so a client disconnect must not drop its audit row. Bounded by
	// WriteTimeout so a stuck DB cannot pin the goroutine forever.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), WriteTimeout)
	defer cancel()
	if err := insert(ctx, l.db, usuarioID, accion, entidad, entidadID, detalle, ip, ua); err != nil {
		log.Error().Err(err).Str("accion", accion).Str("entidad", entidad).Msg("audit_log write failed")
	}
}

// LogTx inserts one audit record inside the caller's transaction and returns
// the error, so the caller rolls the mutation back when it cannot be audited.
// A nil Logger audits nothing (tests).
func (l *Logger) LogTx(
	ctx context.Context,
	tx sqlx.ExecerContext,
	usuarioID *int64,
	accion, entidad string,
	entidadID *int64,
	detalle map[string]any,
	ip, ua string,
) error {
	if l == nil {
		return nil
	}
	if err := insert(ctx, tx, usuarioID, accion, entidad, entidadID, detalle, ip, ua); err != nil {
		return fmt.Errorf("audit_log %s/%s: %w", accion, entidad, err)
	}
	return nil
}

// TxFunc writes the audit row of a mutation inside its transaction.
// Repositories call it (through RunTx) right before Commit; nil audits nothing.
type TxFunc func(ctx context.Context, tx sqlx.ExecerContext) error

// RunTx calls fn when it is not nil.
func RunTx(ctx context.Context, tx sqlx.ExecerContext, fn TxFunc) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, tx)
}

func insert(
	ctx context.Context,
	ex sqlx.ExecerContext,
	usuarioID *int64,
	accion, entidad string,
	entidadID *int64,
	detalle map[string]any,
	ip, ua string,
) error {
	// Pass true nil interface so pq sends SQL NULL for jsonb when detalle is absent.
	var detalleArg any
	if detalle != nil {
		b, err := json.Marshal(detalle)
		if err != nil {
			log.Error().Err(err).Str("accion", accion).Str("entidad", entidad).Msg("audit_log detalle marshal failed")
		} else {
			detalleArg = b
		}
	}
	_, err := ex.ExecContext(ctx, `
		INSERT INTO audit_log (usuario_id, accion, entidad, entidad_id, detalle, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6::inet, $7)`,
		usuarioID, accion, entidad, entidadID, detalleArg, ValidIP(ip), ua)
	return err
}

// ValidIP returns a pointer to the canonical form of ip, or nil (SQL NULL)
// when ip is empty or not a valid IP address. Use it for any inet column.
func ValidIP(ip string) *string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil
	}
	s := parsed.String()
	return &s
}

// ClientIP extracts the real client IP from the request. X-Forwarded-For is
// only honoured when the direct peer is in TRUSTED_PROXIES (see
// middleware.SetTrustedProxies). Returns "" when no valid IP is available.
func ClientIP(r *http.Request) string {
	return middleware.ClientIP(r)
}
