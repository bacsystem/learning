# Borrador C — Textos legales neutros para varias instituciones

**Estado: PROPUESTA, pendiente de aprobación del usuario y de revisión legal. No está publicada y no se ha editado el frontend.**

Decisión del usuario (2026-11-15): una versión neutra para varias instituciones, **solo en español**, con el aviso «Spanish only». El usuario la aprueba antes de publicarla.

Origen: `frontend/src/app/auth/terms-and-conditions/page.tsx` y `privacy-policy/page.tsx`. Este borrador tiene dos partes separadas para que el usuario apruebe cada una por su cuenta:
- **Parte A — Reescritura neutra.** Cambio mínimo sobre el original, **misma numeración**, sin compromisos nuevos. Se puede publicar sola.
- **Parte B — Adiciones opcionales.** Cada una es una promesa o dato nuevo, con su pregunta y la historia de la que depende. Ninguna se publica sin decisión expresa.

## Decisiones del PO sobre la implementación (cuando se apruebe)

1. **Aviso «Spanish only»:** etiqueta visible junto al título de ambos documentos cuando la interfaz está en inglés: «Spanish only — this document is available in Spanish only.» En español no se muestra. El cuerpo no se traduce; el título y el aviso sí (es/en, en `dashboard`).
2. Los dos archivos conservan el comentario «PENDIENTE (revisión legal)» hasta que un abogado los revise. Aprobar el texto como producto no sustituye la revisión legal.
3. Mientras no se publique, se mantiene el texto actual.
4. Vocabulario neutro: «procesos de evaluación» con ejemplos «como ascensos o admisiones»; «las personas usuarias» en lugar de «el personal»; «grado o nivel, si tu institución lo usa» (esta frase exacta en todo el texto).

---

# Parte A — Reescritura neutra

## Términos y condiciones

**Intro:** Estos términos regulan el uso de Capacitate, una plataforma de práctica y simulacros para procesos de evaluación de distintas instituciones, como ascensos o admisiones. Al crear una cuenta o usar la plataforma aceptas estos términos; si no estás de acuerdo con ellos, no la uses.

**1. Objeto del servicio**
Capacitate ofrece bancos de preguntas, prácticas por curso y simulacros de examen para que las personas usuarias se preparen para los procesos de evaluación de su institución. El contenido es material de práctica y de estudio.

Capacitate es solo una aplicación de ayuda para estudiar y prepararse. No es una entidad certificadora ni forma parte de ninguna institución, y no tiene vínculo con las instituciones que convocan los procesos. No otorga certificados, constancias, puntajes ni resultados con validez frente a ninguna institución. Las preguntas y los simulacros son material de práctica, y el contenido no reemplaza las bases, los temarios ni las comunicaciones que publique cada institución para su proceso. La inscripción en un proceso dentro de Capacitate sirve solo para organizar tu práctica y no sustituye ningún trámite ante la institución.

**2. Cuentas de usuario**
- Debes registrarte con datos verdaderos y mantenerlos actualizados.
- La cuenta es personal: no compartas tu contraseña ni permitas que otra persona la use.
- Las cuentas nuevas quedan pendientes hasta que un administrador las activa.
- Eres responsable de la actividad que se realice con tu cuenta.

**3. Uso permitido**
Al usar la plataforma te comprometes a no:
- Copiar, publicar, vender o distribuir las preguntas y materiales fuera de la plataforma.
- Extraer contenido de forma automatizada o masiva.
- Intentar vulnerar la seguridad, el funcionamiento o la integridad del sistema.
- Publicar contenido ofensivo, falso o que infrinja derechos de terceros en opiniones o comentarios.

**4. Propiedad intelectual** *(sin cambios)*
Los bancos de preguntas, recursos, textos, diseño y software de Capacitate pertenecen a sus titulares y están protegidos por la normativa de propiedad intelectual. El acceso a la plataforma no te transfiere ningún derecho sobre ellos.

**5. Contenido que publicas** *(sin cambios)*
Las opiniones y testimonios que envíes deben ser tuyos. Al publicarlos autorizas a Capacitate a mostrarlos dentro de la plataforma y, una vez aprobados por un administrador, también en la página pública de inicio, con tu nombre y la inicial de tu apellido. Los administradores pueden ocultar o retirar el contenido que incumpla estos términos.

**6. Resultados y disponibilidad**
Los puntajes de prácticas y simulacros son solo una referencia para tu estudio. Capacitate no garantiza que apruebes, ascedas ni obtengas un resultado determinado en el proceso real. Procuramos que la plataforma esté disponible y que el contenido sea correcto, pero puede haber interrupciones por mantenimiento o errores que corregiremos en cuanto los detectemos.

**7. Suspensión de cuentas** *(sin cambios)*
Podemos suspender o dar de baja una cuenta que incumpla estos términos, que use datos falsos o que ponga en riesgo la seguridad de la plataforma o de otros usuarios.

**8. Cambios en los términos** *(sin cambios)*
Podemos actualizar estos términos. Publicaremos la versión vigente en esta página y, si el cambio es relevante, te lo avisaremos dentro de la plataforma.

**9. Ley aplicable** *(sin cambios)*
Estos términos se rigen por las leyes de la República del Perú. Cualquier controversia se someterá a los jueces y tribunales competentes del Perú.

**10. Contacto** *(sin cambios)*
Para cualquier consulta sobre estos términos, escríbenos a {CONTACT_EMAIL}.

## Política de privacidad

**Intro** *(sin cambios)*: Esta política explica qué datos personales recoge Capacitate, para qué los usa y cómo los protege, conforme a la Ley N.º 29733, Ley de Protección de Datos Personales, y su reglamento.

**1. Datos que recogemos**
- Datos de registro: nombres, apellidos, documento de identidad, grado o nivel, si tu institución lo usa, correo electrónico y teléfono.
- Datos de uso: prácticas y simulacros realizados, respuestas, puntajes y tiempos.
- Opiniones y testimonios que decidas enviar.
- Datos técnicos para mantener tu sesión y tus preferencias.
- Por cada sesión y en el registro de auditoría guardamos la dirección IP y el user-agent (el navegador y el dispositivo que usas), con fines de seguridad y auditoría. Los administradores de la plataforma pueden verlos.

**2. Para qué los usamos** *(sin cambios)*
- Crear y activar tu cuenta y verificar tu identidad.
- Mostrarte tu progreso, estadísticas y resultados.
- Enviarte avisos sobre tu cuenta, como la activación o el cambio de contraseña.
- Mejorar los bancos de preguntas y el funcionamiento de la plataforma.

**3 a 9** *(sin cambios: con quién los compartimos, cookies y almacenamiento local, seguridad, conservación, derechos, cambios y contacto, tal como están hoy)*

---

# Parte B — Adiciones opcionales (requieren decisión)

Cada una se agrega solo si el usuario la aprueba **y** si su historia dependiente ya está en producción (si no, el texto sería falso).

| # | Adición propuesta | Pregunta al usuario | Depende de |
|---|---|---|---|
| P1 | *(Incorporada a la parte A por indicación del usuario del 15-nov: Capacitate es solo una ayuda de estudio, no certifica ni tiene vínculo con las instituciones. Ya no es opcional.)* | — | — |
| P2 | Términos 2: «Indica la institución a la que perteneces. Solo puedes inscribirte en los procesos de tu institución.» | ¿Se quiere que sea un compromiso expreso? | PLT-06 |
| P3 | Privacidad 1: agregar «institución» a los datos de registro. | Es un dato que ya se guarda (la persona tiene institución); solo hay que decidir si se nombra. | — |
| P4 | Privacidad 1 y 2: «idioma de preferencia» como dato y «avisos de cuenta e inscripciones (activación, aprobación o rechazo con su motivo) en el idioma que elegiste». | ¿Se publica junto con las historias? | PLT-11, PLT-12, PLT-13 |
| P5 | Privacidad 3: «No compartimos tus datos ni resultados con otras instituciones.» | **No se afirma por defecto**: los administradores de la plataforma ven datos de todas las instituciones. ¿Qué se desea prometer? | — |
| P6 | Privacidad: mención al uso de IA (generación de preguntas o plan de estudio) y proveedor. | El texto actual no la menciona. Si hay funciones de IA que tratan datos de usuarios, **debe añadirse antes de publicar** (ver SERUMS IA E9-07). | SERUMS IA |
| P7 | Privacidad intro: «Responsable del tratamiento: [ENTIDAD RESPONSABLE]» (razón social y RUC). | No figura hoy. La Ley N.º 29733 lo exige; el usuario debe dar el dato. | — |
| P8 | «Última actualización: [fecha]» en ambos documentos. | La fecha la fija el tech-lead al versionar. | — |

---

## Criterios de aceptación (cuando el usuario apruebe)
```gherkin
Escenario: Aclaración de alcance
  Dado la parte A publicada
  Cuando se lee la sección 1 de los términos y la sección 6
  Entonces dicen que Capacitate no certifica, no otorga certificados, constancias, puntajes ni resultados con validez, no tiene vínculo con las instituciones, no garantiza aprobar ni ascender, y que preguntas y simulacros son material de práctica

Escenario: Palabras evitadas
  Dado los textos de las partes A y B
  Cuando se buscan "oficial", "certificado", "acreditado" y similares
  Entonces solo aparecen en frases que niegan esa condición (p. ej. «no es una entidad certificadora»)

Escenario: Vocabulario neutro
  Dado el texto de la parte A publicado
  Cuando se buscan "ascenso", "personal" y "grado" en ambas páginas
  Entonces "ascenso" aparece solo como ejemplo («como ascensos o admisiones»), "personal" no aparece como sujeto y "grado" aparece solo como «grado o nivel, si tu institución lo usa»

Escenario: Numeración conservada
  Dado la parte A publicada
  Cuando se comparan las secciones con el original
  Entonces tienen la misma numeración y orden (términos 1–10, privacidad 1–9)

Escenario: Aviso en inglés
  Dado un visitante con la interfaz en inglés
  Cuando abre términos o privacidad
  Entonces ve el título en inglés, el aviso "Spanish only" y el cuerpo en español

Escenario: Sin aviso en español
  Dado un visitante con la interfaz en español
  Cuando abre ambas páginas
  Entonces no ve el aviso

Escenario: Adiciones solo si fueron aprobadas
  Dado una adición de la parte B no aprobada o cuya historia no está en producción
  Cuando se revisa el texto publicado
  Entonces no aparece

Escenario: Sin marcadores pendientes
  Dado el texto listo para publicar
  Cuando se buscan "[ENTIDAD RESPONSABLE]" y "[fecha]"
  Entonces no hay coincidencias
```

## Fuera de alcance
Traducción al inglés de los textos; revisión legal (la hace un abogado); cambios en el frontend hasta la aprobación; casilla de aceptación expresa en el registro (si no existe, historia aparte).
