# ADR 0008: parser del balotario bajo demanda y cita por rango de líneas

- **Estado:** propuesto (2026-11-12). Pendiente de revisión del tech-lead.
- **Producto:** el PO aceptó la cita por solape con los pasajes de la misma versión (D8, supuesto 25). Eso deja sin efecto la Q7 («los balotarios no se unen», `docs/api/README.md` §12).
- **Fecha:** 2026-11-12
- **Historias:** SIA-E3-01 (parser), SIA-E3-03 (importación)
- **Contrato:** [docs/api/serums-ia-e3-01-e3-03-balotario.md](../api/serums-ia-e3-01-e3-03-balotario.md) §2 y §4.2
- **Enmienda:** [ADR 0003](0003-puerto-source-repository-y-adaptador-postgres.md) § Normalización, pasos 6 (última viñeta), 8 y 9

## Contexto

La ADR 0003 dejó a SIA-E3-01 la segmentación de los balotarios «por pregunta»:

- paso 6: «la unión no se aplica a los pasajes de balotario, que van uno por pregunta»;
- paso 9: «los balotarios se segmentan por pregunta en SIA-E3-01… `normalizador_version` distingue los pasajes».

El código actual no lo hace. `fuentes.Normalizar(contenido)` (`backend/internal/fuentes/normalizar.go:80`) no recibe el `tipo`, así que los balotarios ya ingeridos tienen pasajes genéricos v1 (`NormalizadorVersion = 1`, `normalizar.go:15`).

SIA-E3-01 necesita dos cosas:

1. extraer las preguntas con sus alternativas, su clave y su rango de líneas;
2. que cada pregunta importada cite su fuente, porque aprobarla exige ≥ 1 cita a un pasaje de una fuente `activa` (E3-02 §4.1, P4), y `pregunta_citas.pasaje_id` es una FK a `pasajes` (065).

Hay restricciones del código:

- `uq_fuente_versiones_sha256` (`062_create_fuentes.sql:71`) es global: el **mismo archivo no se puede volver a subir**. No hay forma de reingerir un balotario ya cargado para regenerar sus pasajes con otro algoritmo.
- Los pasajes ya publicados pueden estar citados, y la ADR 0003 (paso 9) prohíbe reescribirlos.
- Los pasajes guardan `linea_inicio` y `linea_fin` del contenido original (`062:84-85`), numeradas con `fuentes.DividirLineas` (`source.go:290`). La versión vigente conserva su `contenido` (`ck_fuente_versiones_contenido`, `062:65`).

## Opciones consideradas

1. **Normalizador v2 para `tipo = 'balotario'`, en el worker: un pasaje por pregunta.**
   - A favor: cita exacta, de un pasaje por pregunta.
   - En contra:
     - cambia el worker y el puerto (`Normalizar` tendría que recibir el tipo);
     - mezcla la validación del balotario (preguntas «con problemas») con la ingesta;
     - las versiones ya ingeridas quedan con v1 y no se pueden reingerir por el hash único, así que habría dos caminos de cita para siempre.
   - Descartada.
2. **Parser bajo demanda y cita por solape de rangos de líneas.**
   - El parser es una función pura sobre `fuente_versiones.contenido`. Lo llaman la vista previa y la confirmación.
   - Cada pregunta importada cita los pasajes v1 de la versión cuyo rango se solapa con el suyo.
   - Elegida.
3. **Parser bajo demanda, guardando el resultado del parseo en una tabla al previsualizar.**
   - En contra: obliga a invalidar y a manejar la concurrencia de un estado intermedio.
   - Parsear 2 MiB toma milisegundos, así que no aporta nada (YAGNI).
   - Descartada.

## Decisión

- **`internal/balotario.Parsear(contenido []byte) (Resultado, error)`** es pura y determinista. Usa `fuentes.DividirLineas` para que los números de línea coincidan con los de los pasajes.
  - La vista previa la ejecuta en cada petición.
  - La confirmación la **vuelve a ejecutar** dentro de su transacción, sobre el contenido leído en el servidor. Nunca confía en lo que vio el cliente; solo comprueba que la versión sea la misma.
- **El worker y `Normalizar` no cambian.** Los balotarios siguen con la segmentación genérica v1, también con la unión de fragmentos cortos. `normalizador_version` sigue en 1.
- **Cita por solape:**

  ```sql
  pa.fuente_version_id = $version
  AND pa.linea_inicio <= $linea_fin
  AND pa.linea_fin   >= $linea_inicio
  ```

  - Siempre hay ≥ 1 pasaje, porque los pasajes cubren todo token con palabra.
  - En la práctica son 1–3, porque una pregunta tiene ≤ 5 000 caracteres y un pasaje, ≥ 150 palabras.
  - Si una pregunta no se solapa con ningún pasaje, queda «con problemas» con el motivo `sin_pasaje` y no se importa (regla 9 de E3-03). Es una defensa: no debería ocurrir.
  - Más de 20 pasajes sería una violación del invariante y da `500`.
- **Las preguntas «con problemas» no generan nada:** ni pasaje ni pregunta. Solo se reportan, con su línea y sus motivos.
- **El texto del pasaje conserva `<mark>`** (`normalizar.go:41-42`). Al verificar la clave contra la cita (E3-07 §5, `texto`), el admin ve la marca original del balotario: es justo la evidencia que necesita.
- **Separación de alternativas pegadas** (enmienda del paso 8): la hace el parser, no el normalizador, y solo ante la **siguiente letra esperada con el mismo estilo** de etiqueta. Cada separación deja la alerta `alternativas_separadas` para la revisión de E3-05.

## Consecuencias

- **Positivas:**
  - Ni el worker, ni el puerto, ni las migraciones de fuentes cambian.
  - Las versiones ya ingeridas se pueden importar sin reingerirlas.
  - Hay un solo camino de cita para todos los balotarios.
  - La cita muestra el contexto del balotario con su marca original.
- **Negativas:**
  - La cita es un pasaje de 150–800 palabras que contiene la pregunta y otras vecinas, no solo la pregunta. Para ubicarla, la UI de E3-05/E3-08 usa `linea_inicio` y `linea_fin` de `pregunta_balotario`.
  - Al parsear en cada vista previa se repite un trabajo de milisegundos. Es aceptable.
- **Riesgo aceptado (inicio de pregunta):** una línea numerada con el siguiente número esperado siempre abre una pregunta nueva (contrato §2.2, regla c). Una lista «2.» dentro del enunciado de la pregunta 1 la parte. El resultado es visible (`alternativas_insuficientes` y `numeracion_rota` en las vecinas), y se prefirió a la alternativa, en la que una pregunta sin alternativas absorbe en silencio a la siguiente.
- **Etiquetas ambiguas** (decisión del PO, 2026-11-12, review del #193): una línea con forma de etiqueta que, ya abierta la primera alternativa, no abre una alternativa (letras que retroceden o se repiten, o un nombre científico como «C. difficile» seguido de la B real) **no** se absorbe como continuación. La pregunta va a «con problemas» con el motivo `etiquetas_ambiguas`. Así una clave nunca queda unida a otro texto. Los falsos positivos se aceptan: el admin corrige el archivo.
- **Riesgo aceptado:** la heurística de alternativas pegadas puede partir mal un texto que contenga justo la siguiente letra con el mismo estilo (p. ej. «A) Déficit de vitamina B) …»). Queda marcado con `alternativas_separadas` y E3-05 lo revisa una a una, así que no pasa en silencio.
- **Para la ADR 0003:** los pasos 6 (última viñeta), 8 y 9 quedan sustituidos por esta decisión. Si algún día hiciera falta un pasaje por pregunta, sería una ADR nueva con un `normalizador_version` 2 que solo aplique a versiones nuevas.
