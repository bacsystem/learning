# E7 Plan de estudio personalizado

**Objetivo:** generar un plan diario según la fecha del examen, horas disponibles, dominio por área y peso en el examen real, y mantenerlo al día.
**Valor:** el postulante sabe qué estudiar cada día y cuánto.

| Historia | Título | Fase | Pts |
|---|---|---|---|
| SIA-E7-01 | Configuración del plan (fecha y horas) | MVP | 3 |
| SIA-E7-02 | Generador del plan | MVP | 5 |
| SIA-E7-03 | Vista del plan y registro de cumplimiento | MVP | 3 |
| SIA-E7-04 | Replanificación automática | v2 | 3 |

**Decisión:** el generador del plan es determinista (fórmula sobre dominio y peso); la IA no decide el plan, así no depende de presupuesto ni de aprobación.
**Dependencias:** E4-03, E1-03.
