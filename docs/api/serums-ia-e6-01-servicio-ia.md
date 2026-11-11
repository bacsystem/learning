# Contrato interno — SERUMS IA, SIA-E6-01 Servicio de IA

- **Historia:** [SIA-E6-01](../stories/serums-ia/E6-01-servicio-ia.md)
- **Estado:** borrador para revisión del tech-lead (2026-11-11)
- **ADR:** [0006](../adr/0006-puerto-servicio-ia.md)
- **Tipo:** servicio Go **sin rutas HTTP propias**. Lo consumen E3-04, E6-03, E6-06, E6-08, etc. Los códigos HTTP de §5 son los que **deben** usar esos consumidores al exponer un error del servicio.
- **Migración reservada:** **066** (`ia_usos`).
- **Sin proveedor real:** E6-01 entrega el puerto, el registro, la redacción y el **doble determinista**. Elegir proveedor, modelo y prompts es SIA-E6-02, con la aprobación del usuario. No se agrega ningún SDK ni dependencia.

## 1. Paquete `internal/ia`

| Archivo | Contenido |
|---|---|
| `ia.go` | Tipos públicos, errores y la interfaz `Servicio` |
| `proveedor.go` | Puerto `Proveedor` (lo que implementa cada adaptador) |
| `servicio.go` | Orquestación: límites, redacción, timeout, reintentos, validación de la salida y registro |
| `redaccion.go` | Detección de datos personales (regla 3) |
| `operaciones.go` | Entrada y salida de cada operación, con `Validar()` |
| `doble.go` | Adaptador determinista (regla 7) |
| `registro.go` | Escritura en `ia_usos` |

### Interfaz para consumidores

```go
type Actor struct {
    Rol string // "admin" | "usuario" | "sistema"
    ID  *int64 // id interno de usuarios; nil = sistema. Nunca nombre, email ni DNI
}

type Servicio interface {
    Clasificar(ctx context.Context, a Actor, in EntradaClasificar) (SalidaClasificar, error)
    GenerarPreguntas(ctx context.Context, a Actor, in EntradaGenerarPreguntas) (SalidaGenerarPreguntas, error)
    GenerarMaterial(ctx context.Context, a Actor, in EntradaGenerarMaterial) (SalidaGenerarMaterial, error)
    GenerarExplicacion(ctx context.Context, a Actor, in EntradaGenerarExplicacion) (SalidaGenerarExplicacion, error)
}
```

- **Entradas y salidas tipadas**, una por operación (regla 1).
- Cada entrada separa las **instrucciones**, que son constantes del código, de los **datos**, que son contenido académico y pasajes de fuentes. Los datos viajan al proveedor **delimitados como datos**, nunca concatenados a las instrucciones (sección Seguridad de la historia; SIA-E2-03 regla 5).
- **Ids opacos:** las entradas referencian preguntas, pasajes, temas y áreas solo por id.
- **Campos de cada operación:** E6-01 fija solo el mínimo para que el doble y los tests funcionen. Por ejemplo, `EntradaClasificar{Items []ItemTexto{ID int64; Texto string}; Taxonomia []Opcion}` y `SalidaClasificar{Resultados []Clasificacion{ID int64; AsignaturaID int64; TemaID *int64; Confianza float64}}`. Los amplía cada historia consumidora sin romper el puerto, y backend-dev-2 los propone en su PR.

### Puerto del proveedor

```go
type Solicitud struct {
    Operacion     string     // "clasificar" | "generar_preguntas" | "generar_material" | "generar_explicacion"
    Instrucciones string     // plantilla fija del código
    Datos         []Bloque   // contenido académico, delimitado por el adaptador
    EsquemaSalida string     // nombre del esquema esperado, p. ej. "clasificar.v1"
}
type Bloque struct{ Etiqueta, Texto string }

type Respuesta struct {
    JSON          []byte
    TokensEntrada int
    TokensSalida  int
    Modelo        string
}

type Proveedor interface {
    Nombre() string
    Completar(ctx context.Context, s Solicitud) (Respuesta, error)
    CostoUSD(r Respuesta) float64 // tabla de precios del adaptador; 0 en el doble
}

// Error que el adaptador marca como transitorio (red, 429, 5xx) para reintentar.
type ErrorTransitorio struct{ Causa error }
```

## 2. Flujo de cada llamada

| Paso | Regla | Si falla |
|---|---|---|
| 1 | Servicio habilitado (§3) | `ErrNoDisponible`, sin llamar al proveedor |
| 2 | Tamaño: la suma de runes de todos los textos de `Datos` ≤ **30 000** | `ErrEntradaDemasiadoGrande` |
| 3 | Redacción (§4) sobre **todos** los textos de entrada | `ErrDatosPersonales`, sin llamar al proveedor |
| 4 | Llamada con `context.WithTimeout` de **60 s** por intento | Si vence: `ErrTimeout` (sin reintentar el timeout) |
| 5 | Si el adaptador devuelve `ErrorTransitorio`: hasta **2** reintentos, con espera creciente (1 s y 2 s, inyectable) | Si se agotan: `ErrProveedor` |
| 6 | Decodificar `JSON` con `DisallowUnknownFields` en la salida tipada y llamar a su `Validar()`. `validator/v10` ya es dependencia directa (`go.mod:8`), así que no hace falta una librería de JSON Schema | Si no cumple: **1** reintento completo. Si persiste, `ErrSalidaInvalida` |
| 7 | Registrar el uso (§6) | Si falla el registro: se loguea el `op`. **No** invalida una salida correcta |

- **Peor caso, fijo:** 2 intentos por validación × 3 intentos de transporte = **6 llamadas** como máximo. Con el timeout de 60 s por llamada, son **≤ 6 min** más esperas.
- Los timeouts y las esperas se inyectan en el constructor, para que los tests no esperen.
- **Restricción para consumidores:** el servidor HTTP tiene `WriteTimeout` de **15 s** (`cmd/server/main.go`, `http.Server`). **Ninguna** llamada de IA puede ir dentro de un request síncrono. Los consumidores la ejecutan en un job o worker (patrón de reclamación de `internal/notifier/email_worker.go`) y exponen el estado por polling. Es un requisito para E3-04, E6-03 y siguientes, no algo que el servicio resuelva.

## 3. Configuración (regla 5)

| Variable | Valores | Notas |
|---|---|---|
| `IA_PROVEEDOR` | vacío \| `doble` \| *nombre del adaptador real* (lo define E6-02) | Vacío = **deshabilitado**: toda operación da `ErrNoDisponible` y el resto de la aplicación funciona |
| `IA_API_KEY` | secreto | Solo con un adaptador real. Va como **sealed variable** (ADR 0004). Nunca en el código ni en los logs |
| `IA_MODELO` | texto | Solo con un adaptador real |

- **El doble falla cerrado:** `IA_PROVEEDOR=doble` solo se acepta si `ENV` está en la lista permitida `development`, `test` o `preprod`. Con cualquier otro valor, incluido `production`, o uno mal escrito, `config.Load` **falla al arrancar**. Es el mismo criterio que `config.go:109-113`: el doble nunca sirve contenido ficticio a alumnos reales. El valor exacto de `ENV` en Railway lo confirma devops.
- Un adaptador real sin `IA_API_KEY` también falla al arrancar. Con un nombre desconocido, igual.

## 4. Redacción de datos personales (regla 3)

Se **rechaza**, no se enmascara: la entrada que coincida no sale del servidor.

| Patrón | Expresión (RE2, sin lookbehind) |
|---|---|
| DNI | `\d{8}`, con **frontera de dígito** (ver abajo) |
| Email | `[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}` |
| Teléfono móvil | `(?:\+?51[\s-]?)?9\d{2}[\s-]?\d{3}[\s-]?\d{3}`, con frontera de dígito |
| Teléfono fijo | `\(0\d{1,2}\)\s?\d{6,7}` |

**Frontera de dígito** (corrección H3 del review):
- Go usa RE2, sin lookbehind, y `\b` falla cuando el número va pegado a `+` o a letras. Por eso los patrones **no** usan `\b`.
- En su lugar, el código comprueba cada coincidencia: el rune **anterior** al inicio y el **posterior** al final no deben ser dígitos, o la coincidencia debe estar al principio o al final del texto.
- Para el teléfono, se prueba primero con el prefijo `51` y después sin él, para que `51987654321` no se lea como un número de 11 dígitos.
- **Búsqueda (corrección H5 del re-review):** **no** se usa `FindAll` descartando candidatos. Un candidato rechazado puede solaparse con el número real y taparlo: en `Ref 12345678901 987654321`, el primer candidato móvil es `901 987654`; se descarta porque lo precede un dígito, y `FindAll` seguiría **después** de él, perdiendo `987654321`. El algoritmo es, para cada patrón:
  1. `loc := re.FindStringIndex(texto[pos:])`. Si no hay, termina.
  2. Si `loc` cumple la frontera de dígito, es una detección: se registra (o se enmascara) y `pos` avanza al **final** de la coincidencia.
  3. Si no la cumple, `pos` avanza **un rune** desde el **inicio** del candidato (`inicio + 1`) y se vuelve al paso 1.

- **Plegado de dígitos (decisión del architect, 2026-11-11):** en Go, `\d` solo reconoce los dígitos ASCII. Sin plegado, `１２３４５６７８` (ancho completo) o `١٢٣٤٥٦٧٨` (arábigo-índico) pasarían sin detectarse. Por eso, **antes** de detectar, cada rune de la categoría Unicode `Nd` se reemplaza por su dígito ASCII.
  - Con la librería estándar (`unicode.Nd`), **sin NFKC** y sin dependencias nuevas. `golang.org/x/text` es solo indirecta (`go.mod:44`), y hacerla directa necesita aprobación.
  - El valor es `(r − Lo) % 10` del rango de `unicode.Nd` que contiene `r`. Unicode garantiza que los dígitos `Nd` van en bloques contiguos de 0 a 9.
  - La frontera de dígito y la búsqueda de abajo corren sobre el texto plegado, así que la comprobación ASCII de la frontera ya cubre los demás sistemas.
  - `Enmascarar` devuelve el texto plegado y enmascarado: los dígitos no ASCII salen como ASCII, lo que no cambia el contenido académico.
  - **Riesgo aceptado** (ADR 0006): no se pliegan las letras ni `＠` de ancho completo, ni se quitan los caracteres invisibles (p. ej. U+200B entre dígitos). Un correo o un DNI ofuscados a propósito pueden pasar. La entrada viene de fuentes y preguntas que carga un admin, no de un atacante.
  - Tests: `１２３４５６７８`, `٩٨٧٦٥٤٣٢١` y `𝟗𝟖𝟕𝟔𝟓𝟒𝟑𝟐𝟏` se detectan, y la frontera vale con dígitos mixtos (`1２345678` es DNI; `１２３４５６７８９` no lo es).

  Detectar y `Enmascarar` usan esta misma función. No se usa la alternativa de capturar la frontera dentro del patrón (`(?:^|\D)(…)(?:\D|$)`), porque consume el separador y perdería el segundo de dos números separados por un solo carácter, como `987654321 987654322`.

| Entrada | ¿Se detecta? |
|---|---|
| `+51987654321`, `51987654321`, `+51 987 654 321`, `987654321`, `987-654-321` | sí (teléfono) |
| `1987654321`, `9876543210` | no (forman parte de un número más largo) |
| `12345678`, `DNI12345678`, `A12345678`, `12345678B` | sí (DNI: no hay dígitos al lado) |
| `123456789`, `20261111` incluido en `202611110` | no (son 9 dígitos) |
| `20261111` suelto | sí (falso positivo aceptado) |
| `Ref 12345678901 987654321` | sí, `telefono` (`987654321`). Falla con una implementación ingenua de `FindAll` (H5) |
| `987654321 987654322` | sí, los **dos** (`Enmascarar` reemplaza ambos) |

- **Un DNI no se distingue de otro número de 8 dígitos**, y no se intenta: todo número de 8 dígitos aislado se trata como posible DNI y se rechaza (la historia dice «un número de 8 dígitos aislado como DNI»). Que esté «aislado» significa que **no tiene dígitos al lado**: `123456789` no coincide, pero `DNI12345678`, `A12345678` y `12345678B` sí, porque así suele escribirse un DNI junto a su etiqueta (corrección H3). El precio es algún falso positivo, como una fecha `20261111`, y es aceptado (ADR 0006).
- Se evalúa sobre `Instrucciones` y sobre cada `Bloque.Texto`.
- El error **no** incluye el texto que coincidió: solo el tipo (`dni`, `email`, `telefono`) y el índice del bloque, para que el consumidor sepa qué revisar. Ni el log ni `ia_usos` guardan el texto.
- **Documentos oficiales:** las normas MINSA traen teléfonos y correos de contacto, y pasar esos pasajes completos haría fallar la llamada. Decisión del PO (§7): el consumidor los enmascara antes de llamar.

## 5. Errores y mapeo HTTP para consumidores

| Error Go | Código (formato 2) | HTTP | Resultado en `ia_usos` |
|---|---|---|---|
| `ErrNoDisponible` | `ia_no_disponible` | 503 | `no_disponible` |
| `ErrEntradaDemasiadoGrande` | `entrada_demasiado_grande` | 422 | `entrada_demasiado_grande` |
| `ErrDatosPersonales` | `datos_personales` | 422 | `datos_personales` |
| `ErrTimeout` | `ia_timeout` | 504 | `timeout` |
| `ErrSalidaInvalida` | `salida_invalida` | 502 | `salida_invalida` |
| `ErrProveedor` (no transitorio, o transitorio agotado) | `ia_error` | 502 | `error_proveedor` |
| `ErrCancelado`, que envuelve `context.Canceled`: el **consumidor** canceló su ctx | — (el consumidor lo provocó; no se mapea a HTTP) | — | `cancelado` |
| `ErrActorInvalido`: admin o usuario sin ID, sistema con ID, o rol desconocido. Es un error de programación | `error_interno` | 500 | **no** se registra, y no se llama al proveedor |
| — (correcto) | — | — | `ok` |

- **Cancelación frente a timeout:** si vence el timeout propio de 60 s, se devuelve `ErrTimeout` (`timeout`). Si el ctx del consumidor se cancela, se devuelve `ErrCancelado` (`cancelado`). En los dos casos la fila de uso se escribe con un ctx desacoplado (`context.WithoutCancel`), con los tokens consumidos hasta ese momento.
- **Actor inválido:** se valida al entrar, antes de cualquier otra regla. Un `actor_id` con forma válida pero inexistente solo hace fallar la FK del registro: se loguea el `op` y no invalida la salida (paso 7).
- **El mapeo es código, no solo contrato:** `internal/ia` exporta `Codigo(err error) (codigo string, http int)` con esta tabla, para que E3-04, E6-03 y los demás no la dupliquen. Devuelve `("", 0)` para `nil` y para `ErrCancelado`.

Cuando un consumidor trabaja en un job asíncrono, guarda el código en el estado del job en lugar de responder con ese HTTP.

## 6. Registro de uso — migración 066 (regla 2)

```sql
-- +goose Up
CREATE TABLE ia_usos (
    id              BIGSERIAL     PRIMARY KEY,
    operacion       VARCHAR(30)   NOT NULL CHECK (operacion IN
                    ('clasificar', 'generar_preguntas', 'generar_material', 'generar_explicacion')),
    actor_rol       VARCHAR(10)   NOT NULL CHECK (actor_rol IN ('admin', 'usuario', 'sistema')),
    actor_id        BIGINT        REFERENCES usuarios(id),
    proveedor       VARCHAR(30)   NOT NULL,              -- 'doble' o el adaptador; '' si no_disponible
    modelo          VARCHAR(80),
    tokens_entrada  INT           NOT NULL DEFAULT 0 CHECK (tokens_entrada >= 0),
    tokens_salida   INT           NOT NULL DEFAULT 0 CHECK (tokens_salida >= 0),
    costo_usd       NUMERIC(12,6) NOT NULL DEFAULT 0 CHECK (costo_usd >= 0),
    duracion_ms     INT           NOT NULL CHECK (duracion_ms >= 0),
    intentos        SMALLINT      NOT NULL DEFAULT 0 CHECK (intentos BETWEEN 0 AND 6),
    resultado       VARCHAR(30)   NOT NULL CHECK (resultado IN ('ok', 'no_disponible',
                    'entrada_demasiado_grande', 'datos_personales', 'timeout', 'cancelado', 'salida_invalida', 'error_proveedor')),
    desde_cache     BOOLEAN       NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ia_usos_actor CHECK ((actor_rol = 'sistema') = (actor_id IS NULL))
);
CREATE INDEX idx_ia_usos_created   ON ia_usos (created_at);
CREATE INDEX idx_ia_usos_actor     ON ia_usos (actor_id, created_at) WHERE actor_id IS NOT NULL;
CREATE INDEX idx_ia_usos_operacion ON ia_usos (operacion, created_at);

-- +goose Down
DROP TABLE IF EXISTS ia_usos;
```

- Hay **una fila por invocación**, también si se rechaza antes de llamar, si vence o si la salida es inválida. Los tokens y el costo suman los de todos los intentos.
- **Nunca** guarda el prompt, la respuesta, el texto detectado ni mensajes de error.
- Se escribe con el `*sqlx.DB` del servicio, **fuera** de la transacción del consumidor. Así el uso queda registrado aunque el consumidor revierta su operación.
- `desde_cache` existe por la regla 2, pero E6-01 **no** implementa caché (YAGNI): siempre es `false` hasta que una historia la pida.
- Es tabla nueva, sin efecto sobre los datos existentes. El Down la borra entera, perdiendo el historial de uso.
- **Datos personales:** `actor_id` enlaza con `usuarios`, así que la exportación y el borrado de SIA-E9-07 la incluyen.

## 7. Decisión del PO

| # | Pregunta | Decisión por defecto |
|---|---|---|
| Q-IA1 | Los pasajes de documentos oficiales traen teléfonos y correos de contacto, y la regla 3 rechaza toda la llamada. ¿Se mantiene el rechazo, o se enmascaran antes de enviar? | **Decidida por el PO:** E6-01 rechaza con `datos_personales` y **nunca** enmascara por su cuenta. Quien arma entradas con contenido de fuentes o de preguntas (E3-04, E6-03, E6-06, E6-08) **enmascara antes de llamar** los emails, teléfonos y números de 8 dígitos, reemplazándolos por un marcador. Nunca se envían datos de usuarios. El PO lo deja escrito como supuesto del README de historias. Para que todos enmascaren igual, `internal/ia` **exporta** una función pura `Enmascarar(texto) string` con los mismos patrones de §4, que esos consumidores usan; el servicio no la llama |

## 8. Doble determinista (regla 7)

- Sin red. Salida derivada solo de la entrada: por ejemplo, `Clasificar` asigna a cada item la primera opción de la taxonomía, con confianza 0,5.
- Tokens = runes / 4; costo 0; modelo `"doble"`.
- **Modos de fallo** para los tests de los criterios, activados por una opción del constructor del doble (no por la entrada): `SalidaInvalida`, `Lento` (bloquea hasta que vence el ctx) y `Transitorio(n)` (falla n veces y después responde bien).

## 9. Partición en tareas (backend-dev-2, 1–3 h cada una)

| # | Tarea | h |
|---|---|---|
| A1 | `ia.go` y `proveedor.go`: tipos, errores y puerto; configuración en `config.Load` con la lista de `ENV` permitidos para el doble | 2 |
| A2 | `redaccion.go` y el límite de 30 000, con tests de positivos y falsos positivos (8 dígitos dentro de un número mayor, un año, un correo). También `Enmascarar(texto)`, que es exportada, usa los mismos patrones y es idempotente | 2 |
| A3 | `servicio.go`: timeout, reintentos transitorios, reintento por salida inválida, peor caso de 6 llamadas, y tiempos inyectables | 3 |
| A4 | Migración 066 y `registro.go`, fuera de la transacción del consumidor; una fila por invocación con su resultado | 2 |
| A5 | `doble.go` con sus modos de fallo, y un test por cada escenario de la historia | 3 |
| A6 | Test de logs: con un logger de captura, ninguna línea contiene la entrada, la salida ni el patrón detectado | 1 |
| D1 | devops: `IA_PROVEEDOR`, `IA_API_KEY` e `IA_MODELO` en `deploy/` y en `docs/ops`. Vacío en producción hasta E6-02; `doble` en preproducción | 1 |
