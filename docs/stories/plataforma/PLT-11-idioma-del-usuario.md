---
id: PLT-11
titulo: Guardar el idioma del usuario
epica: PLT-E3
fase: MVP
prioridad: Alta
estimacion: 2
roles: [postulante, admin]
depende_de: []
---

# PLT-11 Guardar el idioma del usuario

**Como** usuario, **quiero** que Capacitate recuerde mi idioma, **para** recibir los avisos en el idioma que uso aunque no tenga la sesión abierta.

## Contexto
Los correos de registro y recuperación usan el `locale` de la petición (`docs/api/auth-idioma-correos.md`), pero la activación la dispara el admin, así que hace falta el idioma guardado. Decisiones: [README, PLT-E3](README.md#épica-plt-e3-aviso-de-decisiones-al-postulante).

## Reglas de negocio
1. Campo de idioma en el usuario: `es` | `en`, no nulo, por defecto `es`. Migración que asigna `es` a los existentes.
2. El usuario se crea al verificar el correo, no al registrarse. Por eso el idioma se fija con el `locale` de `POST /auth/verify-email` (el idioma de la interfaz al terminar el registro) (P11). `register`, `forgot-password` y `reset-password` no cambian el idioma guardado.
3. Al cambiar el idioma con sesión iniciada, el frontend lo envía a `PUT /auth/idioma` y se guarda. Sin sesión, solo cambia la cookie. `GET /auth/me` devuelve `idioma`.
4. Un valor distinto de exactamente `es`/`en` se rechaza con 422 `idioma_invalido` en `PUT /auth/idioma` (en la verificación, ausente o inválido se cae a `es`).

## Criterios de aceptación
```gherkin
Escenario: Registro en inglés
  Dado un visitante con la interfaz en inglés que se registró
  Cuando verifica su correo (se crea su usuario)
  Entonces su usuario queda con idioma "en"

Escenario: Cambio de idioma con sesión
  Dado un postulante con idioma "es" y sesión iniciada
  Cuando cambia la interfaz a inglés
  Entonces su idioma guardado pasa a "en" y persiste al cerrar y abrir sesión

Escenario: Cambio sin sesión
  Dado un visitante en la pantalla de acceso
  Cuando cambia el idioma
  Entonces cambia solo la cookie y no se llama al backend con datos de usuario

Escenario: Valor inválido
  Dado un usuario con sesión
  Cuando envía el idioma "fr"
  Entonces recibe 422 y el idioma no cambia

Escenario: Usuarios existentes
  Dado la base antes de la migración
  Cuando se aplica la migración
  Entonces todos los usuarios tienen idioma "es"

Escenario: Fallo del backend al guardar
  Dado que guardar el idioma falla
  Cuando el usuario cambia de idioma
  Entonces la interfaz cambia igual y el fallo no bloquea ni muestra un error técnico
```

## Fuera de alcance
Más idiomas; elegir idioma de los avisos distinto al de la interfaz; migrar notificaciones antiguas.

## Seguridad y privacidad
Un usuario solo modifica su propio idioma. El idioma no es dato sensible.

## i18n
Sin texto visible nuevo.

## Definición de terminado
Tests de API (registro, cambio, valor inválido, ajeno), migración subida/bajada, `tsc`/`lint`/`build`. Contrato en `docs/api/` (ampliar `auth-idioma-correos.md`). Review limpio.
