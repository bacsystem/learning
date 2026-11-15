# Contrato de API — idioma de los correos de auth (#247, D4)

Estado: borrador para revisión (2026-11-15). Origen: decisión D4 del po (`docs/qa/decisiones-247.md`, worktree de qa): los correos de auth que la persona dispara desde la UI salen en el idioma que usaba al pedirlos; si no se conoce, en español. La ampliación a reset-password y verify-email también la decidió el po el 2026-11-15, sin persistir el idioma.

> **Enmienda (2026-11-15, PLT-11/PLT-13):** el idioma pasa a guardarse en `usuarios.idioma`, en [plataforma-avisos-idioma.md](plataforma-avisos-idioma.md) §1. Los cuatro endpoints de §3 **no cambian**: su `locale` decide solo el correo del request. La única novedad es que `POST /auth/verify-email` también usa su `locale` para fijar el idioma del usuario que crea. Quedan reemplazados el «Fuera de alcance» de §3 (los correos que dispara el admin pasan a usar el idioma guardado, PLT-13) y la regla de precedencia de §5 (ahora vigente, en `plataforma-avisos-idioma.md` §1.4). En §6, la fila «Admin activa a un usuario → español» queda reemplazada: el correo sale en el idioma guardado del usuario (PLT-13).

## 1. Situación actual (código)

- El frontend guarda el locale en la cookie `${NEXT_PUBLIC_STORAGE_PREFIX}-locale` (CLAUDE.md, i18n) y **no** se lo envía al backend: `forgotPassword` manda solo `{ email }` (`frontend/src/lib/auth/auth-api.ts:113-118`) y `register`, `{ dni, email }` (`auth-api.ts:81-88`).
- La cookie tampoco llega al backend: `request()` hace `fetch` a `${API_BASE}` sin `credentials: "include"` (`auth-api.ts:49-57`). El contrato no depende de ella.
- Las plantillas solo existen en español y se renderizan **al encolar**, así que `email_queue` no necesita guardar el idioma:

  | Plantilla | Definición (`backend/internal/auth/email_templates.go`) | Se renderiza en (`service.go`) |
  |---|---|---|
  | `emailVerificarRegistro` | `:165` | `:257` (registro) |
  | `emailOlvidePassword` | `:187` | `:646-647` (recuperación) |
  | `emailPasswordRestablecida` | `:209` | `:682` (reset) |
  | `emailRegistroPendiente` | `:135` | `:339` (verify-email) |
- No hay ninguna columna ni tabla de idioma o preferencias en `backend/migrations/` (búsqueda de `idioma|locale|preferenc`: sin resultados).
- Hoy, `decodeJSON` del paquete auth **no** usa `DisallowUnknownFields` (`backend/internal/auth/handler.go:54-70`). **Eso va a cambiar con el #236:** pasará a `httpx.DecodeJSONStrict` (`backend/internal/httpx/httpx.go:44-46`), que rechaza los campos desconocidos con 400. Desde entonces, un frontend que envíe `locale` a un backend sin ese campo recibe 400. Por eso la compatibilidad depende del **orden de merge** de §4.

## 2. Decisión: campo opcional `locale` en el body

| Opción | Veredicto |
|---|---|
| **Campo `locale` en el body JSON** | **Elegida.** Lleva exactamente el idioma elegido en la UI (el valor de la cookie), es explícito, fácil de probar y no depende del navegador |
| Cabecera `Accept-Language` | Descartada. Refleja el idioma del navegador o del sistema, no el elegido en el selector de la app: con la UI en `en` y el navegador en `es-PE`, el correo saldría en español y fallaría el criterio de D4. Además obliga a parsear pesos `q` y variantes regionales (YAGNI) |
| Leer la cookie en el backend | Descartada: hoy no se envía (§1). Enviarla exigiría `credentials: "include"` y CORS con credenciales en endpoints públicos, a cambio de nada |

## 3. Endpoints afectados

### `POST /auth/forgot-password` (público, `routes.go:29`, con `authRL`)

Request:

```json
{ "email": "persona@example.com", "locale": "en" }
```

| Campo | Tipo | Obligatorio | Regla |
|---|---|---|---|
| `email` | string | sí | Sin cambios (`dto.go:50-52`) |
| `locale` | string | **no** | Lista cerrada `es`, `en`. Ver §4 |

Respuesta: **sin cambios.** Siempre `200 {"message": "Si el email está registrado, …"}` (`handler.go:505-506`), con o sin `locale` y sea cual sea su valor. El campo no cambia la respuesta ni su tiempo: se mantiene el anti-enumeración.

Errores: los mismos de hoy (415, 413, 400 de sintaxis, 422 del `email`, 429 del limitador). **Un valor string de `locale` nunca produce un error.** Un tipo que no sea string (p. ej. `"locale": 123`) es un error de sintaxis como en cualquier otro campo: `400 "Cuerpo de solicitud inválido."` (`handler.go:60-65`).

Efecto: el correo (asunto, cuerpo y texto del botón) sale en el idioma resuelto. Cabecera y pie con «Capacitate» en los dos idiomas (D4, marca).

### `POST /auth/register` (público, `routes.go:26`)

Mismo campo, misma regla: `{ "dni": "12345678", "email": "…", "locale": "en" }`. Decide el idioma del correo de verificación (`emailVerificarRegistro`, `service.go:257`). La respuesta 202 genérica y sus errores no cambian.

### `POST /auth/reset-password` (público, `routes.go:30`)

Mismo campo, misma regla: `{ "token": "…", "password": "…", "confirm_password": "…", "locale": "en" }` (`ResetPasswordRequest`, `dto.go:54-58`). Decide el idioma del correo «contraseña restablecida» (`emailPasswordRestablecida`, `service.go:682`). La respuesta y los errores no cambian, incluido el 400 de token inválido (`handler.go:519-522`).

### `POST /auth/verify-email` (público, `routes.go:27`)

Mismo campo, misma regla: `{ "token": "…", "username": "…", "password": "…", "confirm_password": "…", "locale": "en" }` (`VerifyEmailRequest`, `dto.go:12-17`). Decide el idioma del correo «registro pendiente de activación» (`emailRegistroPendiente`, `service.go:339`). La respuesta 201 y los errores (400, 409, 422, 429) no cambian.

### Fuera de alcance

- Los correos que dispara el admin no vienen de un request de la persona, así que no hay un idioma del que partir. Siguen en `es` hasta SIA-E8-02 (§5), aunque también llevan la marca «Capacitate». Son:
  - activación, rechazo e inactivación (`service.go:510-517`);
  - cambio de correo (`emailCorreoCambiado`, `email_templates.go:230`).
- Sin migración y sin columnas nuevas.

## 4. Normalización del `locale` (servidor)

1. Ausente, `null` o `""` → `es`.
2. Se aplica `TrimSpace` y minúsculas. Si el resultado es `es` o `en`, se usa ese.
3. Cualquier otro valor (`fr`, `en-US`, `EN_us`, una cadena larga) → `es`. No se valida con `validate:"oneof=…"`, porque eso daría 422 y rompería el criterio «locale inválido → español».
4. No hace falta un límite propio: el cuerpo ya está acotado por `maxJSONBody` (`handler.go:59`) y el valor nunca se guarda ni se registra.

- **Implementación (backend-dev):** una función única `Normalize(string) string` con las constantes `ES` y `EN` en un paquete pequeño, `backend/internal/locale`. Así E8-02 la reutiliza desde notificaciones y el worker sin depender de `auth`. Se añade `Locale string \`json:"locale"\`` (sin tag `validate`) a `RegisterRequest`, `VerifyEmailRequest`, `ForgotPasswordRequest` y `ResetPasswordRequest`. El valor normalizado llega a `Service.Register`, `Service.VerifyEmail`, `Service.ForgotPassword` y `Service.ResetPassword`: un parámetro más, o el propio DTO en `VerifyEmail` (`service.go:325`). Las cuatro plantillas reciben el locale y eligen el texto.
- **Frontend (frontend-dev):** `register`, `verifyEmail`, `forgotPassword` y `resetPassword` (`auth-api.ts:81-124`) envían `locale` con el valor activo de next-intl (`useLocale()`), que ya es `es` o `en`. Si falta, se omite el campo.
- **Orden de merge (obligatorio, H1 del review del #254):**
  1. Primero se mergea el PR de backend, con `Locale` en los 4 DTOs. Un backend con el campo acepta a los clientes que no lo envían.
  2. Después, el PR de frontend que envía `locale`.

  El orden vale tanto si el #236 (`DecodeJSONStrict` en auth) se mergea antes como después. Si se mergea antes, invertir el orden da 400 en los cuatro endpoints, también en recuperación y registro. Si se mergea después, el #236 debe partir de un `develop` que ya tenga `Locale` en esos DTOs.
- **Logs y auditoría:** sin cambios. El `locale` no es dato personal, pero no aporta y no se añade.

## 5. Compatibilidad con SIA-E8-02 (idioma preferido)

E8-02 pide «notificaciones y emails en el idioma preferido del usuario (es/en)» (`docs/stories/serums-ia/E8-02-alertas-procesos.md:112`, y el worker «envía en el idioma del usuario», `:51`). Este contrato no le cierra el camino:

- **Mismos valores y la misma función:** `es`/`en` y `locale.Normalize`. La futura preferencia persistida usará esa lista cerrada y esos códigos.
- **No se persiste ahora** (respuesta a la pregunta del po en D4). Motivos:
  - Los cuatro endpoints son públicos: guardar el `locale` en el usuario permitiría a cualquiera que conozca un email cambiar la preferencia de otra persona.
  - La preferencia de E8-02 se fijará desde un endpoint autenticado (p. ej. ajustes o el selector de idioma con sesión). Lo diseña esa historia.
- **Regla de precedencia (propuesta para E8-02; no vigente hasta que esa historia la adopte):**
  - en un correo que la persona pide desde la UI (los cuatro de §3), manda el `locale` del request;
  - si no llega, se usa la preferencia guardada, cuando exista;
  - si tampoco hay preferencia, `es`.
  - Los correos que dispara el sistema o el admin usan la preferencia guardada, y si no hay, `es`.

## 6. Pruebas mínimas (D4)

| Caso | Esperado |
|---|---|
| `locale: "en"` en forgot-password | Asunto, cuerpo y botón en inglés; «Capacitate» |
| `locale: "es"` | Correo en español; «Capacitate» |
| Sin `locale`, y `locale: "fr"` | Correo en español; respuesta 200 idéntica |
| `locale: 123` | 400 de sintaxis (igual que hoy con un tipo inválido) |
| `locale: "en"` en register | Correo de verificación en inglés |
| `locale: "en"` en reset-password | «Contraseña restablecida» en inglés |
| `locale: "en"` en verify-email | «Registro pendiente» en inglés |
| Admin activa a un usuario | Correo en español con «Capacitate» (sin cambio de idioma) |
| Unitaria de `locale.Normalize` | `""`, `" EN "`, `"en-US"`, `"fr"` → `es`, `en`, `es`, `es` |
