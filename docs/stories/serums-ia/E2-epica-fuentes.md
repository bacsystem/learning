# E2 Repositorio de fuentes académicas

**Issue:** #3

**Objetivo:** almacenar fuentes (balotarios, URLs oficiales, archivos .md) con ingesta, normalización en pasajes citables y versionado, detrás del puerto `SourceRepository`. El MVP y la v2 operan al 100 % sobre el adaptador propio en PostgreSQL; la búsqueda semántica (pgvector o NotebookLM) se reevalúa en v3 (decisión P1, issue #65).
**Valor:** toda cita de contenido generado apunta a un pasaje de una fuente versionada.

| Historia | Título | Fase | Pts | Issue |
|---|---|---|---|---|
| SIA-E2-01 | Spike: reevaluar búsqueda semántica (pgvector vs NotebookLM) | v3 | 2 | #15 |
| SIA-E2-02 | Puerto SourceRepository y adaptador propio | MVP | 5 | #16 |
| SIA-E2-03 | Ingesta de archivo .md | MVP | 5 | #17 |
| SIA-E2-04 | Ingesta de URL oficial | v2 | 5 | #18 |
| SIA-E2-05 | Normalización y pasajes citables | MVP | 3 | #19 |
| SIA-E2-06 | Versionado, ciclo de vida y marca «oficial verificada» | MVP | 5 | #20 |
| SIA-E2-07 | Gestión de fuentes (admin) | MVP | 5 | #21 |
| SIA-E2-08 | Adaptador de búsqueda semántica (pgvector o NotebookLM) | v3 | 5 | #22 |

**Dependencias:** E1 para vincular fuentes a profesión/área. Ninguna historia de MVP ni de v2 depende de SIA-E2-01 ni de SIA-E2-08.
**Decisiones:** sin cuenta ni proyecto de NotebookLM por ahora ($0 USD) (P1). Una fuente puede marcarse «oficial verificada» por un admin (SIA-E2-06, SIA-E2-07); esa marca habilita la revisión simplificada de balotarios oficiales (P2, SIA-E3-05).
