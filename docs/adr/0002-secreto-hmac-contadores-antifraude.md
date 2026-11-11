# ADR 0002: secreto del HMAC de los contadores antifraude

- **Estado:** propuesto (pendiente de revisión del tech-lead)
- **Fecha:** 2026-11-11
- **Historias:** SIA-E4-01 (cooldown del diagnóstico), SIA-E6-02 (cuota de IA), SIA-E9-07 (borrado de datos de aprendizaje)
- **Decisión de producto de origen:** P8 (issue #65), supuesto 18 de `docs/stories/serums-ia/README.md:146`

## Contexto

Dos contadores deben sobrevivir al borrado de datos de aprendizaje (SIA-E9-07) sin guardar el `usuario_id` en claro ni ningún otro dato personal (README:146 y límites en README:171):

- **cuota de acciones IA reales** por postulante: 5 por día y 60 por mes, hasta fin de mes en `America/Lima`;
- **fecha del último diagnóstico finalizado:** cooldown de 30 días.

La clave de cada fila es `HMAC(usuario_id)` con un secreto de servidor, que «vive solo en variables de entorno» (SIA-E4-01, sección Seguridad).

Lo que ya hay en el repo:

- `internal/crypto/crypto.go:42-46` ofrece `HMAC(key []byte, data string)` (HMAC-SHA256, hex).
- `cmd/server/main.go:70-79` reutiliza `ENCRYPTION_KEY` como clave AES y como clave HMAC de DNI y email, y deja anotado que lo correcto sería tener claves separadas. No se puede rotar sin una migración de datos (`docs/puesta-en-marcha.md:71`).
- `internal/config/config.go:13,70-75` valida `JWT_SECRET` (obligatorio, ≥ 32 bytes) y `config.go:76-78` exige `ENCRYPTION_KEY`. Si `config.Load` falla, `cmd/server/main.go:49-53` termina con código 1.
- `internal/auth/repository.go:52` separa dominios con un prefijo (`"ip:"+…`) y `056_create_auth_intentos_fallidos.sql:11-12` documenta claves de contador con prefijo (`reg_dni_ip:`, `reg_dni:`). La tabla de 056 (`clave`, `ventana_inicio`, `intentos`, `expira_en`, con purga perezosa de las filas vencidas) es el patrón para los contadores nuevos.

### Límite del pseudónimo

El espacio de `usuario_id` (BIGSERIAL) es pequeño. Con el secreto en la mano, recalcular `HMAC(id)` para todos los ids es trivial. El HMAC protege frente a quien solo tiene la base de datos (volcado, backup o réplica de lectura), no frente a quien también tiene el entorno del servidor. Por eso el secreto **no puede** estar en la base de datos ni en el repo, y no se comparte con otras claves.

## Opciones consideradas

1. **Reutilizar `ENCRYPTION_KEY`.** Descartada: amplía el radio de una clave que ya no se puede rotar (`main.go:70-74`) y ata la rotación de los contadores a la del cifrado de personas.
2. **Derivar una subclave de `ENCRYPTION_KEY` (HKDF).** Sin variable nueva, pero hereda la imposibilidad de rotar de `ENCRYPTION_KEY`. Descartada.
3. **Variable de entorno propia con rotación que reinicia los contadores.** Simple, pero cada rotación (y sobre todo una rotación tras una filtración, justo cuando más importa) regala a todos los usuarios una cuota de IA y un diagnóstico nuevos. Además, el costo de IA tiene presupuesto fijo (40 USD/mes, README:167). Descartada.
4. **Variable de entorno propia con doble clave durante la rotación.** Es elegida. La vida máxima de un contador es corta (cuota: hasta fin de mes, ≤ 31 días; cooldown: 30 días), así que el periodo con dos claves también lo es.

## Decisión

### Variables de entorno

| Variable | Obligatoria | Formato | Uso |
|---|---|---|---|
| `ANTIFRAUD_HMAC_KEY` | sí, en **todos** los entornos (también `development` y `test`) | hex, ≥ 64 caracteres (≥ 32 bytes decodificados), mismo formato que `ENCRYPTION_KEY`; se genera con `openssl rand -hex 32` | clave vigente: con ella se escribe y se lee |
| `ANTIFRAUD_HMAC_KEY_PREVIOUS` | no | mismo formato | clave anterior durante una rotación: solo se lee |

### Carga y validación (`internal/config/config.go`)

`config.Load` gana los campos `AntifraudHMACKey` y `AntifraudHMACKeyPrevious` (bytes ya decodificados), y **falla al arrancar** si se da cualquiera de estos casos:

- `ANTIFRAUD_HMAC_KEY` falta, no es hex válido o tiene menos de 32 bytes;
- es igual a `ENCRYPTION_KEY` o a `JWT_SECRET`, o igual a `ANTIFRAUD_HMAC_KEY_PREVIOUS`;
- `ANTIFRAUD_HMAC_KEY_PREVIOUS` está definida pero es inválida.

El mensaje de error nombra la variable, nunca su valor, igual que `config.go:73-74`.

La variable entra con la **primera historia que la use** (SIA-E6-02 o SIA-E4-01, la que se implemente antes), no antes. Si fuera obligatoria sin consumidor, rompería el arranque y el CI sin necesidad.

### Derivación de la clave de cada contador

```text
clave = prefijo + ":" + hex(HMAC-SHA256(ANTIFRAUD_HMAC_KEY, prefijo + ":" + strconv.FormatInt(usuario_id, 10)))
```

- Prefijos (separación de dominios): `ia_cuota` (SIA-E6-02) y `diag_cooldown` (SIA-E4-01). Con prefijos distintos, el mismo usuario tiene valores distintos en cada contador, y quien solo tiene la base de datos no puede cruzarlos entre sí.
- Lo calcula un helper único, `crypto.HMAC` (`crypto.go:42`), detrás de una función del nuevo paquete de contadores. Ningún otro paquete arma claves.
- Nunca se registran en logs ni la clave ni el HMAC. En los logs solo aparece el prefijo del contador.

### Rotación con doble clave

1. El operador mueve el valor vigente a `ANTIFRAUD_HMAC_KEY_PREVIOUS`, pone uno nuevo en `ANTIFRAUD_HMAC_KEY` y reinicia.
2. **Escritura:** solo con la clave vigente.
3. **Lectura:** se calculan las dos claves y se combinan:
   - cuota: `usados = contador(vigente) + contador(anterior)` para la misma ventana. No hay doble conteo porque la anterior ya no recibe escrituras.
   - cooldown: `último = max(fecha(vigente), fecha(anterior))`.
   - **Reserva atómica durante la rotación:** la reserva es un único `INSERT … ON CONFLICT DO UPDATE … WHERE valor < límite` sobre **una** clave (abajo). Primero se lee el contador de la clave anterior; ese valor está congelado porque ya no recibe escrituras. Después se reserva sobre la clave vigente con `límite_efectivo = límite − contador(anterior)`. Si `límite_efectivo ≤ 0`, se rechaza sin escribir. Como la clave anterior no cambia, no hay carrera entre la lectura y la reserva.
4. Pasados **31 días** desde la rotación, todos los contadores escritos con la clave anterior ya vencieron (`expira_en`). Entonces se quita `ANTIFRAUD_HMAC_KEY_PREVIOUS` y la purga perezosa elimina las filas que queden.
5. Si la rotación se hace por una filtración, el procedimiento es el mismo: la clave filtrada sigue sirviendo para leer durante ≤ 31 días, y lo que permite (pseudonimizar contadores de corta vida) es menor que el costo de regalar cuotas.

### Forma de las tablas (la detallan E4-01 y E6-02)

Se reutiliza el patrón de `056_create_auth_intentos_fallidos.sql:17-25`: `clave TEXT`, `ventana_inicio TIMESTAMPTZ`, valor del contador, `expira_en TIMESTAMPTZ`, PK `(clave, ventana_inicio)` e índice por `expira_en`. La reserva se hace con un único `INSERT … ON CONFLICT DO UPDATE … WHERE valor < límite RETURNING`. Las filas no llevan `usuario_id` ni FK a `usuarios`. SIA-E9-07 no las borra: se borran solas al vencer.

### Archivos que debe tocar la historia que introduzca la variable

| Archivo | Cambio |
|---|---|
| `backend/internal/config/config.go` y `config_test.go` | campos, validación y tests de falta / formato / igualdad |
| `.env.example` de la raíz (lo referencia `Makefile:124-127`; el architect no tiene permiso para leerlo) | variable con valor vacío y comentario |
| `docker-compose.yml:56-58` (bloque `environment` del backend) | `ANTIFRAUD_HMAC_KEY: ${ANTIFRAUD_HMAC_KEY:?…}` como las demás |
| `Makefile:124-137` (`compose/init`) | generarla con `openssl rand -hex 32` al crear `.env` |
| `.github/workflows/backend.yml` (bloque `env`, `:23`) | solo si algún test llama a `config.Load`; si no, los tests inyectan la clave directamente |
| `docs/puesta-en-marcha.md:70-71` | fila de la tabla de variables, procedimiento de rotación y aviso de que no se debe copiar `ENCRYPTION_KEY` |

## Consecuencias

- **Positivas:**
  - Clave aislada y rotable sin pérdida de contadores ni migración de datos.
  - Falla al arrancar en vez de calcular HMACs con una clave vacía.
  - Las claves de contador no se pueden cruzar entre tablas.
- **Negativas:**
  - Una variable obligatoria más en todos los entornos.
  - Durante una rotación, cada lectura calcula dos HMAC (costo despreciable).
- **Límite asumido:** la seudonimización no resiste a quien obtenga a la vez la base de datos y el entorno del servidor. Está documentado y es aceptable porque los contadores no contienen datos académicos ni personales.
