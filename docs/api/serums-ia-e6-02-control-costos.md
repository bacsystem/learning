# Contrato API — SERUMS IA, SIA-E6-02 Control de costos: presupuesto, límites y caché

- **Historia:** [SIA-E6-02](../stories/serums-ia/E6-02-control-costos.md), issue #40.
- **Estado:** borrador para revisión del tech-lead (2026-11-12).
  - Aplica las decisiones D14–D21 del PO (supuesto 25).
  - Respeta el cambio de alcance aprobado por el usuario: entran los límites y la API de consumo; la pantalla del postulante no entra.
- **ADR:** [0009](../adr/0009-control-costos-ia-reserva-sincrona.md). Aplica la [ADR 0002](../adr/0002-secreto-hmac-contadores-antifraude.md) (HMAC) y extiende la [ADR 0006](../adr/0006-puerto-servicio-ia.md).
- **Convenciones:** [README.md](README.md). Todos los errores usan el **formato 2**.
- **Depende de:** E6-01 (`internal/ia`, migración **066** `ia_usos`, PR #166).
- **Migración:** **068**, reservada. Orden de merge: 065 → 066 → 067 → 068.
- **Solo con el doble.**
  - El proveedor real, su modelo y sus precios son **SIA-E6-10** (#173). Esa historia está bloqueada hasta que el usuario los apruebe.
  - Mientras tanto, en producción `IA_PROVEEDOR` queda vacío y el servicio, deshabilitado.

## 1. Impacto en el código existente

| # | Lugar | Hoy | Cambio |
|---|---|---|---|
| C1 | `internal/ia` (E6-01) | Llama sin límites, sin caché y sin presupuesto (E6-01 §2) | Se suman `costos.go` (acciones y presupuesto), `cache.go`, `configuracion.go` y las rutas de §3. El flujo pasa a ser el de §2.3 |
| C2 | `ia.Solicitud` | `Operacion`, `Instrucciones`, `Datos`, `EsquemaSalida` | Se suma **`Fuentes []int64`**: los `fuente_id` cuyo contenido va en `Datos`. Lo llena el consumidor y se usa para la vigencia de la caché (§4) |
| C3 | Interfaz `ia.Servicio` | `Clasificar(ctx, a Actor, in)` y las otras tres | El actor sale de la reserva: `Clasificar(ctx, r *Reserva, in)`. Se suma `BuscarEnCache` (§2.2) |
| C4 | `ia.Codigo(err)` (E6-01 §5) | — | Mapea los errores nuevos (§5) |
| C5 | `ia_usos.resultado` (066, que ya incluye `cancelado`) | 8 valores | Se suma `presupuesto_agotado` (§6) |
| C6 | `internal/config` | Sin `ANTIFRAUD_HMAC_KEY` | Entra aquí si E6-02 se implementa antes que E4-01. Archivos: ADR 0002 § Archivos |
| C7 | `cmd/server/main.go` | — | Importa `time/tzdata` (librería estándar) y monta las rutas de §3 con `Mount(mux, svc, audLog, …)` (README §1) |
| C8 | `notificaciones` (037) | Solo las crea el admin (`notifications/repository.go:24-30`) | El aviso del 80 % lo inserta el sistema: `tipo = 'sistema'`, `destinatario = 'rol:admin'` y `creado_por = NULL`. Los lectores ya filtran por `rol:<rol>` (`repository.go:76-77`) |

- **C2–C5 cambian el contrato de E6-01**, y van **en esta historia**, en las tareas K1–K5 de backend-dev-2. **No** en el PR #166 de E6-01, que ya está aprobado y se mergea con su contrato. El PR de E6-02 actualiza la interfaz y sus tests. Como hasta entonces E6-01 no tiene consumidores, el cambio no rompe nada.
- **Consumidores:** E3-04, E6-03, E6-06 y E6-08 usan esta API cuando lleguen.
- **E5-06** no consume (regla 7) y, en el MVP, **el postulante no tiene ninguna acción IA real** (D21).
- E6-02 se prueba con el doble y con un consumidor de test. Los escenarios de cuota del postulante se prueban por API y por servicio.

## 2. Acciones y presupuesto

### 2.1 Límites

| Actor | Día | Mes | Presupuesto |
|---|---|---|---|
| `usuario` | `limite_usuario_dia` (5) | `limite_usuario_mes` (60) | sí |
| `admin` | `limite_admin_dia` (200) | — | sí |
| `sistema` | — | — | sí |

- **Ventanas:** día y mes calendario en `America/Lima`. `ventana_inicio` es la medianoche de Lima, guardada como `timestamptz`. `reinicia_at` es el inicio de la ventana siguiente.
- **Vencimiento (`expira_en`):** la fila del día vence a la medianoche siguiente y la del mes, el día 1 siguiente. Nada sobrevive al mes en curso (regla 11).
- **Qué cuenta como acción IA real:** una invocación que llega al LLM y **termina bien**. No cuentan la caché, un rechazo antes de llamar (servicio deshabilitado, tamaño, datos personales), un fallo ni un corte por presupuesto. En todos esos casos se libera la acción (regla 6; D15).
- **Bajar un límite** por debajo de lo ya consumido no es retroactivo: las reservas siguientes dan `429` (D19).
- **Contadores:** postulante y admin usan la misma tabla y la misma clave `ia_cuota:<hmac>` (ADR 0009). `sistema` no tiene contador.

### 2.2 API Go para consumidores (propuesta; backend-dev-2 la ajusta en su PR)

```go
// Reserva vive en el job del consumidor. No lleva la clave HMAC (ADR 0009).
type Reserva struct {
    Actor    Actor
    Acciones int       // pendientes de usar
    Dia, Mes time.Time // inicio de cada ventana; Mes es cero para admin y sistema
}

// Request síncrono, sin LLM:
// Con un acierto, escribe su fila en ia_usos (desde_cache = true), con el actor.
func BuscarEnCache[S any](ctx context.Context, s Servicio, a Actor, in Entrada[S]) (S, bool, error)
func (s) Reservar(ctx context.Context, a Actor, acciones int) (Reserva, error) // todo o nada (D17)
func (s) Consumo(ctx context.Context, a Actor) (Consumo, error)                // acciones restantes
func (s) Configuracion(ctx context.Context) (Config, error)                    // max_preguntas_solicitud, lote_clasificacion…

// Job asíncrono:
Clasificar(ctx context.Context, r *Reserva, in EntradaClasificar) (SalidaClasificar, error) // y las otras tres
func (s) Liberar(ctx context.Context, r *Reserva, acciones int) error
```

- **`Reservar`** corre en una transacción y sigue el orden **D16**. El primer fallo es el código que se devuelve, y revierte todo:
  1. **Presupuesto:** `gastado_usd` del mes ≥ `presupuesto_mensual_usd` da `ErrPresupuestoAgotado`.
  2. **Mes** (solo `usuario`): `INSERT INTO ia_cuotas … ON CONFLICT (clave, ventana, ventana_inicio) DO UPDATE SET usados = ia_cuotas.usados + $n WHERE ia_cuotas.usados + $n <= $limite_efectivo RETURNING usados`. Si no devuelve fila, o si `n > límite`, da `*ErrLimite{Ventana: "mes"}`.
  3. **Día:** el mismo `INSERT`; si falla, `*ErrLimite{Ventana: "dia"}`.
  - **`límite_efectivo`** = límite − contador de la clave anterior durante una rotación (ADR 0002, paso 3).
  - **`ErrLimite`** trae `Limite`, `Usados` y `ReiniciaAt`. Con esos datos el consumidor calcula cuántas acciones caben (E3-04 lo multiplica por el lote).
- **`Liberar`** resta: `usados = GREATEST(usados − n, 0)` en las ventanas de la reserva. Usa la clave vigente y, si la fila no existe, la anterior.
- **`Consumo`** suma la clave vigente y la anterior.
- **Reintento** (regla 6): el job reintentado usa la **misma** reserva.

### 2.3 Flujo de cada invocación en el job (sustituye a E6-01 §2 en los pasos ★)

| Paso | Regla | Si falla |
|---|---|---|
| ★0 | `r.Acciones ≥ 1` y actor válido | `ErrSinReserva` (error de programación): `500`, sin llamar ni registrar |
| ★1 | Caché vigente (§4) | Si hay respuesta: `Liberar(r, 1)` y una fila en `ia_usos` con `desde_cache = true`, `resultado = 'ok'`, `proveedor = ''` (no se llamó a ninguno, como en `no_disponible`), `modelo = NULL`, costo 0 e intentos 0. Se devuelve la salida. Este paso va antes que «habilitado» (paso 2), así que la caché se sirve también con el servicio deshabilitado |
| 2–4 | Pasos 1–3 de E6-01: habilitado, tamaño, redacción | Error de E6-01 y `Liberar(r, 1)` |
| ★5 | Presupuesto: `gastado ≥ presupuesto` (D14) | `ErrPresupuestoAgotado`, `Liberar(r, 1)` y `ia_usos` con `presupuesto_agotado`. Si falla la BD al leer el presupuesto: `ErrCostos`, `Liberar(r, 1)`, `ia_usos` con `error_interno` y no se llama al proveedor (review del #202, H2) |
| 6–8 | Pasos 4–6 de E6-01: llamada, reintentos, validación | — |
| ★9 | En una transacción: fila del mes `FOR UPDATE` (se crea si no existe); `gastado_usd += uso.CostoUSD` de **todos** los intentos, también si falló (D15); evaluación de la alerta (§2.4) | Si falla, log con `op`; no invalida la salida |
| ★10 | Si salió bien: `r.Acciones--` y se escribe la caché. Si falló: `Liberar(r, 1)` | — |
| 11 | Registro en `ia_usos` (paso 7 de E6-01) | — |

- **Exceso aceptado (D14):** las invocaciones que ya pasaron el paso ★5 terminan aunque otra lleve el gasto al 100 %.
- **Corte a mitad de un lote** (E3-04 regla 7): con `ErrPresupuestoAgotado`, el consumidor se detiene y hace `Liberar(r, r.Acciones)`.
- **Mes calendario** (regla 3): la fila del mes nuevo arranca en 0 (escenario «Reinicio mensual»).
- **Subir el presupuesto** reabre las llamadas de inmediato (D19).
- **Bajarlo** a un valor ≤ al gasto del mes las corta en ese momento (regla 18): la siguiente reserva o el siguiente ★5 dan `presupuesto_agotado`.
- **D18:** `ia_gasto_mes` no lleva `actor_id`, así que E9-07 no la toca y el gasto no baja.

### 2.4 Aviso del 80 % (regla 2; D19, D20)

- `ia_gasto_mes.alerta_80_presupuesto_usd` guarda el valor del presupuesto para el que ya se avisó. Vale `NULL` si todavía no se avisó.
- **Condición:** `gastado ≥ 0,8 × presupuesto` **y** `alerta_80_presupuesto_usd IS DISTINCT FROM presupuesto`. Si se cumple, en la misma transacción:
  - se pone `alerta_80_presupuesto_usd = presupuesto` y `alerta_80_at = now()`;
  - se inserta la notificación.
- **Cuándo se evalúa:**
  - al sumar gasto (★9);
  - en el `PUT` de configuración (§3.3), cada vez que cambia el presupuesto, tanto si sube como si baja (regla 18). En la misma transacción, el `PUT` **reinicia la marca** del mes: pone `alerta_80_presupuesto_usd` y `alerta_80_at` en `NULL`. Después evalúa la condición. Si el gasto ya está en ≥ 80 % del nuevo valor, la alerta se envía en ese momento. Escenario «Bajar el presupuesto»: de 40 a 24 USD con 20 gastados, la alerta sale al guardar.

  Con el mismo valor del presupuesto, la alerta se envía una sola vez. **Cada cambio de valor la rearma**, aunque se vuelva a un valor ya avisado (review del #202). Ejemplo: se avisó con 40, se cambia a 50 (con 35 gastados no se avisa) y se vuelve a 40. Al volver a 40, la alerta sale de nuevo, porque el cambio anterior reinició la marca. Un `PUT` que no cambia el presupuesto no toca la marca.
- **Notificación:** va por la campana (in-app).
  - `titulo = "Presupuesto de IA al 80 %"`;
  - `cuerpo = "Se gastaron 32,10 USD de 40,00 USD en noviembre de 2026."`;
  - `tipo = 'sistema'`, `destinatario = 'rol:admin'`, `creado_por = NULL`.

  El texto va en español, como el resto de las notificaciones guardadas. El correo queda fuera de alcance (D20).

## 3. Endpoints

### 3.1 `GET /serums-ia/ia/consumo` (cualquier autenticado; regla 9, D21)

```json
{
  "dia": { "usados": 3, "limite": 5, "disponibles": 2, "reinicia_at": "2026-11-13T05:00:00Z" },
  "mes": { "usados": 20, "limite": 60, "disponibles": 40, "reinicia_at": "2026-12-01T05:00:00Z" },
  "presupuesto_agotado": false
}
```

- El actor sale de `claims` (sin IDOR). Para un admin, `mes` es `null`.
- No exige inscripción y no se audita.
- **Sin pantalla del postulante en esta historia** (D21): la API queda lista para la primera historia que le dé una acción IA real.

### 3.2 `GET /admin/ia/configuracion` (admin)

```json
{ "limite_usuario_dia": 5, "limite_usuario_mes": 60, "limite_admin_dia": 200,
  "max_preguntas_solicitud": 20, "lote_clasificacion": 20, "presupuesto_mensual_usd": 40.00,
  "updated_at": "2026-11-12T15:04:05Z" }
```

### 3.3 `PUT /admin/ia/configuracion` (admin; regla 8)

Lleva **todos** los campos de §3.2 salvo `updated_at`. Es un reemplazo completo.

| # | Validación, en orden | Error |
|---|---|---|
| 1 | JSON válido, sin campos desconocidos y dentro del límite | `400 cuerpo_invalido` / `413 cuerpo_demasiado_grande` |
| 2 | Rangos (D19), con todos los campos presentes:<br>• `limite_usuario_dia`: 0–50<br>• `limite_usuario_mes`: 0–1 000, y además `limite_usuario_dia` ≤ `limite_usuario_mes`<br>• `limite_admin_dia`: 1–1 000<br>• `presupuesto_mensual_usd`: 1–1 000, con ≤ 2 decimales (`dec2`, README §1)<br>• `max_preguntas_solicitud` y `lote_clasificacion`: 1–50 (rango técnico; la historia los fija en 20) | `422 validacion` (`detalle.campos`) |

- Bloquea la fila `FOR UPDATE` y responde `200` con el objeto de §3.2.
- Si cambió el presupuesto, evalúa la alerta (§2.4) en la misma transacción.
- **Auditoría** en la misma transacción: `accion = "ia.configurar"`, `entidad = "ia_configuracion"`, `entidad_id = 1`, `detalle = {"cambios": {"<campo>": {"antes": x, "despues": y}}}`, solo con los campos que cambiaron. Si no cambió nada: `200` sin fila de auditoría.

### 3.4 `GET /admin/ia/gasto` (admin; regla 9)

```json
{
  "mes": "2026-11",
  "presupuesto_usd": 40.00,
  "gastado_usd": 32.10,
  "porcentaje": 80.25,
  "alerta_80_at": "2026-11-20T18:00:00Z",
  "agotado": false,
  "reinicia_at": "2026-12-01T05:00:00Z",
  "por_operacion": [
    { "operacion": "clasificar", "invocaciones": 29, "desde_cache": 3, "costo_usd": 1.45 }
  ]
}
```

- Solo el mes en curso.
- `gastado_usd` sale de `ia_gasto_mes`, que es el valor que manda.
- `por_operacion` agrega `ia_usos` del mes en hora de Lima. Ninguna cifra es **por usuario**: el consumo individual solo lo ve su dueño.
- Los montos se redondean a 2 decimales en la respuesta y se guardan con 6.
- `porcentaje` va con 2 decimales y puede superar 100, por el exceso de D14.

## 4. Caché (regla 5; D16)

- **Clave:** `hex(sha256(operacion ‖ esquema_salida ‖ instrucciones ‖ datos))`, con `\x00` entre las partes.
  - En `datos`, cada bloque es `etiqueta + "\x00" + texto`, con los espacios colapsados y `TrimSpace`.
  - El texto de los pasajes ya va en `datos`, así que la versión de la fuente entra en la clave por su contenido.
  - Si cambian las instrucciones, la clave cambia y la caché anterior deja de usarse.
- **Vigencia:** al escribir se guarda, para cada `fuente_id` de `Solicitud.Fuentes`, su `version_vigente_id` **en ese momento**. La entrada vale si se cumplen las dos cosas:
  - no pasó su `expira_en`, que es `created_at` + 30 días;
  - cada una de esas fuentes sigue `activa` y con la **misma** versión vigente.

  Si alguna falla, no hay respuesta y la entrada se borra (escenario «Invalidación»). Retirar una fuente también la invalida.
  - **Por qué se compara con la versión vigente al escribir:** una explicación (E6-08) usa el pasaje de la versión citada, que puede ser anterior a la vigente. Comparar con la versión del contenido haría que esa entrada naciera inválida.
- **Escritura:** solo con resultado `ok` y una salida de ≤ 256 KiB, con `ON CONFLICT (clave) DO NOTHING`. Al escribir se purgan hasta 100 entradas vencidas.
- **Qué guarda:** la salida validada, la operación y los pares (fuente, versión vigente). **Nunca** el actor, el prompt ni la entrada. Se comparte entre actores, porque la entrada ya pasó por la redacción de datos personales.
- **Dónde se consulta:** en el request, **antes** de `Reservar`, de modo que se sirve con el presupuesto o el límite agotados (D16), y otra vez en el job (★1).
- **Regenerar:** si un consumidor quiere una respuesta nueva (p. ej. «regenerar» en E6-03), agrega a los datos un bloque `regeneracion` con un contador. La clave cambia y la acción cuenta.

## 5. Errores

| Error Go | Código | HTTP | `detalle` |
|---|---|---|---|
| `ErrPresupuestoAgotado` | **`presupuesto_agotado`** | 429 | `{"reinicia_at"}` (inicio del mes siguiente en Lima) |
| `*ErrLimite{Ventana:"mes"}` | **`limite_ia_mensual`** | 429 | `{"limite", "usados", "reinicia_at"}` |
| `*ErrLimite{Ventana:"dia"}` | **`limite_ia`** | 429 | `{"limite", "usados", "reinicia_at"}`; E3-04 suma `caben_preguntas` |
| `ErrSinReserva` | `error_interno` | 500 | — |
| `ErrCostos` (falla la BD al leer el presupuesto en ★5) | `error_interno` | 500 | —. En `ia_usos`, resultado `error_interno` |

- `ia.Codigo(err)` mapea estos errores y mantiene la firma `(codigo, http)` de E6-01. Para `*ErrLimite`, elige `limite_ia` o `limite_ia_mensual` según `Ventana`.
- El `detalle` lo da una función aparte, **`ia.Detalle(err) map[string]any`**, que devuelve `nil` si el error no trae detalle. Acordado con backend-dev-2, para no volver a cambiar la firma de `Codigo`.
- Los endpoints de §3 usan `400`, `401`/`403` del middleware, `413`, `422 validacion` y `500`.
- **i18n** (README §8):
  - de módulo: `dashboard.serumsIa.errores.{presupuesto_agotado,limite_ia,limite_ia_mensual}`, con la hora de reinicio en la zona del navegador;
  - de recurso: `dashboard.serumsIa.ia.errores.validacion`;
  - el panel va en `dashboard.serumsIa.ia.*`.
  - Los USD van con el formato de moneda del locale y la receta de formato de khipu. Si la receta no tiene moneda, va primero un PR a khipu.

## 6. Modelo de datos — migración 068

```sql
-- +goose Up
CREATE TABLE ia_configuracion (
    id                      SMALLINT      PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    limite_usuario_dia      INT           NOT NULL DEFAULT 5   CHECK (limite_usuario_dia BETWEEN 0 AND 50),
    limite_usuario_mes      INT           NOT NULL DEFAULT 60  CHECK (limite_usuario_mes BETWEEN 0 AND 1000),
    limite_admin_dia        INT           NOT NULL DEFAULT 200 CHECK (limite_admin_dia BETWEEN 1 AND 1000),
    max_preguntas_solicitud INT           NOT NULL DEFAULT 20  CHECK (max_preguntas_solicitud BETWEEN 1 AND 50),
    lote_clasificacion      INT           NOT NULL DEFAULT 20  CHECK (lote_clasificacion BETWEEN 1 AND 50),
    presupuesto_mensual_usd NUMERIC(10,2) NOT NULL DEFAULT 40.00 CHECK (presupuesto_mensual_usd BETWEEN 1 AND 1000),
    updated_by              BIGINT        REFERENCES usuarios(id),
    updated_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ia_configuracion_mes CHECK (limite_usuario_dia <= limite_usuario_mes)
);
INSERT INTO ia_configuracion (id) VALUES (1);

-- Contadores antifraude (ADR 0002): sin usuario_id ni FK; SIA-E9-07 no los borra.
CREATE TABLE ia_cuotas (
    clave          TEXT        NOT NULL CHECK (clave LIKE 'ia_cuota:%'),
    ventana        VARCHAR(3)  NOT NULL CHECK (ventana IN ('dia', 'mes')),
    ventana_inicio TIMESTAMPTZ NOT NULL,
    usados         INT         NOT NULL CHECK (usados >= 0),
    expira_en      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (clave, ventana, ventana_inicio)   -- el día 1, el día y el mes empiezan a la misma hora
);
CREATE INDEX idx_ia_cuotas_expira ON ia_cuotas (expira_en);

-- Acumulador del gasto: sin actor_id, para que el borrado de E9-07 no lo baje (D18).
CREATE TABLE ia_gasto_mes (
    mes                       DATE          PRIMARY KEY CHECK (EXTRACT(DAY FROM mes) = 1),
    gastado_usd               NUMERIC(12,6) NOT NULL DEFAULT 0 CHECK (gastado_usd >= 0),
    alerta_80_presupuesto_usd NUMERIC(10,2),   -- valor del presupuesto ya avisado
    alerta_80_at              TIMESTAMPTZ,
    updated_at                TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ia_gasto_mes_alerta CHECK ((alerta_80_presupuesto_usd IS NULL) = (alerta_80_at IS NULL))
);

CREATE TABLE ia_cache (
    clave       CHAR(64)    PRIMARY KEY,
    operacion   VARCHAR(30) NOT NULL CHECK (operacion IN
                ('clasificar', 'generar_preguntas', 'generar_material', 'generar_explicacion')),
    salida      JSONB       NOT NULL,
    fuente_ids  BIGINT[]    NOT NULL DEFAULT '{}',
    vigente_ids BIGINT[]    NOT NULL DEFAULT '{}',   -- version_vigente_id de cada fuente al escribir
    expira_en   TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ia_cache_fuentes CHECK (cardinality(fuente_ids) = cardinality(vigente_ids))
);
CREATE INDEX idx_ia_cache_expira ON ia_cache (expira_en);

-- El CHECK de 066 no tiene nombre: Postgres le pone ia_usos_resultado_check. Sin IF EXISTS a propósito, como en 065.
ALTER TABLE ia_usos DROP CONSTRAINT ia_usos_resultado_check;
ALTER TABLE ia_usos ADD CONSTRAINT ck_ia_usos_resultado CHECK (resultado IN ('ok', 'no_disponible',
    'entrada_demasiado_grande', 'datos_personales', 'timeout', 'cancelado', 'salida_invalida',
    'error_proveedor', 'presupuesto_agotado', 'error_interno'));

-- +goose Down
UPDATE ia_usos SET resultado = 'no_disponible' WHERE resultado = 'presupuesto_agotado';
UPDATE ia_usos SET resultado = 'error_proveedor' WHERE resultado = 'error_interno';
ALTER TABLE ia_usos DROP CONSTRAINT IF EXISTS ck_ia_usos_resultado;
ALTER TABLE ia_usos ADD CONSTRAINT ia_usos_resultado_check CHECK (resultado IN ('ok', 'no_disponible',
    'entrada_demasiado_grande', 'datos_personales', 'timeout', 'cancelado', 'salida_invalida', 'error_proveedor'));
DROP TABLE IF EXISTS ia_cache;
DROP TABLE IF EXISTS ia_gasto_mes;
DROP TABLE IF EXISTS ia_cuotas;
DROP TABLE IF EXISTS ia_configuracion;
```

- **Efecto sobre los datos existentes:** ninguno. Las tablas son nuevas y en `ia_usos` solo se amplía el `CHECK`.
- **Qué pierde el Down:** la configuración (vuelve a los valores por defecto), los contadores, el gasto del mes y la caché. Además, las filas con resultado `presupuesto_agotado` pasan a `no_disponible`, y las de `error_interno`, a `error_proveedor`.
- **Nombre del `CHECK`:** confirmado en el #166 (b474ec6). El `CHECK` de `resultado` de la 066 no tiene nombre explícito, así que Postgres lo llama `ia_usos_resultado_check`, e incluye `cancelado`.
- **Purga perezosa** (como en 056): las filas vencidas de `ia_cuotas` se borran al reservar, hasta 100 cada vez.

## 7. Seguridad

- **Contadores** (regla 11; escenario «Contador sin datos personales»): solo la clave `ia_cuota:<hmac>`, la ventana, su inicio, el número y el vencimiento. El HMAC nunca va a los logs ni a los jobs.
- **Verificación en el servidor** antes de cada llamada. Las acciones se reservan con bloqueo de fila (reglas 4 y 10). El cliente nunca envía un `usuario_id`.
- El consumo individual solo lo ve su dueño (§3.1); el admin ve agregados (§3.4).
- **Auditoría:** el `PUT` de configuración se audita (§3.3).
- **Logs:** solo `op`, el código, la ventana y el resultado.
- **Variables:** `ANTIFRAUD_HMAC_KEY` y `ANTIFRAUD_HMAC_KEY_PREVIOUS` según la ADR 0002. Es tarea de devops.

## 8. Trazabilidad con los criterios de aceptación

| Escenario | Cómo se cumple |
|---|---|
| Dentro del límite | §2.2 `Reservar`; §3.1 muestra 3 de 5 (por API, D21) |
| Límite diario / mensual / de admin | §2.2: `429 limite_ia` / `limite_ia_mensual` con `reinicia_at`, sin llamar al proveedor |
| Caché | §4 y ★1: no reserva, o libera la acción, y no suma gasto |
| Invalidación | §4: cambió la versión vigente desde que se escribió |
| Solicitud administrativa sin LLM | No se llama a `Reservar` (regla 7) |
| Concurrencia | §2.2, `ON CONFLICT … WHERE`: entra 1 de 3 y las otras 2 reciben 429. Test con goroutines |
| Alerta al 80 % | §2.4: 31,90 → 32,10, una sola notificación |
| Corte al 100 % | §2.2, paso 1; ★5: `429 presupuesto_agotado` |
| Caché con presupuesto agotado | §4: la caché se consulta antes de reservar (D16) |
| Reinicio mensual | §2.3, con reloj inyectable: el 30/31 y el día 1 a las 00:00 de Lima (05:00 UTC) |
| Contador tras borrado de datos | `ia_cuotas` no tiene FK ni `usuario_id` |
| Contador sin datos personales | §6, §7 |
| Auditoría | §3.3 |
| Bajar el presupuesto | §2.3 y §2.4: de 40 a 24 con 20 gastados, la alerta sale al guardar; si baja a ≤ gasto, el corte es inmediato |

## 9. Partición en tareas (1–3 h cada una)

### backend-dev-2

| # | Tarea | Depende de | h |
|---|---|---|---|
| K1 | Migración 068, Up y Down, con `up → down → up`. `ANTIFRAUD_HMAC_KEY` en `config.Load`, con tests (ADR 0002), si E4-01 no llegó antes | 066 | 2 |
| K2 | `costos.go`: la clave HMAC se pide a **`internal/antifraude`**, un paquete nuevo y único que arma las claves (ADR 0002: «ningún otro paquete arma claves»). Su forma real (PR #202) es el tipo `antifraude.Claves`, que se construye una vez con las claves de la configuración, con el método `(*Claves).Clave(prefijo, usuarioID)`. Ese método da la clave vigente y la anterior durante una rotación; la firma exacta es la del #202. E4-01 lo reutilizará con `diag_cooldown`. La tabla `ia_cuotas` y la reserva quedan en `ia`. El resto de la tarea: ventanas de Lima (`time/tzdata`, reloj inyectable), `Reservar` (orden D16), `Liberar` y `Consumo`, con rotación. Tests: dentro del límite, día, mes, admin, todo o nada con n > 1, concurrencia, reinicio diario y mensual, persistencia tras borrar datos, y precedencia del código cuando se agotan varios límites | K1 | 3 |
| K3 | Presupuesto: ★5 y ★9, alerta por valor (§2.4) y notificación. Tests: corte al 100 %, 31,90 → 32,10 con una sola notificación, subir y bajar el presupuesto rearman la alerta (y la envían si el gasto ya está en ≥ 80 %), bajar a ≤ gasto corta, costo de una llamada fallida, y que el gasto no baja al borrar filas de `ia_usos` | K1 | 3 |
| K4 | `cache.go`: clave, vigencia, escritura, purga y `BuscarEnCache` con su fila en `ia_usos`. Tests: caché, invalidación por versión nueva y por retiro, caché con el presupuesto y el límite agotados, y explicación sobre una versión citada anterior | K2, K3 | 3 |
| K5 | Rutas de §3, `ia.Codigo` (§5) y auditoría del `PUT`. Tests: rangos de D19, `dec2`, `403`, auditoría solo de lo que cambió, `mes = null` para el admin | K2, K3 | 3 |

### devops

| # | Tarea | h |
|---|---|---|
| D2 | `ANTIFRAUD_HMAC_KEY` en `env.example` (renombrado en el #125), `docker-compose.yml`, `Makefile`, `docs/puesta-en-marcha.md` y Railway (sealed), según la ADR 0002 § Archivos. Coordinado con K1 | 1 |

### frontend-dev (panel de admin; sin pantalla del postulante, D21)

| # | Tarea | h |
|---|---|---|
| F1 | Cliente `src/lib/serums-ia/ia/api.ts` (`configuracion`, `guardarConfiguracion`, `gasto` y, para el futuro, `consumo`), con el manejo de errores del README §3 | 1 |
| F2 | Panel de IA: formulario de límites y presupuesto (formik + yup con los rangos de D19) y tarjeta de gasto (USD en el formato del locale, porcentaje con barra de progreso, estado de alerta o agotado y tabla por operación). Estados de carga, vacío y error. Claves i18n es/en, incluidos los tres 429 | 3 |

- Todo con componentes y tokens de khipu: formulario, tarjeta, progreso y tabla. Si falta alguno, o la receta de moneda, va primero un PR a khipu.
