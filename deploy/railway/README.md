# Railway (producción)

El runbook completo está en [`docs/ops/railway.md`](../../docs/ops/railway.md):
servicios, imágenes y tags, configuración por servicio, backups, monitoreo y
lo que falta confirmar. El checklist de release está en
[`docs/ops/checklist-release.md`](../../docs/ops/checklist-release.md).

Aquí no hay archivos de configuración. El config as code de Railway
(`railway.json`) está obsoleto (deja de leerse el 01-12-2026), así que la
configuración va en el panel y queda escrita en el runbook. Nada se crea ni se
cambia en Railway sin la aprobación del usuario.

## Plantilla de variables

Solo nombres y valores de ejemplo, **sin secretos**. Los secretos se generan
con `openssl rand -hex 32`, se cargan como **variables selladas** y se guarda
una copia fuera de Railway. **Nunca** se reutiliza un valor de
`deploy/local/preprod.vars`.

### Servicio `backend` (imagen `cbaciliod/learning-backend:vX.Y.Z`)

| Variable | Valor | Sellada |
|---|---|---|
| `ENV` | `production` | no |
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}?sslmode=require` | no (es una referencia) |
| `JWT_SECRET` | 64 hex (`openssl rand -hex 32`) | **sí** |
| `ENCRYPTION_KEY` | 64 hex (`openssl rand -hex 32`). El usuario guarda una copia fuera de Railway | **sí** |
| `CORS_ORIGINS` | `https://<dominio del frontend>` | no |
| `FRONTEND_BASE_URL` | `https://<dominio del frontend>` | no |
| `TRUSTED_PROXIES` | rango del proxy de Railway (pendiente de confirmar) | no |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_FROM` | los del proveedor de correo | no |
| `SMTP_PASSWORD` | la del proveedor | **sí** |
| `LOG_LEVEL` | vacío (`info`) | no |
| `MIGRATE_LOCK_TIMEOUT` | `10m` | no |
| `RAILWAY_HEALTHCHECK_TIMEOUT_SEC` | `660` | no |
| `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` | `45` | no |

### Servicio `frontend` (imagen `cbaciliod/learning-frontend:vX.Y.Z`)

Sin variables propias: `NEXT_PUBLIC_*` van dentro de la imagen y se fijan al
construirla. `PORT` lo inyecta Railway.
