# E3 Banco SERUMS

**Issue:** #4

**Objetivo:** convertir balotarios (formato con `<mark>` como clave) en preguntas del banco, clasificadas por área/tema con IA y revisión humana, sin duplicados, con estado `borrador` y trazabilidad a la fuente.
**Valor:** banco amplio y trazable, requisito del diagnóstico, la práctica dirigida y la generación.

| Historia | Título | Fase | Pts | Issue |
|---|---|---|---|---|
| SIA-E3-01 | Parser del balotario con marca de clave | MVP | 5 | #23 |
| SIA-E3-02 | Estado borrador y trazabilidad pregunta-fuente | MVP | 3 | #24 |
| SIA-E3-03 | Importación del balotario con vista previa | MVP | 3 | #25 |
| SIA-E3-04 | Clasificación por área y tema con IA | MVP | 5 | #26 |
| SIA-E3-05 | Revisión humana de la clasificación y de la clave | MVP | 5 | #27 |
| SIA-E3-06 | Deduplicación contra el banco existente | MVP | 5 | #28 |

**Dependencias:** E1 (taxonomía), E2 (fuentes), E6-01 (servicio de IA) para E3-04.
**Decisión P2 (issue #65):** enfoque híbrido de revisión. Balotario de fuente «oficial verificada» (SIA-E2-06) y sin `conflicto_clave` (SIA-E3-06): basta revisar área/tema y consistencia estructural. Balotario no oficial, pregunta incierta o con conflicto de clave: verificación explícita de clave con nota (SIA-E3-05).
**Existente que se extiende:** tabla `preguntas` (estado hoy: activo/eliminada, sin `borrador` ni fuente), import con preview, `etiquetas_pregunta`.
