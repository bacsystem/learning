---
id: PLT-03
titulo: Salud del correo y versión del frontend
epica: PLT-E1
fase: MVP
prioridad: Media
estimacion: 2
roles: [admin]
depende_de: [PLT-01, PLT-02]
---

# PLT-03 Salud del correo y versión del frontend

**Como** admin, **quiero** ver si los correos salen a tiempo y qué versión del frontend está desplegada, **para** detectar correos atascados (verificación de cuenta, recuperación de contraseña) y confirmar qué se desplegó.

## Contexto
Los correos se encolan en `email_queue` y los envía un worker por SMTP. Si no hay `SMTP_*`, quedan en la cola sin enviarse; esto solo se permite con `ENV=development`/`test` (ver `docs/puesta-en-marcha.md`). Un correo atascado impide completar un registro, así que es lo más valioso de vigilar después de la base de datos.

## Reglas de negocio
1. **Correo (`correo`):** el estado sale de `email_queue`, sin abrir una conexión SMTP en cada comprobación. Definiciones:
   - **Sin enviar:** estado `pending` o `sending`, o `failed` con reintentos restantes. La antigüedad se mide desde que se encoló.
   - **Fallido definitivo:** `failed` que agotó sus reintentos. Hoy son 5 intentos con backoff de 30 s a 1 h, unos 15 min en total.
   - **Enviado:** `sent`.
2. Estados:
   - `no_configurado`: no hay SMTP configurado (solo posible en desarrollo/test).
   - `operativo`: SMTP configurado, ningún correo sin enviar con más de 10 min y menos de 3 fallidos definitivos en las últimas 24 h.
   - `degradado` (categoría `atrasado`): el correo sin enviar más antiguo tiene > 10 min, o hay ≥ 3 fallidos definitivos en las últimas 24 h.
   - `caido` (categoría `sin_conexion`): en los últimos 30 min hay ≥ 3 fallidos definitivos y ningún enviado. Es la señal de un SMTP caído: como los reintentos se agotan en unos 15 min, un SMTP caído no deja correos «sin enviar» mucho tiempo, sino fallidos definitivos.
   - Limitación aceptada: 1 o 2 fallidos aislados (p. ej. una dirección mal escrita) no cambian el estado.
3. Datos que se muestran: nº de correos sin enviar, antigüedad del más antiguo (min), nº de fallidos definitivos en 24 h y hora del último enviado. **Nunca** destinatarios, asuntos, contenido ni `last_error`.
4. **Frontend:** tarjeta con la versión desplegada del frontend que el admin está viendo. Es el `version` de `frontend/package.json`, el mismo que ya se ve en «Acerca de», sin variable nueva. El release asegura que coincide con el tag sin la «v», en el mismo formato que el backend (architect). No tiene estado operativo/caído (decisión 10 del README): si el frontend está caído, el panel no se ve. Si la versión del frontend no coincide con la del backend, se muestra una nota informativa, «El frontend y el backend tienen versiones distintas», que no cambia el estado global.
5. El servicio `correo` cuenta para el estado global según las reglas de PLT-01. El frontend no cuenta.

## Criterios de aceptación
```gherkin
Escenario: Correo al día
  Dado SMTP configurado y 2 correos sin enviar encolados hace 1 min
  Cuando un admin abre «Salud de servicios»
  Entonces la tarjeta «Correo» está «Operativo» y muestra 2 sin enviar, la antigüedad del más antiguo y la hora del último enviado

Escenario: Correo atrasado
  Dado un correo en "failed" con reintentos restantes, encolado hace 12 min
  Cuando un admin abre la pantalla
  Entonces «Correo» está «Degradado — Atrasado» y el estado global es al menos «Degradado»

Escenario: SMTP caído
  Dado en los últimos 30 min 3 correos agotaron sus reintentos y ninguno se envió
  Cuando un admin abre la pantalla
  Entonces «Correo» está «Caído — Sin conexión» y el estado global es «Caído»

Escenario: Fallos definitivos repetidos
  Dado 3 correos agotaron sus reintentos en las últimas 24 h, se enviaron otros después y no hay correos sin enviar
  Cuando un admin abre la pantalla
  Entonces «Correo» está «Degradado» y muestra 3 fallidos en 24 h

Escenario: Fallo aislado
  Dado 1 correo agotó sus reintentos hace 3 h y no hay correos sin enviar
  Cuando un admin abre la pantalla
  Entonces «Correo» está «Operativo» y muestra 1 fallido en 24 h

Escenario: SMTP no configurado en desarrollo
  Dado ENV=development sin SMTP
  Cuando un admin abre la pantalla
  Entonces «Correo» está «No configurado» y no empeora el estado global

Escenario: Sin datos personales del correo
  Dado correos pendientes y fallidos con destinatario y error SMTP registrados
  Cuando un admin consulta el estado de salud
  Entonces la respuesta no contiene direcciones de correo, asuntos, contenido, host SMTP ni el texto del error

Escenario: Versión del frontend
  Dado un frontend desplegado con versión 1.4.0 y un backend con 1.4.0
  Cuando un admin abre la pantalla
  Entonces la tarjeta «Frontend» muestra 1.4.0, sin estado operativo/caído y sin nota de diferencia

Escenario: Versiones distintas
  Dado un frontend 1.4.0 y un backend 1.5.0
  Cuando un admin abre la pantalla
  Entonces ve la nota «El frontend y el backend tienen versiones distintas» y el estado global no cambia por eso
```

## Fuera de alcance
Prueba de conexión SMTP en vivo, reenviar o reencolar correos desde el panel, ver el detalle de los correos y comprobar la disponibilidad del frontend desde fuera (monitor externo).

## Dependencias
PLT-01, PLT-02. La versión del frontend sale del `version` de `frontend/package.json` (decisión del architect). El release la alinea con el tag.

## Seguridad y privacidad
Solo conteos y antigüedades: ni destinatarios, ni asuntos, ni cuerpo, ni `last_error`, ni host, puerto o usuario SMTP.

## i18n
Todo en es/en, bajo `dashboard.saludServicios.*`: «Correo», «Frontend», las etiquetas de pendientes, de antigüedad y de fallidos en 24 h, y la nota de versiones distintas.

## Definición de terminado
Tests de API: operativo, degradado por antigüedad (incluido `failed` con reintentos restantes), caído por fallidos sin envíos en 30 min, degradado por ≥ 3 fallidos, fallo aislado que sigue operativo, `no_configurado`, y respuesta sin datos personales ni host SMTP. `tsc`, `lint` y `build` en verde. Claves es/en. Review limpio.
