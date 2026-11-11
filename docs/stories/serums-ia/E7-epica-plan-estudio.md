# E7 Plan de estudio personalizado

**Issue:** #8

**Objetivo:** generar un plan diario según la fecha del examen, horas disponibles, dominio por área y peso en el examen real, y mantenerlo al día.
**Valor:** el postulante sabe qué estudiar cada día y cuánto.

| Historia | Título | Fase | Pts | Issue |
|---|---|---|---|---|
| SIA-E7-01 | Configuración del plan (fecha y horas) y fecha oficial del examen | MVP | 5 | #48 |
| SIA-E7-02 | Generador del plan | MVP | 5 | #49 |
| SIA-E7-03 | Vista del plan y registro de cumplimiento | MVP | 3 | #50 |
| SIA-E7-04 | Replanificación automática | v2 | 3 | #51 |

**Decisión:** el generador del plan es determinista (fórmula sobre dominio y peso); la IA no decide el plan, así no depende de presupuesto ni de aprobación.
**Decisión P5 (issue #65):** la fecha oficial del examen viene del hito `examen` cargado por admin con fuente `.gob.pe` (SIA-E8-01); sus cambios se propagan automáticamente a los planes con fecha de origen `oficial`, nunca a los de origen `manual` (SIA-E7-01); la vista avisa y ofrece regenerar (SIA-E7-03) y en v2 se propone replanificar (SIA-E7-04).
**Dependencias:** E4-03, E1-03, E8-01 (fecha oficial del examen).
