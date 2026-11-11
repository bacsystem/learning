---
id: SIA-E8-03
titulo: Recordatorios del plan de estudio
epica: E8
fase: v2
prioridad: Media
estimacion: 3
roles: [usuario]
depende_de: [SIA-E7-03, SIA-E8-02]
---

# SIA-E8-03 Recordatorios del plan de estudio

**Como** postulante, **quiero** recibir un recordatorio de mi sesión de estudio del día, **para** mantener la constancia.

## Reglas de negocio
1. Preferencias: activar/desactivar, hora del recordatorio (por defecto 19:00 America/Lima, en pasos de 30 min) y canal (in-app / email).
2. Se envía solo en días con bloques pendientes; no se envía si el usuario ya marcó todo hecho o si ya estudió ese día.
3. Máx. 1 recordatorio por día; después de 3 recordatorios ignorados seguidos (sin actividad), se pausan y se informa en la próxima visita.
4. El texto incluye el área principal del día y los minutos totales; sin datos personales distintos del saludo.
5. Se desactivan al finalizar o cancelar el plan.

## Criterios de aceptación
```gherkin
Escenario: Recordatorio del día
  Dado un día con 2 bloques pendientes y recordatorio a las 19:00
  Cuando llegan las 19:00
  Entonces recibe un aviso con área principal y minutos

Escenario: Día completado
  Dado un día con todos los bloques hechos
  Cuando llega la hora del recordatorio
  Entonces no se envía

Escenario: Máximo diario
  Dado un recordatorio ya enviado hoy
  Cuando el trabajo de recordatorios corre de nuevo
  Entonces nunca se envían 2 recordatorios el mismo día

Escenario: Pausa por inactividad
  Dado 3 recordatorios ignorados seguidos
  Cuando llega el siguiente recordatorio
  Entonces se pausan y la próxima visita muestra «Pausamos tus recordatorios» con opción de reactivarlos

Escenario: Desactivar
  Dado un usuario con recordatorios activos
  Cuando desactiva los recordatorios
  Entonces no recibe más y la preferencia se conserva

Escenario: Plan finalizado
  Dado un plan en estado finalizado o cancelado
  Cuando llega la hora del recordatorio
  Entonces no se envían recordatorios

Escenario: Hora inválida
  Dado la pantalla de preferencias de recordatorios
  Cuando elige 19:15
  Entonces recibe 422
```

## Fuera de alcance
Push móvil.

## Dependencias
SIA-E7-03, SIA-E8-02 (preferencias y canales).

## Seguridad y privacidad
Preferencias solo del dueño.

## i18n
Textos del recordatorio y de preferencias en es/en.

## Definición de terminado
Tests de pausa, hora y zona horaria; plantillas es/en; review limpio.
