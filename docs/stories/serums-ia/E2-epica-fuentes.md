# E2 Repositorio de fuentes académicas

**Issue:** #3

**Objetivo:** almacenar fuentes (balotarios, URLs oficiales, archivos .md) con ingesta, normalización en pasajes citables y versionado, detrás del puerto `SourceRepository`. El adaptador propio (Postgres) es el principal; NotebookLM es un adaptador opcional posterior a un spike.
**Valor:** toda cita de contenido generado apunta a un pasaje de una fuente versionada.

| Historia | Título | Fase | Pts | Issue |
|---|---|---|---|---|
| SIA-E2-01 | Spike: verificar la API de NotebookLM | MVP | 2 | #15 |
| SIA-E2-02 | Puerto SourceRepository y adaptador propio | MVP | 5 | #16 |
| SIA-E2-03 | Ingesta de archivo .md | MVP | 5 | #17 |
| SIA-E2-04 | Ingesta de URL oficial | v2 | 5 | #18 |
| SIA-E2-05 | Normalización y pasajes citables | MVP | 3 | #19 |
| SIA-E2-06 | Versionado y ciclo de vida de fuentes | MVP | 3 | #20 |
| SIA-E2-07 | Gestión de fuentes (admin) | MVP | 3 | #21 |
| SIA-E2-08 | Adaptador NotebookLM | v3 | 5 | #22 |

**Dependencias:** E1 para vincular fuentes a profesión/área. Ninguna historia, salvo E2-08, depende del spike.
