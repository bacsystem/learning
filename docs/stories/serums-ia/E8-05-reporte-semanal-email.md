---
id: SIA-E8-05
titulo: Reporte semanal por email
epica: E8
fase: v3
prioridad: Media
estimacion: 5
roles: [usuario]
depende_de: [SIA-E8-02, SIA-E4-03]
---

# SIA-E8-05 Reporte semanal por email

**Como** postulante, **quiero** un resumen semanal de mi avance por email, **para** mantener el rumbo sin entrar a la plataforma.

## Reglas de negocio
1. Envío los lunes a las 8:00 America/Lima vía `email_queue`, solo a usuarios con inscripción aprobada, con actividad o plan en las últimas 4 semanas y preferencia activa (por defecto activada con opt-out).
2. Contenido: preguntas respondidas en la semana, aciertos %, cambio de dominio por las 3 áreas con mayor variación, cumplimiento del plan (% de minutos), días para el examen y las 3 prioridades de la semana (SIA-E5-03).
3. No se envía si la semana no tuvo actividad ni plan pendiente (se envía un solo mensaje «te extrañamos» tras 2 semanas de inactividad y luego se pausa).
4. Cada email incluye enlace de baja de un clic (token firmado, expira a 90 días) y enlace a preferencias.
5. Idempotente: un reporte por (usuario, semana ISO).
6. El email no contiene preguntas, claves ni contenido médico; solo métricas y enlaces a la plataforma.
7. Límite de envío: máx. 500 emails por hora por el worker (configurable).

## Criterios de aceptación
```gherkin
Escenario: Reporte con actividad
  Dado un usuario con 40 respuestas en la semana
  Cuando llega el lunes a las 8:00
  Entonces se encola un email con métricas y 3 prioridades

Escenario: Sin actividad
  Dado un usuario sin actividad ni plan pendiente
  Cuando llega el lunes a las 8:00
  Entonces no se envía reporte

Escenario: Recordatorio de inactividad
  Dado un usuario con 2 semanas sin actividad
  Cuando llega el lunes a las 8:00
  Entonces recibe un solo mensaje «te extrañamos» y después no más hasta volver a tener actividad

Escenario: Baja de un clic
  Dado un email de reporte con enlace de baja válido
  Cuando abre el enlace de baja
  Entonces la preferencia queda desactivada sin iniciar sesión y ve confirmación

Escenario: Token vencido o alterado
  Dado un enlace de baja con token vencido o alterado
  Cuando lo abre
  Entonces ve «Enlace no válido» y no cambia nada (400)

Escenario: Idempotencia
  Dado un reporte ya encolado para la semana ISO
  Cuando el trabajo se ejecuta dos veces el mismo lunes
  Entonces se envía un solo reporte por usuario

Escenario: Sin contenido médico
  Dado un email de reporte encolado
  Cuando se inspecciona su contenido
  Entonces el email no contiene enunciados, claves ni explicaciones

Escenario: Idioma
  Dado un usuario con idioma en
  Cuando se envía su reporte
  Entonces el email llega en inglés

Escenario: Límite de envío
  Dado 2 000 destinatarios en la cola
  Cuando el worker procesa el envío
  Entonces envía a lo sumo 500 por hora
```

## Fuera de alcance
Resumen para tutores o terceros; reportes mensuales.

## Dependencias
SIA-E8-02 (preferencias y canal), SIA-E4-03, SIA-E5-03.

## Seguridad y privacidad
Token de baja firmado y sin datos personales; sin PII en logs; solo se usa el email ya registrado.

## i18n
Plantilla del email y páginas de baja en es/en.

## Definición de terminado
Plantillas es/en probadas; tests de idempotencia y baja; review limpio.
