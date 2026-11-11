# ADR 0005: estado operativo para el panel de salud

- **Estado:** aceptado (2026-11-11). Las decisiones 1 y 2 las aprobó el tech-lead; el PO apoya la 1.
- **Fecha:** 2026-11-11
- **Historias:** PLT-01 a PLT-05 (`docs/stories/plataforma/`, PR #108)
- **Contrato:** [docs/api/plataforma-salud.md](../api/plataforma-salud.md)
- **Relacionado:** [ADR 0004 § Decisión 11](0004-alojamiento-base-de-datos-produccion.md) (copia diaria a R2)

## Contexto

El PO pide un panel admin con la salud de la API, la BD, el correo, el worker de ingesta y la copia a R2. Revisando el código para diseñarlo aparecieron dos problemas que van más allá del panel:

1. **La autenticación con la BD caída.**
   - `RequireAuth` consulta la sesión en la BD en cada request (`backend/internal/auth/middleware.go:47-54`; `FindActiveSesion`, `auth/repository.go:564-579`) y responde **401** tanto si la sesión no existe como si la consulta **falla**.
   - El frontend trata todo 401 como sesión expirada y dispara `auth:expired` (p. ej. `frontend/src/lib/admin/api.ts:13`, `catalogs/api.ts:15`, `questions/api.ts:15`).
   - Resultado: **una caída de la BD saca a todos los usuarios** del panel y de la app, no solo al admin del panel de salud.
2. **El registro de la copia diaria.** El panel tiene que mostrar la última copia correcta, con dos restricciones:
   - el cron conecta con `backup_ro`, de solo lectura (ADR 0004 § Decisión 11);
   - el backend no recibe credenciales de R2.

## Opciones consideradas

### 1. Autenticación con la BD caída

- **A. Dejarlo como está (401).** Es lo más simple, pero una caída transitoria de la BD desloguea a todos. Descartada.
- **B. Responder 503 cuando el checker devuelve un error.** La petición sigue sin pasar (falla cerrado), pero el frontend no confunde la caída con una sesión expirada. Es coherente con `/health`, que ya responde 503 cuando la BD falla (`cmd/server/main.go:114-116`). **Elegida.**
- **C. Validar solo el JWT cuando la BD falla.** Dejaría pasar sesiones revocadas o usuarios desactivados durante la caída (falla abierto). Descartada por seguridad.

### 2. Registro de la última copia

- **A. Tabla en la BD con INSERT del cron.** La API la lee como cualquier dato, es portable (Decisión 9 de la ADR 0004) y no expone R2. **Elegida.**
- **B. Objeto de estado en R2.** La API necesitaría credenciales de R2. Descartada.
- **C. API de Railway (estado del cron).** Requiere un token nuevo en el backend y ata el código a Railway (ADR 0004 § Decisión 9). Descartada.
- **D. Rol separado `backup_registro`, propuesto por devops.** No da más seguridad: el mismo cron tendría las dos credenciales, y agrega un secreto. Descartada por el tech-lead.

## Decisión

1. **`RequireAuth` responde 503 cuando `checkSession` devuelve un error.**
   - El cuerpo es `{"error": "Servicio no disponible."}`, en el formato heredado.
   - El 401 queda solo para «no existe, expiró, fue revocada, otro usuario o estado ≠ activo».
   - Nunca deja pasar la petición con un error.
   - Aplica a **todos** los endpoints autenticados.
   - El frontend **no** dispara `auth:expired` con 503 ni con 403.
   - Tareas:
     - backend-dev: middleware y tests de sesión inexistente → 401, error de BD → 503 y sesión válida → pasa;
     - frontend-dev: revisar los clientes de `src/lib/*/api.ts`.
2. **Tabla `copias_bd` con una fila por ejecución**, insertada por el cron al terminar.
   - El rol `backup_ro` conserva `pg_read_all_data` y suma **solo** `INSERT` sobre `copias_bd` y `USAGE` sobre su secuencia. Sin UPDATE ni DELETE.
   - El GRANT es un paso de aprovisionamiento documentado por devops, no va en goose, porque el rol no existe en CI ni en compose. La tabla sí se crea con goose (backend).
   - Esquema y reglas en el contrato.
3. **Caché del resultado en el proceso, 15 s**, sin Redis ni tabla.
   - Con varias réplicas, cada una cachea por su cuenta.
   - Se usa un `sync.Mutex` para que las peticiones simultáneas compartan una ejecución.
4. **Versión del frontend desde `frontend/package.json`**, la que ya muestra «Acerca de», sin variable de build nueva. El release verifica que coincida con el tag.

## Consecuencias

- **Positivas:**
  - Una caída de la BD ya no desloguea a nadie: la sesión se conserva y la UI muestra un error del servidor.
  - El panel no necesita secretos nuevos.
  - Todo sigue siendo portable entre nubes.
- **Negativas:**
  - Con la BD totalmente caída, el panel no puede mostrar `base_datos: caido`, porque la autenticación falla antes y responde 503. La UI muestra «No se pudo consultar». Los escenarios de BD caída de PLT-01 se prueban con la sonda aislada, y el PO los reformula.
  - `backup_ro` deja de ser estrictamente de solo lectura: tiene un único INSERT sobre una tabla sin datos personales. Queda documentado en la ADR 0004 § Decisión 11.
  - El dump incluye ese GRANT. Al restaurar en un Postgres sin el rol hay que usar `pg_restore --no-owner --no-privileges` o crear antes el rol (pruebas C6/C7 de la ADR 0004).
  - Si el INSERT del cron falla después de una subida correcta, el panel muestra la copia como atrasada aunque exista en R2. Se acepta: el cron alerta (ADR 0004, C13).
