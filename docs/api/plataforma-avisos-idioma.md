# Contrato de API — Idioma guardado y avisos de decisión (PLT-11, PLT-12, PLT-13)

Estado: borrador para revisión (2026-11-15). ADR: [0011](../adr/0011-idioma-guardado-y-avisos-personales.md). Historias:

- [PLT-11](../stories/plataforma/PLT-11-idioma-del-usuario.md), [PLT-12](../stories/plataforma/PLT-12-aviso-decision-inscripcion.md) y [PLT-13](../stories/plataforma/PLT-13-correos-cuenta-idioma-usuario.md).
- Las decisiones de la épica están en el [README de plataforma](../stories/plataforma/README.md), sección PLT-E3.
- El PO confirmó P1–P11 con las decisiones por defecto (2026-11-15). Amplía [auth-idioma-correos.md](auth-idioma-correos.md) (§3 «Fuera de alcance» y §5 quedan reemplazados por §1.4 de este documento).

Formato de error 2 (README §3) **solo** en las respuestas que se indican. El frontend las lee con `apiErrorFrom` (`frontend/src/lib/http/api-error.ts`).

## 0. Decisiones (confirmadas por el PO el 2026-11-15 con estos valores por defecto)

| # | Pregunta | Por defecto |
|---|---|---|
| P6 | Hoy el servidor no exige el motivo del rechazo ni le pone límite (`inscripciones/handler.go:187-194`; `observacion TEXT`, `011_create_inscripciones.sql:7`) | El servidor lo exige al rechazar, de 1 a 500 caracteres. Al aprobar es opcional, de hasta 500 |
| P11 | PLT-11 dice «se fija al registrarse», pero el usuario se crea en verify-email (`auth/service.go:326-331`), no en register | El idioma se fija con el `locale` de `POST /auth/verify-email`, el idioma de la interfaz al terminar el registro |
| P7 | ¿Qué correos entran en PLT-13? | Activada, **rechazada** (`emailRechazado`, `email_templates.go:119`, dentro del rango :96-151 que cita la historia) y desactivada. «Correo cambiado» no entra |

## 1. PLT-11 Idioma del usuario

### 1.1 Modelo (migración 069)

`usuarios.idioma VARCHAR(2) NOT NULL DEFAULT 'es'`, con la restricción `usuarios_idioma_check CHECK (idioma IN ('es','en'))`. Los usuarios existentes quedan en `es`.

### 1.2 Dónde se fija

| Momento | Origen | Regla |
|---|---|---|
| Alta del usuario | `POST /auth/verify-email` (crea el usuario, `auth/repository.go:377`) | `locale.Normalize(req.Locale)`: ausente o inválido → `es` |
| Cambio con sesión | `PUT /auth/idioma` (§1.3) | El valor exacto; uno inválido da 422 |
| Seeds y altas por admin | — | El default `es` |

- `POST /auth/register`, `/auth/forgot-password` y `/auth/reset-password` **no** cambian el idioma guardado: son públicos (ADR 0011, opción 1B).
- Su `locale` sigue decidiendo solo el correo de ese request.

### 1.3 `PUT /auth/idioma` (autenticado, cualquier rol; `auth/routes.go`)

```json
{ "idioma": "en" }
```

Respuesta `200 {"idioma": "en"}`. Es idempotente y no se audita, porque es una preferencia del propio usuario (README §7). El usuario sale de `claims.UserID`: no hay ningún id en la ruta ni en el cuerpo.

| HTTP | `error` (formato 2) | Cuándo |
|---|---|---|
| 400 | `cuerpo_invalido` | JSON mal formado, campo desconocido o `idioma` que no es string (`httpx.DecodeJSONStrict`) |
| 401 | (middleware) | Sin sesión |
| 413 | `cuerpo_demasiado_grande` | README §3 |
| 422 | `idioma_invalido` | `idioma` no es exactamente `es` ni `en` (p. ej. `fr`, `EN`, `""`). El valor guardado no cambia |
| 500 | `error_interno` | — |

**`GET /auth/me` y el `user` de `POST /auth/login`:** `UserProfile` (`auth/dto.go:28-42`, consulta `GetUserProfileByID`, `auth/repository.go:1377`) añade `"idioma": "es"`.

### 1.4 Precedencia del idioma en los correos (reemplaza `auth-idioma-correos.md` §5)

1. Correos que la persona pide desde la UI (registro, verificación, recuperación y restablecimiento): el `locale` del request. Si no llega, `es`. No cambia.
2. Correos que dispara el admin o el sistema (cuenta activada, rechazada o desactivada, y la decisión de una inscripción): `usuarios.idioma`, pasado por `locale.Normalize`. Por eso, un valor ausente o raro cae a `es`.

### 1.5 Frontend

- `frontend/src/components/app/barra-idioma.tsx:34`: después de `setClientLocale(valor)`, **solo si hay sesión**, se llama `PUT /auth/idioma` sin esperar la respuesta.
- Un error se ignora en la UI: no hay toast ni mensaje técnico (PLT-11, escenario 6).
- Sin sesión, no hay llamada.
- No se aplica el idioma guardado a la cookie al iniciar sesión (fuera de alcance).

## 2. PLT-12 Aviso de aprobación o rechazo

### 2.1 `PATCH /admin/inscripciones/{id}/estado` (admin; `inscripciones/routes.go:29`)

Request sin cambios: `{"estado": "aprobado" | "rechazado", "observacion": "…"}`. Respuesta `200 {"message": "Inscripción actualizada."}` sin cambios.

**Validación nueva** (P6; `TrimSpace`; longitud en runas):

- **`rechazado`:** `observacion` es obligatoria, de 1 a 500.
- **`aprobado`:** `observacion` es opcional, de hasta 500. Vacía se guarda como hoy.

**Todos los errores pasan al formato 2:**

| HTTP | `error` | Cuándo |
|---|---|---|
| 400 | `parametro_invalido` | `{id}` no numérico o ≤ 0 |
| 400 | `cuerpo_invalido` | JSON mal formado o `estado` ausente |
| 404 | `no_encontrado` | La inscripción no existe |
| 409 | `transicion_invalida` | Ya no está `pendiente`. `detalle`: `{"estado_actual": "aprobado"}`. **No se envía ningún aviso** |
| 422 | `validacion` | `estado` distinto de los dos valores, u `observacion` de más de 500. `detalle`: `{"campo": "estado" \| "observacion"}` |
| 422 | `motivo_requerido` | `rechazado` sin `observacion` (en blanco o vacía) |
| 500 | `error_interno` | Incluye un fallo de la BD al crear el aviso: la decisión se revierte |

### 2.2 Efecto, en la misma transacción que la decisión

La guarda es `WHERE id=$1 AND estado='pendiente'` (`inscripciones/repository.go:228-232`). Solo cuando cambia una fila (`n == 1`), en este orden:

1. **Audit:** igual que hoy (`cambiar_estado_inscripcion`, `inscripciones/handler.go:195-199`).
2. **Datos del aviso:** una consulta trae `u.idioma`, `pe.email` (cifrado), `pe.nombres`, `pr.nombre` (proceso) e `i.nombre` (institución del proceso).
3. **Notificación personal:** se crea **siempre** (§2.3).
4. **Correo:** si `pii.Decrypt(gcm, email, "email")` da un valor no vacío, se hace un `INSERT INTO email_queue (to_address, subject, body_html)` renderizado en el idioma (§2.4).
   - `pii.Decrypt` devuelve `""` tanto para un valor vacío como para uno que no se puede descifrar, y ya registra el aviso sin el valor (`db/pii/pii.go:19-33`).
   - Con `""` no se encola: se registra `op` e `inscripcion_id` y la decisión sigue.

- **Fallos:** un fallo de **envío** posterior (SMTP) no revierte nada. El worker reintenta (`notifier/email_worker.go`) y la salud del correo lo muestra (PLT-03).
- **Repeticiones y reinscripciones:** repetir la acción da 409 y no avisa. Una reinscripción tras un rechazo vuelve a `pendiente` (`repository.go:123-135`), así que la siguiente decisión genera un aviso nuevo.
- **Logs:** nunca llevan el motivo, el email ni el cuerpo; solo `op`, `inscripcion_id` y `usuario_id`.

### 2.3 Notificación en la app (migración 070)

**Modelo nuevo de `notificaciones`:**

- `usuario_id BIGINT NULL REFERENCES usuarios(id) ON DELETE CASCADE`;
- `clave VARCHAR(60) NULL`;
- `parametros JSONB NULL`.

**Restricciones:**

- `notificaciones_personal_check CHECK ((destinatario = 'usuario') = (usuario_id IS NOT NULL))`;
- `notificaciones_parametros_check CHECK (parametros IS NULL OR clave IS NOT NULL)`.

**Índice:** `idx_notificaciones_usuario ON notificaciones (usuario_id, created_at DESC) WHERE usuario_id IS NOT NULL`.

**Fila creada:**

| Columna | Valor |
|---|---|
| `destinatario` | `'usuario'` |
| `usuario_id` | El postulante de la inscripción |
| `tipo` | `'sistema'` (valor existente; la UI no necesita un icono nuevo) |
| `clave` | `inscripcion_aprobada` o `inscripcion_rechazada` |
| `parametros` | `{"proceso": "<procesos.nombre>", "institucion": "<instituciones.nombre>", "motivo": "<observacion>"}`. `motivo` solo va al rechazar |
| `titulo`, `cuerpo` | El texto renderizado en `usuarios.idioma`, como respaldo. `titulo` ≤ 200 |
| `creado_por` | El admin que decide |

**`GET /notificaciones`** (`notifications/routes.go:20`): sigue siendo un array, con hasta 100 elementos y los más nuevos primero. Incluye las personales del usuario y cada ítem añade dos campos:

```json
{ "id": 91, "titulo": "Inscripción rechazada", "cuerpo": "…", "tipo": "sistema", "destinatario": "usuario",
  "created_at": "2026-11-15T15:04:05Z", "leida": false,
  "clave": "inscripcion_rechazada",
  "parametros": { "proceso": "Proceso 2026", "institucion": "Institución A", "motivo": "Documento ilegible" } }
```

- `clave` y `parametros` valen `null` en las notificaciones de difusión.
- **Visibilidad** en `ListForUser`, `UnreadCount`, `MarkRead` y `MarkAllRead` (`notifications/repository.go:68-149`): `destinatario = 'todos' OR destinatario = 'rol:'||rol OR usuario_id = $usuario`. Marcar como leída una personal ajena da `404`, como hoy.
- **Admin:** `GET /admin/notificaciones` (`ListAll`, `:36-48`) añade `WHERE usuario_id IS NULL`. `DELETE /admin/notificaciones/{id}` de una personal da `404`. `POST /admin/notificaciones` no cambia: su `oneof` no admite `usuario` (`notifications/dto.go:27`).

**Frontend (campana):**

- Si `clave` tiene traducción, muestra `t('…plantillas.<clave>.titulo')` y `t('…plantillas.<clave>.cuerpo', parametros)` en el idioma **actual**. Si no, muestra `titulo` y `cuerpo`.
- `parametros.motivo` se interpola como texto, nunca como HTML.
- El enlace sale de la clave: las dos van a `/procesos`.
- Las claves van en es y en, bajo `dashboard`.

### 2.4 Correo

- **Plantillas:** en `backend/internal/inscripciones/correo.go`. Usan el layout que se mueve de `auth/email_templates.go:11-89` a `backend/internal/correo` (ADR 0011, decisión 6): marca «Capacitate», envoltura, botón, recuadro y pie, ya en es y en.
- **Contenido:** saludo con `nombres`, nombre del proceso, institución, el **motivo** tal cual en un recuadro (solo al rechazar) y un botón a `{FRONTEND_BASE_URL}/procesos`.
- **Cableado (backend-dev):** hoy `inscripciones` no recibe la URL del frontend (`cmd/server/main.go:100-101`). `inscripciones.NewService` (o `NewRepository`) pasa a recibir `cfg.FrontendBaseURL` (`internal/config/config.go:48`, ya sin barra final, `:178`), igual que `auth` (`frontendBaseURL`, `auth/service.go:257,510`).
- **Escapado:** todo valor de la BD pasa por `html.EscapeString` (H130).
- **Destinatario:** solo el postulante. No hay copia ni datos de otras personas.
- **Texto neutro** (decisión 10 del PO): «tu inscripción a la práctica del proceso … fue aprobada o rechazada». No se usan «oficial», «certificado» ni «acreditado», ni nada que sugiera un vínculo con la institución. El PO revisa el texto final en el PR.

| | es | en |
|---|---|---|
| Asunto (aprobada) | «Tu inscripción fue aprobada: {proceso}» | «Your enrollment was approved: {proceso}» |
| Asunto (rechazada) | «Tu inscripción fue rechazada: {proceso}» | «Your enrollment was rejected: {proceso}» |

El asunto se recorta a 255 caracteres (`email_queue.subject VARCHAR(255)`).

### 2.5 Pruebas mínimas

- Aprobar con idioma `es` → un correo en español y una notificación sin leer con `clave` y enlace.
- Rechazar con `en` → los dos en inglés, con el motivo literal.
- Idioma ausente o inválido → `es`. La BD no deja guardar un valor inválido (CHECK de la 069), así que se prueba con la unidad que elige la plantilla, pasándole `""` o `"fr"`.
- Repetir la aprobación → 409, sin una fila nueva en `email_queue` ni en `notificaciones`.
- Con SMTP caído, la decisión se guarda, la notificación existe y el correo queda `pending`.
- Persona sin email → la notificación existe y no hay fila de correo.
- Rechazar sin motivo → 422 `motivo_requerido`.
- Reinscripción y nuevo rechazo → un aviso nuevo con el motivo nuevo.
- Otro usuario no ve la notificación ni la puede marcar (404). El panel admin no la lista.

## 3. PLT-13 Correos de cuenta en el idioma del usuario

- **Quién y en qué orden (tech-lead, 2026-11-15):** backend-dev-2, después de PLT-11 y de las partes de backend del #274. **Se rebasea sobre PLT-12**, porque PLT-12 mueve el layout de `auth/email_templates.go` a `internal/correo` (§2.4).
- **Cambio en `auth/service.go:504-521`:** se usa `profile.Idioma`, que añade PLT-11, normalizado con `locale.Normalize`. `emailActivado`, `emailRechazado` y `emailInactivo` reciben `loc` y tienen texto en es y en.
- **Idioma:** manda el del usuario afectado, no el del admin.
- **Sin cambios:** destinatarios, enlaces, contenido, la API y las cuatro plantillas pedidas desde la UI.
- **Pruebas:** cada plantilla en los dos idiomas, la caída a `es` y la regresión de registro y recuperación.

## 4. Migraciones y orden

| N.º | Archivo | Historia | Efecto en los datos | Lo que pierde el Down |
|---|---|---|---|---|
| — | (sin migración) | PLT-06 | — | — |
| **069** | `069_add_usuarios_idioma.sql` | PLT-11 | Los existentes quedan en `es` (DEFAULT constante, sin reescribir la tabla) | La preferencia |
| **070** | `070_add_notificaciones_personales.sql` | PLT-12 | Ninguno: las filas existentes tienen `usuario_id` NULL y `destinatario` `todos` o `rol:*`, así que cumplen el CHECK | Primero `DELETE … WHERE usuario_id IS NOT NULL` (sin la columna quedarían invisibles), luego quita el índice, los CHECK y las columnas |
| 071 | (antes 069) | #28 | — | — |
| 072 | (antes 070) | #52 | — | — |
| sin reservar | `…_create_catalogos_institucion.sql` | PLT-07 (**diferida**) | Toma el siguiente número libre cuando se retome ([plataforma-multi-institucion.md](plataforma-multi-institucion.md) §8) | — |

- **Numeración confirmada por el tech-lead (2026-11-15, después del recorte de alcance).** El merge es estricto, en orden numérico (goose sin allow-missing): 069 → 070. No se intercambian números.
- **Orden de los PR:** PLT-11 (069) → PLT-12 (070) → PLT-13. PLT-13 se rebasea sobre PLT-12, porque PLT-12 mueve el layout de correo a `internal/correo` (§2.4).
- **Compatibilidad:** un binario viejo funciona con el esquema nuevo de las dos.
  - 069: columna con default.
  - 070: el binario viejo no ve las notificaciones personales, porque no son `todos` ni `rol:*`.
