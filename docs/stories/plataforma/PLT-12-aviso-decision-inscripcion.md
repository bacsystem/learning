---
id: PLT-12
titulo: Aviso de aprobación o rechazo de inscripción
epica: PLT-E3
fase: MVP
prioridad: Alta
estimacion: 3
roles: [postulante, admin]
depende_de: [PLT-11]
---

# PLT-12 Aviso de aprobación o rechazo de inscripción

**Como** postulante, **quiero** enterarme por correo y dentro de la app cuando mi inscripción es aprobada o rechazada, y por qué, **para** actuar sin tener que revisar /procesos.

## Contexto
Hoy no se envía correo ni notificación, aunque el diálogo del admin dice «El usuario será notificado por correo». El motivo del rechazo solo se ve si el postulante vuelve a /procesos. «Inscripción» es la de la app (organiza la práctica), no un trámite ante la institución. Decisiones: [README, PLT-E3](README.md#épica-plt-e3-aviso-de-decisiones-al-postulante).

## Reglas de negocio
1. Al aprobar, se envía un correo y se crea una notificación en la app. Contenido: nombre del proceso, institución y un enlace a /procesos.
2. Al rechazar, igual, e incluye el **motivo** que escribió el admin (E3 decisión 4).
3. Idioma: el del usuario (PLT-11), con fallback `es`.
4. Sin duplicados (E3 decisión 6); un fallo de correo no revierte la decisión (E3 decisión 5).
5. La notificación aparece en la campana existente, sin leer, y enlaza a /procesos.
6. El correo no incluye datos de otros postulantes.
7. **Motivo (P6):** el servidor lo exige al rechazar (1 a 500 caracteres, tras quitar espacios; vacío o en blanco = 422 `motivo_requerido`). Al aprobar es opcional, hasta 500. No depende solo de la validación del diálogo.
8. Si el postulante vuelve a inscribirse tras un rechazo (flujo `onReapply` existente), un nuevo rechazo o aprobación genera un nuevo aviso.

## Criterios de aceptación
```gherkin
Escenario: Aprobación
  Dado un postulante con idioma "es" y una inscripción pendiente
  Cuando el admin la aprueba
  Entonces el postulante recibe un correo en español con el nombre del proceso y una notificación sin leer que enlaza a /procesos

Escenario: Rechazo con motivo
  Dado un postulante con idioma "en" y una inscripción pendiente
  Cuando el admin la rechaza con el motivo "Documento ilegible"
  Entonces el correo y la notificación están en inglés, rodean el motivo "Documento ilegible" tal cual lo escribió el admin, y enlazan a /procesos

Escenario: Idioma ausente
  Dado un usuario sin idioma válido
  Cuando se le aprueba una inscripción
  Entonces el aviso sale en español

Escenario: Acción repetida
  Dado una inscripción ya aprobada
  Cuando se repite la aprobación
  Entonces no se envía otro correo ni se crea otra notificación

Escenario: Falla el envío
  Dado el servicio de correo caído
  Cuando el admin aprueba la inscripción
  Entonces la decisión se guarda, la notificación en la app existe y el correo queda registrado para reintento (visible en la salud del correo)

Escenario: Motivo vacío en rechazo
  Dado el diálogo de rechazo
  Cuando el admin intenta rechazar sin motivo
  Entonces no se envía y se muestra el campo requerido (comportamiento actual)

Escenario: Motivo vacío llamando a la API
  Dado una inscripción pendiente
  Cuando un cliente rechaza sin motivo o con un motivo de más de 500 caracteres
  Entonces recibe 422 (motivo_requerido / validacion), la inscripción sigue pendiente y no se envía ningún aviso

Escenario: Nueva inscripción tras rechazo
  Dado un postulante rechazado que se inscribe de nuevo
  Cuando el admin la rechaza otra vez
  Entonces recibe un nuevo aviso con el nuevo motivo
```

## Fuera de alcance
Preferencias para desactivar avisos; avisos al admin cuando alguien se inscribe; SMS o push; traducir el motivo; avisos de otras transiciones (cancelación, vencimiento).

## Seguridad y privacidad
El motivo puede contener datos del trámite: solo se envía al propio postulante. Los logs no registran el cuerpo del correo ni el motivo. El admin sigue auditado (`audit_log`) como hoy.

## i18n
Plantilla de correo (asunto y cuerpo), texto de notificación y la frase del diálogo del admin (que añade que también se notifica en la app), en es/en. El motivo no se traduce.

## Definición de terminado
Tests de backend: transición única, idioma es/en/fallback, motivo incluido, fallo de correo sin revertir, sin duplicados. Prueba con correo de desarrollo (captura de es/en en el PR). Contrato en `docs/api/`. `tsc`/`lint`/`build`. Review limpio.
