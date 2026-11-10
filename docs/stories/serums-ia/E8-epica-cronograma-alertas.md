# E8 Cronograma SERUMS, alertas, recordatorios, .ics y reporte semanal

**Objetivo:** cronograma de hitos de procesos SERUMS configurable por admin, alertas a postulantes, recordatorios del plan, exportación a calendario y reporte semanal por email.
**Valor:** nadie pierde una fecha de inscripción o examen; refuerza el hábito.

| Historia | Título | Fase | Pts |
|---|---|---|---|
| SIA-E8-01 | Cronograma de procesos (admin) | MVP | 3 |
| SIA-E8-02 | Alertas de próximos procesos | MVP | 3 |
| SIA-E8-03 | Recordatorios del plan de estudio | v2 | 3 |
| SIA-E8-04 | Exportación del plan a .ics | v2 | 2 |
| SIA-E8-05 | Reporte semanal por email | v3 | 5 |

**Existente que se reutiliza:** notificaciones in-app, `email_queue` con worker, `audit_log`.
**Dependencias:** E1-03, E7-03, E4-03.
