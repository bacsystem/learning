# E3 Banco SERUMS

**Objetivo:** convertir balotarios (formato con `<mark>` como clave) en preguntas del banco, clasificadas por área/tema con IA y revisión humana, sin duplicados, con estado `borrador` y trazabilidad a la fuente.
**Valor:** banco amplio y trazable, requisito del diagnóstico, la práctica dirigida y la generación.

| Historia | Título | Fase | Pts |
|---|---|---|---|
| SIA-E3-01 | Parser del balotario con marca de clave | MVP | 5 |
| SIA-E3-02 | Estado borrador y trazabilidad pregunta-fuente | MVP | 3 |
| SIA-E3-03 | Importación del balotario con vista previa | MVP | 3 |
| SIA-E3-04 | Clasificación por área y tema con IA | MVP | 5 |
| SIA-E3-05 | Revisión humana de la clasificación | MVP | 3 |
| SIA-E3-06 | Deduplicación contra el banco existente | MVP | 5 |

**Dependencias:** E1 (taxonomía), E2 (fuentes), E6-01 (servicio de IA) para E3-04.
**Existente que se extiende:** tabla `preguntas` (estado hoy: activo/eliminada, sin `borrador` ni fuente), import con preview, `etiquetas_pregunta`.
