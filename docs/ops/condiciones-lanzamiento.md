# Condiciones de lanzamiento (ADR 0004)

Registro de verificación de las condiciones C1–C13 del
[ADR 0004](../adr/0004-alojamiento-base-de-datos-produccion.md) (aceptado con
condiciones). Ninguna se marca como cumplida sin ejecutarla. Cada fila se
completa con la fecha, quién la verificó y el resultado medido.

**Críticas:** C5 (RPO) y C6 (RTO). Sin ellas no se abre la plataforma a
usuarios.

Todo lo que se haga en Railway o en R2 lo **aprueba el usuario**. Los
procedimientos están en [railway.md](railway.md).

| # | Condición | Cómo se verifica | Estado | Fecha · quién · resultado |
|---|---|---|---|---|
| C1 | El plan Pro incluye backups de volumen, PITR y cron jobs | En el panel o con soporte de Railway | Pendiente | |
| C2 | Postgres 16 con `pg_trgm` | `SELECT version();` y `SELECT * FROM pg_available_extensions WHERE name='pg_trgm';` | Pendiente | |
| C3 | `sslmode=require` conecta por la URL privada | El backend arranca con `DATABASE_URL=${{Postgres.DATABASE_URL}}?sslmode=require` (H-5 lo exige). Se anota cómo quedó el parámetro | Pendiente | |
| C4 | `max_connections` ≥ 2 × 25 × réplicas + margen, más el cron | `SHOW max_connections;` | Pendiente | |
| C5 | **RPO ≤ 5 min con PITR** | Restauración de prueba de PITR ([railway.md](railway.md) §6): diferencia entre el último commit y el punto restaurable | Pendiente | |
| C6 | **RTO ≤ 2 h**, por PITR y desde R2 | Las dos restauraciones cronometradas de [railway.md](railway.md) §6 | Pendiente | |
| C7 | La copia de R2 corre a diario, cifrada, con ciclo de vida y bucket lock de 30 días, y se puede restaurar | Primera ejecución del cron más la restauración de prueba desde R2 | Pendiente (implementación en `deploy/backup/`) | |
| C8 | Región y cifrado en reposo de los backups de Railway y del bucket de PITR | En el panel o con soporte | Pendiente | |
| C9 | Qué pasa con `DATABASE_URL` si se activa el pooling de Railway | En el panel o con soporte. Mientras no se active, no aplica | Pendiente | |
| C10 | Cómo se ejecutan `cmd/seed/admin` y `cmd/rehash-emails` con la base privada y las variables selladas | Procedimiento propuesto: incluir esos binarios en la imagen del backend y ejecutarlos dentro del contenedor del servicio (las variables selladas sí están en tiempo de ejecución). Falta confirmar en la documentación de Railway cómo abrir una sesión en el contenedor | Propuesto | |
| C11 | Las imágenes públicas no llevan secretos, datos ni `.env` | `make docker/verify DOCKER_TAG=vX.Y.Z` en cada release (`docker/push` depende de él) | Verificado en local con imágenes de prueba (11-11-2026). Se repite en cada release | 11-11-2026 · devops · las dos imágenes pasan; una imagen mala a propósito se rechaza |
| C12 | Ubicación o jurisdicción del bucket de R2 | La decide el usuario antes de crear el bucket | Pendiente (usuario) | |
| C13 | El cron alerta si falla o no corre | Una ejecución fallida provocada a propósito | Pendiente (implementación en `deploy/backup/`) | |
| C14 | Uptime Kuma vigila `/health` y avisa por email al usuario (docs/ops/railway.md §7.1) | Monitor temporal hacia una ruta 404: llega el email de caído, y el de recuperado al pausarlo | Pendiente (lo crea el usuario) | |
