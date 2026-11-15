---
id: PLT-13
titulo: Correos de cuenta activada, rechazada y desactivada en el idioma del usuario
epica: PLT-E3
fase: MVP
prioridad: Media
estimacion: 2
roles: [postulante, admin]
depende_de: [PLT-11]
---

# PLT-13 Correos de cuenta activada, rechazada y desactivada en el idioma del usuario

**Como** usuario, **quiero** que los correos de activación, rechazo y desactivación de mi cuenta lleguen en mi idioma, **para** entenderlos aunque los dispare un admin.

## Contexto
Los correos de cuenta activada y desactivada están escritos fijos en español (`backend/internal/auth/email_templates.go:96-151`). Decisiones: [README, PLT-E3](README.md#épica-plt-e3-aviso-de-decisiones-al-postulante).

## Reglas de negocio
1. Las plantillas de cuenta activada, **cuenta rechazada** (`emailRechazado`) y cuenta desactivada, dentro de `email_templates.go:96-151`, pasan a tener versión es y en (P7); se usa el idioma guardado del usuario (no el del admin que dispara la acción).
2. Fallback a `es`.
3. El contenido, enlaces y destinatarios no cambian; solo el idioma.
4. Las plantillas de registro y recuperación no se tocan (ya usan `locale`).

## Criterios de aceptación
```gherkin
Escenario: Activación en inglés
  Dado un usuario pendiente con idioma "en"
  Cuando un admin con la interfaz en español activa su cuenta
  Entonces el correo de cuenta activada está en inglés

Escenario: Desactivación en español
  Dado un usuario con idioma "es"
  Cuando un admin con la interfaz en inglés desactiva su cuenta
  Entonces el correo de cuenta desactivada está en español

Escenario: Rechazo de cuenta en inglés
  Dado un usuario pendiente con idioma "en"
  Cuando un admin con la interfaz en español rechaza su cuenta
  Entonces el correo de cuenta rechazada está en inglés

Escenario: Idioma ausente
  Dado un usuario sin idioma válido
  Cuando se activa su cuenta
  Entonces el correo sale en español

Escenario: Sin regresión
  Dado un usuario cualquiera
  Cuando se registra o recupera su contraseña
  Entonces los correos siguen en el idioma de la petición como hoy
```

## Fuera de alcance
La plantilla de «correo cambiado» (P7); nuevos correos de cuenta; rediseñar las plantillas; reactivación con motivo.

## Seguridad y privacidad
Sin datos nuevos en el correo: mismos destinatarios y enlaces que hoy; los logs no registran el cuerpo.

## i18n
Asunto y cuerpo de las tres plantillas en es/en, con los mismos datos.

## Definición de terminado
Tests de plantilla por idioma y de fallback; capturas es/en en el PR. Review limpio.
