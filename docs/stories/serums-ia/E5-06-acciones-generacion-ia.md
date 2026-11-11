---
id: SIA-E5-06
titulo: Acciones rápidas de generación con IA
epica: E5
fase: v2
prioridad: Media
estimacion: 2
roles: [usuario]
depende_de: [SIA-E5-03, SIA-E6-06, SIA-E6-07]
---

# SIA-E5-06 Acciones rápidas de generación con IA

**Como** postulante, **quiero** pedir desde el dashboard material de estudio o preguntas de mi área débil, **para** reforzarla sin buscar recursos por mi cuenta.

## Contexto
Botones «Material de estudio» y «Preguntas tipo SERUMS» en cada área débil. Solo conectan con E6: no generan nada que no pase por cita y aprobación.

## Reglas de negocio
1. «Material»: si existe material aprobado del área/tema, se abre de inmediato; si no, se ofrece «Solicitar al equipo» (crea una solicitud para admin, SIA-E6-06) y se informa que se revisará antes de mostrarse.
2. «Preguntas»: abre una práctica con preguntas aprobadas (SIA-E5-04); si hay menos de 10, se ofrece solicitar más al equipo.
3. El alumno nunca ve contenido generado que no esté aprobado.
4. Encolar una solicitud al equipo no llama al LLM y **no descuenta cuota de IA** del postulante (decisión P3, issue #65; SIA-E6-02 regla 7). La generación posterior la hace un admin y descuenta de su propio límite.
5. Se evita duplicar una solicitud abierta del mismo (área, tema) (409 `solicitud_abierta`).
6. Para evitar abuso sin cuota IA: máx. 5 solicitudes abiertas a la vez por postulante (429 `limite_solicitudes`); al atenderse o descartarse una, se libera el cupo.

## Criterios de aceptación
```gherkin
Escenario: Material disponible
  Dado material aprobado del área
  Cuando pulsa «Material de estudio»
  Entonces se abre el material con sus citas

Escenario: Material no disponible
  Dado ningún material aprobado del área
  Cuando pulsa «Solicitar al equipo»
  Entonces se crea una solicitud y ve «Lo revisaremos antes de publicarlo»

Escenario: Solicitud duplicada
  Dado una solicitud abierta del mismo tema
  Cuando el usuario solicita de nuevo ese tema
  Entonces recibe 409 "solicitud_abierta"

Escenario: Solicitud no descuenta cuota de IA
  Dado un postulante con 2 de 5 acciones IA reales consumidas hoy
  Cuando crea una solicitud al equipo
  Entonces la solicitud se crea y su consumo de IA sigue en 2 de 5

Escenario: Cuota de IA agotada no bloquea la solicitud
  Dado un postulante con 5 de 5 acciones IA reales consumidas hoy
  Cuando crea una solicitud al equipo de un tema nuevo
  Entonces la solicitud se crea

Escenario: Límite de solicitudes abiertas
  Dado un postulante con 5 solicitudes abiertas
  Cuando crea otra solicitud
  Entonces recibe 429 "limite_solicitudes"

Escenario: Ningún contenido sin aprobar
  Dado un material en borrador cuyo id conoce el usuario
  Cuando el usuario intenta abrirlo
  Entonces recibe 404 y no puede abrir ningún borrador ni material en revisión
```

## Fuera de alcance
Generación en vivo para el alumno sin aprobación (decisión de seguridad médica).

## Dependencias
SIA-E5-03, SIA-E6-06, SIA-E6-07.

## Seguridad y privacidad
Sin datos personales en la solicitud; autorización por dueño.

## i18n
Botones y mensajes en es/en.

## Definición de terminado
Tests de límites y de acceso a contenido no aprobado; `tsc`, `lint`, `build`; review limpio.
