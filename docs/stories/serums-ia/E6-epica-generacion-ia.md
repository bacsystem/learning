# E6 Generación con IA, citas y aprobación de admin

**Issue:** #7

**Objetivo:** servicio de IA agnóstico de proveedor con control de costos, y generación de preguntas, material y explicaciones basada en fuentes, con cita obligatoria, marca de incertidumbre y aprobación de admin antes de llegar al alumno.
**Valor:** escala el contenido sin sacrificar fiabilidad médica.

| Historia | Título | Fase | Pts | Issue |
|---|---|---|---|---|
| SIA-E6-01 | Servicio de IA: puerto, registro de uso y redacción de datos | MVP | 5 | #39 |
| SIA-E6-02 | Control de costos: presupuesto, límites y caché | MVP | 5 | #40 |
| SIA-E6-03 | Generación de preguntas borrador con citas | MVP | 5 | #41 |
| SIA-E6-04 | Validación de citas y marca de incertidumbre | MVP | 5 | #42 |
| SIA-E6-05 | Cola de aprobación de preguntas (admin) | MVP | 5 | #43 |
| SIA-E6-06 | Generación de material de estudio con citas | v2 | 5 | #44 |
| SIA-E6-07 | Material de estudio aprobado para el postulante | v2 | 3 | #45 |
| SIA-E6-08 | Explicaciones con cita por respuesta | v2 | 5 | #46 |
| SIA-E6-09 | Reporte de contenido erróneo por el postulante | v2 | 5 | #47 |
| SIA-E6-10 | Adaptador del proveedor real de IA | MVP | 3 | #173 |

**Regla maestra:** sin cita a pasaje de fuente activa Y sin aprobación de admin, nada llega al alumno; lo incierto exige revisión explícita. Los reportes de postulantes nunca ocultan contenido aprobado por un admin: lo priorizan en la cola y avisan a los admins (SIA-E6-09).
**Dependencias:** E2 (pasajes), E3-02 (estado borrador).
**Decisiones (issue #65):** P3: 5 acciones IA reales/día y 60/mes por postulante, 200/día por admin, 20 preguntas por solicitud, presupuesto global 40 USD/mes con alerta al 80 % y corte al 100 %; encolar una solicitud sin llamada al LLM no descuenta cuota (SIA-E6-02, SIA-E5-06). P4: segundo revisor desactivado por defecto, gobernado por `revision.segundo_revisor_obligatorio` en `configuracion_sistema` y auditado; la verificación de cita y la nota para lo incierto siguen obligatorias (SIA-E6-05). P8: contadores de cuota IA bajo `HMAC(usuario_id)` (SIA-E6-02).
