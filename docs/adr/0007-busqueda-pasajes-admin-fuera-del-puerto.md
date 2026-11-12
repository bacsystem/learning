# ADR 0007: búsqueda de pasajes para el admin, fuera del puerto

- **Estado:** propuesto (2026-11-11). Pendiente de revisión del tech-lead.
- **Fecha:** 2026-11-11
- **Historias:** SIA-E3-07 (API), SIA-E3-08 (pantalla)
- **Contrato:** [docs/api/serums-ia-e3-07-revision-basica.md](../api/serums-ia-e3-07-revision-basica.md) §2
- **Relacionada:** [ADR 0003](0003-puerto-source-repository-y-adaptador-postgres.md), puerto `SourceRepository`

## Contexto

Para citar una pregunta, el admin tiene que elegir un pasaje (SIA-E3-07, regla 1): búsqueda por texto, solo en fuentes `activa`, por relevancia, con ≤ 20 resultados, y en cada resultado los ids, el título de la fuente, el número de versión y un extracto.

Hoy la única búsqueda es el puerto `BuscarPasajes` (`backend/internal/fuentes/source.go:188-190`; SQL en `pgsource.go:165-191`). No sirve tal cual:

- exige `EspecialidadID > 0` (`NormalizarBusqueda`, `source.go:207`), pero la historia no filtra por profesión y `preguntas.especialidad_id` es opcional (`014_create_preguntas.sql:8`);
- `PasajeEncontrado` (`source.go:146-155`) no trae el título de la fuente y trae el texto completo del pasaje (hasta 800 palabras), no un extracto;
- el puerto es el contrato de la generación con IA. Un adaptador v3 (SIA-E2-08) reemplazaría **solo** `BuscarPasajes` (`source.go:176-177`). La búsqueda para citar tiene que devolver siempre pasajes **propios** de la tabla `pasajes`, porque `pregunta_citas.pasaje_id` es una FK a ella (065).

Hay precedente de consultas admin fuera del puerto: el listado S3 (`source.go:120-122`; `listado.go:18-40`, que usa `$3::bigint = 0` para «sin filtro de profesión»).

## Opciones consideradas

1. **Exponer el puerto por HTTP y exigir `especialidad_id`.** Falla con las preguntas sin profesión, filtra algo que la historia no pide y obliga a una segunda consulta para el título. Si el puerto cambiara de adaptador, la búsqueda para citar podría devolver pasajes que no son propios. Descartada.
2. **Cambiar el puerto para aceptar `EspecialidadID = 0` y agregar el título a `PasajeEncontrado`.** Toca el contrato del puerto, su suite (`fuentestest/contract.go`) y el doble en memoria por una necesidad que es solo del admin. Descartada.
3. **Consulta admin en `internal/fuentes`, fuera del puerto, que comparte el SQL de búsqueda con `PgSource.BuscarPasajes`.** **Elegida.**

## Decisión

- **Ruta:** `GET /admin/pasajes?q=…`, solo admin, en `internal/fuentes` (`RutaBuscarPasajes` junto a las de `routes.go:21-28`).
- **Un único SQL de búsqueda**, en una función no exportada de `pgsource.go` que reciben las dos entradas:
  - el texto ya plegado con `texto.Plegar`, `websearch_to_tsquery('spanish', …)`, solo `fuentes.estado = 'activa'` y versión vigente (`f.version_vigente_id = v.id`), orden `ts_rank_cd DESC, p.id ASC`, `LIMIT`;
  - el filtro de profesión pasa a `($2::bigint = 0 OR NOT EXISTS (…) OR EXISTS (…))`, con el mismo patrón que `listado.go:36-38`;
  - la consulta también selecciona `f.titulo`.
- **El puerto no cambia:** `BuscarPasajes` sigue exigiendo `EspecialidadID > 0` con `NormalizarBusqueda` y llama a la función con su profesión. Su suite de contrato debe seguir pasando.
- **La consulta admin** valida `q` con los mismos 3–300 runes (`MinConsulta`, `MaxConsulta`), llama a la función con profesión `0` y límite `MaxResultados` (20), y arma el extracto con `texto.Extracto` (contrato §5).
- **Sin filtros de profesión, área ni tema** en el MVP (YAGNI). Si E3-08 los necesita, se agregan como query opcional sin tocar el puerto.

## Consecuencias

- La relevancia y el criterio de «fuente activa, versión vigente» son los mismos para el admin y para la IA: un único SQL.
- El admin solo puede elegir pasajes de la versión vigente. Las citas a versiones anteriores, que se conservan por la inmutabilidad (ADR 0003), siguen visibles en el detalle de la pregunta.
- Un adaptador v3 del puerto no afecta a la búsqueda del admin, que siempre cita pasajes propios.
- Sin dependencias ni servicios nuevos. Sin migración: usa `idx_pasajes_tsv` (062).
- La consulta `q` viaja en la URL, como en `GET /admin/fuentes`. Hoy ningún middleware registra la URL. Un logger de peticiones futuro debe omitir la query (README §9).
