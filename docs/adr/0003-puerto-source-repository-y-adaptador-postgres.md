# ADR 0003: puerto `SourceRepository` y adaptador propio sobre PostgreSQL

- **Estado:** aceptado (2026-11-11). `texto.Plegar` ya está implementado (E1-01); el resto se implementa con E2. Revisado el 2026-11-11: la sección «Plegado» añade las formas descompuestas (NFD); «El puerto» define los tipos que faltaban (`NuevaFuente`, `FiltroFuentes`, `Pagina`, `FuenteResumen`, `Pasaje`), reemplaza `audit.TxFunc` por un `Auditor` que recibe el `Evento` con el id de la fuente (también en `GuardarFuente`), fija la normalización de `Page` y `PerPage` y el error de `EspecialidadID ≤ 0`; la suite de contrato recibe los ganchos de prueba `Retirar` y `NuevaVersion`, y se fija el `detalle` de `fuente.procesar` y `fuente.fallar`.
- **Fecha:** 2026-11-11
- **Historias:** SIA-E2-02 (#16); la usan SIA-E2-03, E2-05, E2-06, E2-07 y, más adelante, E3-01, E6-03, E6-04 y E6-08
- **Decisión de producto de origen:** P1 (issue #65). MVP y v2 usan el 100 % del `SourceRepository` propio en PostgreSQL, a $0 USD. La búsqueda semántica (pgvector o NotebookLM) se reevalúa en v3 (SIA-E2-01, SIA-E2-08).

## Contexto

Las citas de todo contenido generado apuntan a (fuente, versión, pasaje) y deben poder resolverse siempre, incluso después de nuevas versiones (SIA-E2-02, reglas 1–2). La búsqueda devuelve como máximo 20 pasajes y considera solo fuentes `activa` de la profesión pedida. La consulta tiene entre 3 y 300 caracteres. No se agregan servicios ni dependencias externas (P1 y las reglas del equipo).

Lo que ya existe en la base de datos y en el backend:

- La única extensión instalada es `pg_trgm` (`backend/migrations/013_create_etiquetas_pregunta.sql:2`). Se usa para `ILIKE` sobre enunciados (`014_create_preguntas.sql:27`). No hay `unaccent` ni `tsvector` en el repo.
- Postgres 16 (`docker-compose.yml:24`, `.github/workflows/backend.yml:31`) trae la configuración de texto `spanish` (diccionario Snowball) y `websearch_to_tsquery` sin extensiones.
- Patrón de worker con reclamación atómica: `internal/notifier/email_worker.go:146-170` (`FOR UPDATE SKIP LOCKED` y `claimed_at` con reclamación de reclamaciones viejas), lanzado desde `cmd/server/main.go:149-157`.

## Opciones consideradas para la búsqueda en el MVP

1. **Full-text search de Postgres con configuración `spanish` sobre texto plegado en Go.** Es la elegida. Tiene stemming en español y ranking (`ts_rank_cd`), y lo cubre un índice GIN. No requiere extensión.
2. **FTS `spanish` con `unaccent` (extensión contrib).** Ignora tildes dentro de Postgres. Exige crear la extensión y una configuración de texto propia. **No se aprueba por ahora** (tech-lead, TL2, 2026-11-11). Si hiciera falta, se escribe un ADR nuevo y se pide aprobación al usuario.
3. **Similitud de trigramas (`pg_trgm`, ya instalada).** No tiene stemming y su ranking es malo en pasajes de 150–800 palabras. Descartada para búsqueda; sigue bien para títulos.
4. **pgvector o NotebookLM.** Fuera del MVP por P1; v3.

## Decisión

### Paquete y responsabilidades

Paquete nuevo `backend/internal/fuentes` con la convención del repo (`handler.go`, `service.go`, `repository.go`, `dto.go`, `errors.go`, `routes.go` con `Mount`). Dentro:

- `source.go`: el **puerto** `SourceRepository` y sus tipos. No importa nada de HTTP ni de `auth`.
- `pgsource.go`: el **adaptador propio** sobre Postgres (`*sqlx.DB`).
- `fuentestest/contract.go`: la **suite de contrato** `RunContract(t, nuevo func(t *testing.T) Entorno)`, donde:

  ```go
  type Entorno struct {
      Repo                         SourceRepository
      EspecialidadA, EspecialidadB int64
      // Ganchos de prueba: lo que la suite necesita y el puerto no ofrece.
      Retirar      func(t *testing.T, fuenteID int64)                                // fuente → 'retirada'
      NuevaVersion func(t *testing.T, fuenteID int64, contenido []byte) (version int) // crea la versión n+1 en 'procesando'
  }
  ```

  - Con Postgres las especialidades deben existir por la FK, así que la fábrica las siembra y entrega sus ids. El adaptador en memoria no valida que existan.
  - Se ejecuta sobre el adaptador Postgres y sobre `fuentestest/memoria.go` (adaptador en memoria de prueba). Cubre el escenario «Intercambiabilidad» de SIA-E2-02.
  - Siembra sus datos a través del puerto (`GuardarFuente` y luego `PublicarVersion`), así los dos adaptadores parten del mismo estado.
  - Los ganchos cubren lo que el puerto no hace porque es ciclo de vida admin (E2-03, E2-06):
    - «Fuente inactiva excluida» usa `Retirar`;
    - «Resolver cita histórica» usa `NuevaVersion` para crear las versiones 2 y 3, que luego publica con `PublicarVersion`.
  - En Postgres los ganchos son SQL del test; en memoria, métodos de `Memoria`. Viven solo en `fuentestest`: no forman parte de la interfaz `SourceRepository` ni de la API de producción. Cuando existan las operaciones reales de E2-03 y E2-06, se prueban con sus propios tests, no con estos ganchos.
- `normalizar.go`: normalización y segmentación deterministas (SIA-E2-05), sin IA y sin E/S.
- `worker.go`: worker de ingesta (abajo).
- El ciclo de vida admin (retirar, restaurar, reintentar, marca oficial) vive en `service.go` y `repository.go`, **fuera** del puerto: no es intercambiable por proveedor. Un adaptador v3 (SIA-E2-08) solo reemplazaría `BuscarPasajes`. Las citas se resuelven siempre con el adaptador propio (SIA-E2-08, regla 3).

### El puerto

```go
// Cita es inmutable: la versión de una cita nunca cambia (SIA-E2-02 regla 1).
type Cita struct {
    FuenteID int64
    Version  int   // número de versión (1..n), no el id interno
    PasajeID int64
}

type Busqueda struct {
    Consulta       string // 3–300 caracteres (runes) tras TrimSpace; si no, ErrConsultaInvalida
    EspecialidadID int64  // obligatorio (> 0); ≤ 0 → ErrConsultaInvalida
    AsignaturaID   *int64 // aceptado; el adaptador MVP no lo usa para filtrar (ver «Área y tema»)
    TemaID         *int64 // ídem
    Limite         int    // 1..20; 0 = 20; > 20 se recorta a 20
}

// NuevaFuente: quien llama ya validó los metadatos y saneó NombreArchivo (S1 del contrato E2).
type NuevaFuente struct {
    Titulo          string
    Tipo            string   // balotario | guia | norma | otro
    Anio            *int
    ProcesoID       *int64
    EspecialidadIDs []int64  // vacío = aplica a todas
    NombreArchivo   string
    Contenido       []byte   // original UTF-8; el adaptador calcula sha256, tamaño y líneas con Huella
    CreadoPor       *int64   // nil = sistema / prueba
}

// Huella calcula sha256 (hex), tamaño en bytes y líneas del contenido. Es la única
// implementación: la usan los adaptadores y E2-03 (duplicado por hash).
func Huella(contenido []byte) (sha256 string, tamanoBytes, lineas int)

type Pagina[T any] struct {
    Data    []T
    Total   int
    Page    int
    PerPage int
}

// FiltroFuentes. En EspecialidadID, Estado y Tipo, el valor cero = sin filtro.
// Page y PerPage NO siguen esa regla: el puerto los normaliza, sin error
// (Page ≤ 0 → 1; PerPage ≤ 0 → 20; PerPage > 100 → 100). La validación estricta
// de la query (400 parametro_invalido, README §5) la hace el handler HTTP antes de llamar.
type FiltroFuentes struct {
    EspecialidadID int64  // > 0: fuentes de esa especialidad más las que aplican a todas
    Estado         string // procesando | activa | fallida | retirada
    Tipo           string // balotario | guia | norma | otro
    Page, PerPage  int
}

// Evento de auditoría que arma el adaptador dentro de su transacción, cuando ya
// conoce el id de la fuente (mismo patrón que temas.Evento, internal/temas/service.go:12-21).
type Evento struct {
    Accion    string         // fuente.crear | fuente.procesar | fuente.fallar
    Entidad   string         // "fuente"
    EntidadID int64          // id de la fuente
    Detalle   map[string]any // sin contenido ni datos personales (README §7)
}

// Auditor escribe ev en la transacción de la mutación; si falla, la mutación se revierte.
// Quien llama (handler o worker) lo arma con el actor, la IP y el user agent (actor nil = sistema).
type Auditor func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error

// FuenteResumen: mínimo que cualquier adaptador puede dar (sin nombres de catálogo).
type FuenteResumen struct {
    ID              int64
    Titulo, Tipo    string
    Estado          string
    Anio            *int
    ProcesoID       *int64
    EspecialidadIDs []int64 // vacío = aplica a todas; orden ascendente
    VersionVigente  *int    // número de versión; nil si no hay vigente
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// Pasaje: mismos campos que PasajeEncontrado (abajo).
type Pasaje = PasajeEncontrado

type SourceRepository interface {
    // Crea la fuente y su versión 1 en 'procesando'. Llama a aud con fuente.crear y el id nuevo
    // en la misma transacción.
    GuardarFuente(ctx context.Context, f NuevaFuente, aud Auditor) (fuenteID int64, version int, err error)
    ListarFuentes(ctx context.Context, f FiltroFuentes) (Pagina[FuenteResumen], error) // UpdatedAt DESC, ID DESC
    // Orden ASC; page y perPage se normalizan igual que en FiltroFuentes.
    ObtenerPasajes(ctx context.Context, fuenteID int64, version int, page, perPage int) (Pagina[Pasaje], error)
    BuscarPasajes(ctx context.Context, b Busqueda) ([]PasajeEncontrado, error) // ≤ 20, por relevancia
    ResolverCita(ctx context.Context, c Cita) (PasajeCitado, error)

    // Publicación: la usa el worker y la suite de contrato la usa para sembrar datos.
    // Atómica: todos los pasajes y la vigencia, o nada. aud (fuente.procesar / fuente.fallar)
    // se escribe en la misma transacción. El adaptador en memoria no llama a aud.
    PublicarVersion(ctx context.Context, fuenteID int64, version int, pasajes []PasajeNuevo, aud Auditor) error
    MarcarVersionFallida(ctx context.Context, fuenteID int64, version int, motivo string, aud Auditor) error
}

var (
    ErrConsultaInvalida = errors.New("consulta_invalida")
    ErrCitaInvalida     = errors.New("cita_invalida")    // no existe o (fuente, versión, pasaje) no coinciden
    ErrNoEncontrado     = errors.New("no_encontrado")
)
```

- `PasajeEncontrado` y `PasajeCitado` llevan: `FuenteID`, `Version`, `PasajeID`, `Orden`, `LineaInicio`, `LineaFin`, `Seccion`, `Texto` y, en `PasajeCitado`, además `TituloFuente`, `EstadoFuente` y `OficialVerificada` de esa versión. No llevan ningún dato de usuario (regla 4).
- `ResolverCita` **no** filtra por estado de la fuente: una cita histórica a la versión 1 devuelve el texto exacto de la versión 1 aunque la fuente vaya por la 3 o esté `retirada` (escenario «Resolver cita histórica»). El consumidor decide qué hacer con `EstadoFuente`.
- **`ObtenerPasajes`** tampoco filtra por estado de la fuente. Si la fuente o la versión no existen, devuelve `ErrNoEncontrado`. Una versión sin pasajes (`procesando` o `fallida`) devuelve `Data` vacío. Normaliza `page` y `perPage` igual que `FiltroFuentes`.
- **`ListarFuentes` en el puerto, en versión mínima** (decisión del 2026-11-11, consulta de backend-dev al empezar E2-02). La historia SIA-E2-02 (§Contexto, línea 17) pone «listar fuentes» entre las operaciones del puerto, así que no se saca. Pero `FuenteResumen` solo lleva lo que cualquier adaptador puede dar: ids y no nombres de especialidad, y el número de la versión vigente. El listado de admin S3 del contrato E2 (nombres de especialidad, `version_en_proceso`, filtros `oficial` y `q`) es una consulta de `repository.go` fuera del puerto, que implementa E2-07, igual que el ciclo de vida admin.
- **Auditoría en el puerto (`Auditor` + `Evento`):** el adaptador arma el `Evento` dentro de su transacción. Solo ahí se conoce el `entidad_id` de una fuente nueva, y el contrato E2 lo exige (S1, «Respuesta 202»: `fuente.crear` con `entidad_id`). También arma el `detalle` de cada acción:

  | Acción | `detalle` |
  |---|---|
  | `fuente.crear` | `{"version", "sha256", "tamano_bytes", "tipo", "especialidad_ids", "estado_nuevo"}` |
  | `fuente.procesar` | `{"version", "pasajes_count", "estado_anterior", "estado_nuevo"}` |
  | `fuente.fallar` | `{"version", "motivo_fallo", "estado_anterior", "estado_nuevo"}` |

  `estado_anterior` y `estado_nuevo` son los de la **fuente**, no los de la versión (E2-06, escenario «Auditoría»). Al publicar la versión 2 de una fuente `activa` quedan `activa → activa`, y `version` distingue el caso. Quien llama solo aporta el actor, la IP y el user agent a través del `Auditor`, igual que `temas.Auditor` (`internal/temas/service.go:19-21`, usado en `temas/repository.go:197`). Un `audit.TxFunc` no basta, porque la closure se arma antes de conocer el id. Si el `Auditor` devuelve error, la mutación se revierte (README §7).
- **`GuardarFuente`** recibe el `Auditor` para escribir `fuente.crear` en la misma transacción (README §7). El adaptador en memoria no lo llama. Calcula `sha256`, `tamano_bytes` y `lineas` con `Huella`. E2-02 no valida ni el duplicado por hash ni el límite de fuentes. Los agrega E2-03 dentro de esa misma transacción, bajo el advisory lock del contrato E2 (S1, «Límite de fuentes»), y mapea la violación 23505 de `uq_fuente_versiones_sha256` a `fuente_duplicada`.

### Recuperación de pasajes (adaptador Postgres)

1. **Plegado en Go** con `texto.Plegar(s)` (`backend/internal/texto/texto.go`, creado por SIA-E1-01). Las reglas están en la sección «Plegado», más abajo. La misma función sirve para la clave de nombre de los temas (`docs/api/serums-ia-e1-taxonomia.md`).
2. **Al publicar un pasaje** se guarda `tsv = to_tsvector('spanish', plegar(texto))` en `pasajes.tsv`, con índice GIN. El texto original se guarda sin plegar en `pasajes.texto`, que es el que se cita. Los pasajes son inmutables, así que `tsv` no puede desincronizarse.
3. **Consulta:**

```sql
-- $1 = plegar(consulta), $2 = especialidad_id, $3 = límite (≤ 20)
WITH q AS (SELECT websearch_to_tsquery('spanish', $1) AS q)
SELECT f.id, v.numero, p.id, p.orden, p.linea_inicio, p.linea_fin, p.seccion, p.texto,
       ts_rank_cd(p.tsv, q.q) AS rank
FROM q
JOIN pasajes p          ON p.tsv @@ q.q
JOIN fuente_versiones v ON v.id = p.fuente_version_id
JOIN fuentes f          ON f.version_vigente_id = v.id
WHERE f.estado = 'activa'
  AND (NOT EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id)
       OR EXISTS (SELECT 1 FROM fuente_especialidades fe
                  WHERE fe.fuente_id = f.id AND fe.especialidad_id = $2))
ORDER BY rank DESC, p.id ASC
LIMIT $3;
```

- Solo entran pasajes de la **versión vigente** de fuentes `activa`. Las fuentes `retirada` y las versiones anteriores no aparecen (escenario «Fuente inactiva excluida»; SIA-E2-06 regla 4).
- Una fuente sin profesiones aplica a todas (SIA-E2-03 regla 2).
- `websearch_to_tsquery` no falla con entrada arbitraria (comillas, guiones, `or`). Si la consulta queda vacía (solo stopwords), el resultado es una lista vacía, no un error.
- Desempate por `p.id` para que el orden sea estable y testeable.
- Todo va parametrizado. El texto de la consulta y de los pasajes no se registra en logs, solo ids (SIA-E2-02, sección Seguridad).

### Plegado

`texto.Plegar` (`backend/internal/texto/texto.go:10-19`) produce la forma de comparación sin mayúsculas ni tildes. La usan la clave de nombre de los temas (`temas.nombre_clave`, E1) y el `tsv` y la consulta de los pasajes (E2).

**Reglas:**

1. Pasar a minúsculas (`strings.ToLower`).
2. Quitar las tildes `á é í ó ú ü à è ì ò ù`, venga la letra **compuesta** (NFC, un solo code point, p. ej. U+00E1) o **descompuesta** (NFD: vocal + marca combinante U+0301 aguda, U+0300 grave o U+0308 diéresis).
3. La `ñ` se conserva para no confundir «año» con «ano». La forma descompuesta `n` + U+0303 se convierte en la `ñ` compuesta (U+00F1), **no** se le quita la marca.
4. No recorta ni colapsa espacios: lo hace quien compara nombres.

**Problema detectado (review de E1-01):** la primera versión solo reemplaza las letras compuestas. Un nombre en NFD (frecuente al pegar desde un PDF o desde macOS) da otra clave que su forma NFC. Así, «Epidemiología» escrita de las dos formas no se detecta como duplicado, y una consulta en NFD no encuentra pasajes en NFC (E2).

**Opciones consideradas:**

| Opción | Evaluación |
|---|---|
| A. Normalizar a NFC con `golang.org/x/text/unicode/norm` antes de plegar | Cubre todo Unicode. `golang.org/x/text v0.43.0` ya está en `backend/go.mod:44`, pero solo como dependencia **indirecta**: usarla la vuelve directa, y por las reglas del equipo eso requiere aprobación del usuario. Más de lo que hace falta para el conjunto de letras que define `Plegar` |
| B. Añadir las secuencias descompuestas a la tabla de `strings.NewReplacer` | **Elegida.** Sin dependencias. Son 12 secuencias más, en minúscula porque el reemplazo corre después de `ToLower`: 5 vocales × 2 marcas (U+0301 y U+0300), más `u` + U+0308, más `n` + U+0303 → `ñ`. Da el mismo resultado que A para todas las letras que `Plegar` pliega |
| C. Quitar toda marca combinante (rango U+0300–U+036F) | Descartada: borraría la U+0303 de la `ñ` descompuesta («año» en NFD acabaría como «ano») y trataría distinto las letras fuera del conjunto según vengan en NFC o NFD |

**Decisión: opción B.** La implementa backend-dev en `internal/texto`:

- Tabla de reemplazo con las letras compuestas y las secuencias descompuestas. Como se aplica después de `ToLower`, basta con las minúsculas: `ToLower` no toca las marcas combinantes y baja la vocal base.
- Tests obligatorios (tabla en `texto_test.go`):
  - para cada letra del conjunto, en mayúscula y minúscula, `Plegar(NFC) == Plegar(NFD)`;
  - `Plegar("año" en NFD) == Plegar("año" en NFC) == "año"`, y distinto de `Plegar("ano")`;
  - en `internal/temas`, un nombre en NFD da `422 tema_duplicado` frente al mismo nombre en NFC.
- El texto original (`temas.nombre`, `pasajes.texto`) se guarda **tal como llega**. Solo la clave y el `tsv` se pliegan.

**Consecuencias:**

- Sin backfill: no existe base de datos de producción (confirmado por el usuario) y E2 aún no escribió pasajes. En una base local con temas creados antes del cambio, una `nombre_clave` antigua solo puede diferir si el nombre vino en NFD; basta con recrear la base de desarrollo.
- Límite aceptado: las letras fuera del conjunto (`ç`, `â`, `ã`…) no se pliegan, y su forma NFC y su forma NFD siguen dando claves distintas. En nombres de temas y pasajes en español es un caso marginal. Si alguna vez hiciera falta, la opción A se reabre con un ADR nuevo y con la aprobación del usuario (igual que `unaccent`, TL2).
- `Plegar` es la única función de plegado. Cualquier cambio a sus reglas cambia las claves guardadas, así que va con su propio ADR.

### Área y tema

Los metadatos de fuente (SIA-E2-03 regla 2) no tienen área ni tema, así que el adaptador MVP **no** filtra por ellos. Los recibe en `Busqueda` para que el contrato no cambie cuando se añadan. El consumidor (SIA-E6-03 o SIA-E6-06) incluye el nombre del área o del tema en `Consulta`. Lo confirmó el PO (SIA-E2-02 regla 6).

### Worker de ingesta

- Goroutine en `cmd/server`, igual que `emailWorker` (`main.go:149-157`), con apagado ordenado por `ctx`.
- Reclama **una versión** a la vez:

```sql
UPDATE fuente_versiones SET claimed_at = clock_timestamp(), intentos = intentos + 1
WHERE id = (SELECT id FROM fuente_versiones
            WHERE estado = 'procesando'
              AND (claimed_at IS NULL OR claimed_at < NOW() - INTERVAL '10 minutes')
            ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
RETURNING …;
```

  Con varias réplicas, ninguna procesa la misma versión.
- Normaliza y segmenta en memoria (el contenido es ≤ 2 MiB). Publica con `PublicarVersion` del puerto, que en **una sola transacción**:
  - inserta todos los pasajes con `tsv`;
  - marca la versión `lista`;
  - mueve `fuentes.version_vigente_id` y pone la fuente en `activa`;
  - escribe el `audit_log` con el `Auditor` del puerto (`fuente.procesar`). El worker lo arma sobre `audit.LogTx` (`internal/audit/audit.go:55-71`) con actor `NULL` = sistema.

  Si algo falla, no se publica ningún pasaje (SIA-E2-03 «Falla de procesamiento»; SIA-E2-05 regla 4). El archivado por el límite de 20 versiones **no** ocurre aquí, sino al subir la versión, en S2 del contrato E2: es el único punto que puede responder `422 limite_versiones`.
- Fallos deterministas (`sin_contenido`, `limite_pasajes`): la versión pasa a `fallida` con su `motivo_fallo` en la primera ejecución.
- Fallos transitorios (base de datos, pánico recuperado): la versión queda `procesando` y se reintenta al vencer la reclamación. A los 3 `intentos` pasa a `fallida` con motivo `error_interno`.
- Intervalo de sondeo: 2 s, para que la UI (que refresca cada 5 s, SIA-E2-07 regla 6) vea el cambio en el siguiente ciclo.

### Normalización y segmentación (SIA-E2-05)

Es determinista e idempotente: mismo contenido y misma `normalizador_version` dan los mismos pasajes en el mismo orden. Precisada el 2026-11-11 con las consultas de backend-dev-2 al empezar E2-05.

**Interfaz.** `func Normalizar(contenido []byte) ([]PasajeNuevo, error)` en `internal/fuentes/normalizar.go`. Es pura, sin E/S y sin IA.

- Errores: `ErrSinContenido` y `ErrLimitePasajes`. El worker los registra con `MarcarVersionFallida` como `sin_contenido` y `limite_pasajes`.
- `const NormalizadorVersion = 1`.

**Líneas (regla única).** `\r\n` cuenta como **un** salto; `\r` suelto y `\n`, como un salto cada uno.

- Un salto al final no agrega una línea vacía, y el contenido vacío tiene 0 líneas. Ejemplos: `"a\r\nb"` → 2 líneas, `"a\r\rb"` → 3, `"a\n"` → 1.
- La implementa **una sola** función, `fuentes.DividirLineas(contenido []byte) []string` (PR #92). `fuentes.ContarLineas` es `len(DividirLineas(…))`.
  - `ContarLineas` la usan `Huella` (`fuente_versiones.lineas`) y la validación de ≤ 20 000 líneas de S1.
  - `Normalizar` corta el contenido con `DividirLineas`.
- Así `linea_inicio` y `linea_fin` (base 1) siempre caen en `1..lineas` de su versión.

**Palabras.** Antes de contar se quitan `<mark>` y `</mark>`. Después cuenta cada token de `strings.Fields` que tenga al menos una letra o un dígito (`unicode.IsLetter` o `unicode.IsDigit`).

- Viñetas (`-`), marcas de encabezado (`#`), `|` y `—` sueltos no cuentan.
- Esta definición vale para todos los umbrales (150 y 800) y para `pasajes.palabras`.

1. **Limpieza.**
   - Se quita el BOM UTF-8.
   - Los bytes que no son UTF-8 válido se reemplazan por U+FFFD (`strings.ToValidUTF8`). Es solo defensivo: S1 ya rechaza el archivo que no es UTF-8.
   - Los saltos pasan a `\n` según la regla de líneas.
   - Se eliminan los caracteres de control salvo `\n` y `\t`, y `\t` pasa a espacio.
   - Se colapsan los espacios repetidos dentro de cada línea.
2. **Viñetas unificadas.** Al inicio de línea, con sangría opcional, `*`, `+`, `•`, `▪` o `◦` seguidos de **al menos un espacio** pasan a `- `. Sin el espacio no es viñeta: «**negrita**» queda igual.
3. **Marcas.** `<mark>` y `</mark>` se conservan literalmente. No se interpreta ningún otro HTML: el texto es dato, no instrucción.
4. **Bloques de código** (entre líneas que empiezan con ` ``` `). Dentro no se detectan encabezados ni se unifican viñetas; sí se aplica la limpieza del paso 1. Un bloque sin cerrar llega hasta el final del documento.
5. **Estructura.**
   - Encabezado: línea que empieza con `#`…`######` seguido de un espacio, fuera de un bloque de código.
   - Párrafo: líneas consecutivas separadas por una o más líneas en blanco.
   - Texto del pasaje: las líneas normalizadas unidas con `\n` y los párrafos con `\n\n`. Incluye la línea del encabezado.
   - `seccion`: texto del encabezado del primer bloque del pasaje, sin los `#`, con `TrimSpace` y ≤ 300 runes. Vale `nil` para el texto anterior al primer encabezado.
6. **Segmentación** (SIA-E2-05 regla 2).
   - Los bloques se cortan en encabezados.
   - Unidades de corte, en este orden de preferencia: párrafo; si un párrafo pasa de 800 palabras, sus oraciones (cortes tras `.`, `?` o `!` seguidos de espacio o fin); si una oración pasa de 800, **partes equilibradas por palabras**. Esas partes son k = ⌈n/800⌉ trozos de ⌊n/k⌋ o ⌈n/k⌉ palabras, los mayores primero; con n > 800, cada trozo tiene al menos 400 (p. ej. n = 801 → 401 y 400).
   - Greedy: se acumulan unidades mientras el pasaje no pase de 800.
   - Un fragmento de menos de 150 palabras (sección corta o resto final) se une al pasaje **anterior**, o al **siguiente** si es el primero. Como la dirección es fija, el resultado es determinista.
   - Si al unirse el conjunto pasa de 800 (queda entre 801 y 949), se parte en **dos**:
     - por el límite de párrafo más cercano a la mitad, si los dos lados quedan con ≥ 150 palabras;
     - si no, por el límite de oración más cercano a la mitad, con la misma condición;
     - si no, por palabras a la mitad.

     Siempre cierra, porque cada mitad queda con al menos 400 palabras (≥ 150).
   - Cada pasaje guarda `orden`, `linea_inicio`, `linea_fin`, `seccion` y `palabras`.
   - La unión no se aplica a los pasajes de balotario, que van uno por pregunta (SIA-E3-01).
   - Test de propiedad obligatorio: en todo documento de 150 palabras o más, cada pasaje tiene entre 150 y 800.
7. **Excepción** (decisión del PO): un documento entero de 1 a 149 palabras se publica como un único pasaje (escenario «Documento corto»). Si no tiene ninguna palabra, `ErrSinContenido` (`fallida` con `sin_contenido`). Con más de 5 000 pasajes, `ErrLimitePasajes` (`fallida` con `limite_pasajes`).
8. **Alternativas pegadas** («a) … b) … c) …» en una línea): **no** se separan en el MVP. La regla 1 de SIA-E2-05 no lo pide y la heurística da falsos positivos (p. ej. «vitamina b) …»). La DoD las cubre como caso de test: el texto se conserva igual y el resultado es determinista. La separación por pregunta y alternativa de los balotarios llega con SIA-E3-01.
9. Los balotarios se segmentan por pregunta en SIA-E3-01. Hasta entonces usan esta segmentación genérica. `normalizador_version` (`SMALLINT`, hoy 1) distingue los pasajes generados por cada algoritmo. Una versión nueva del algoritmo no reescribe pasajes ya citados.

## Consecuencias

- **Positivas:**
  - $0 USD y sin dependencias ni extensiones nuevas (el plegado NFD tampoco agrega ninguna; ver «Plegado»).
  - Las citas no dependen de la búsqueda, así que un adaptador v3 no puede romperlas.
  - Ingesta segura con varias réplicas, publicación atómica y una suite de contrato que fija el comportamiento.
- **Negativas:**
  - Las tildes se pliegan en Go, así que cualquier escritor de `pasajes.tsv` debe usar `plegar`. Solo el worker escribe en esa tabla.
  - Las stopwords del diccionario `spanish` están acentuadas y algunas variantes sin tilde no se filtran (efecto menor en el ranking).
  - Sin búsqueda semántica: un sinónimo no encuentra el pasaje. Se acepta en el MVP (P1).
- **Revisión:** en v3 (SIA-E2-01). Si hiciera falta mejorar las tildes antes, la opción 2 (`unaccent`) requiere un ADR nuevo y la aprobación del usuario.
