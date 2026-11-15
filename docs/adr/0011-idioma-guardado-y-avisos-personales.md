# ADR 0011: idioma guardado del usuario y avisos personales de decisión

- **Estado:** propuesto (2026-11-15). Pendiente de la revisión del tech-lead.
- **Fecha:** 2026-11-15
- **Historias:** PLT-11, PLT-12 y PLT-13 (`docs/stories/plataforma/PLT-11-idioma-del-usuario.md`, `PLT-12-aviso-decision-inscripcion.md`, `PLT-13-correos-cuenta-idioma-usuario.md`; decisiones de la épica en `docs/stories/plataforma/README.md`, sección PLT-E3)
- **Contrato:** [docs/api/plataforma-avisos-idioma.md](../api/plataforma-avisos-idioma.md), que amplía [auth-idioma-correos.md](../api/auth-idioma-correos.md) §5
- **Relacionado:** [ADR 0010](0010-multi-institucion-elegibilidad-y-catalogos.md)

## Contexto

1. **No hay idioma guardado.**
   - El `locale` del request solo decide los cuatro correos que la persona pide desde la UI (`auth-idioma-correos.md` §3).
   - Los correos que dispara el admin salen en español: activada, rechazada y desactivada (`auth/email_templates.go:96-157`, renderizados en `auth/service.go:509-518`).
   - `usuarios` no tiene una columna de idioma (`migrations/003_create_usuarios.sql`).
   - El usuario se crea en **verify-email**, no en register (`auth/service.go:326-331`; `INSERT INTO usuarios`, `auth/repository.go:377`).
2. **Las notificaciones solo se difunden.**
   - `destinatario` vale `todos` o `rol:<rol>`, con DEFAULT `'todos'` (`migrations/037_create_notificaciones.sql:7`).
   - Las lecturas comparan solo con esos valores (`notifications/repository.go:68-149`).
   - El panel admin lista todas las filas (`:36-48`).
   - El texto es fijo, en un idioma (`titulo` y `cuerpo`).
3. **Decidir una inscripción no avisa a nadie.**
   - `UpdateEstado` decide en una transacción con la guarda `WHERE estado='pendiente'` y audita en esa misma transacción (`inscripciones/repository.go:222-255`).
   - El texto del diálogo del admin promete un correo que no existe.
4. **Envío de correo.**
   - `email_queue` se renderiza al encolar (`007_create_email_queue.sql`) y la vacía `notifier/email_worker.go`, con reintentos.
   - Su estado se ve en la salud del correo (`salud/correo.go:20-43`).
   - Las plantillas y el layout son privados del paquete `auth` (`email_templates.go:20-89`).

## Opciones consideradas

### 1. Dónde y cuándo se guarda el idioma

- **A. Columna `usuarios.idioma`, fijada en verify-email con su `locale` y cambiada por un endpoint autenticado.** **Elegida.**
- **B. Guardarla también desde register, forgot-password o reset-password.** Descartada: son públicos, y cualquiera que conozca un email podría cambiar la preferencia de otra persona (`auth-idioma-correos.md` §5).
- **C. Una tabla `preferencias_usuario`.** Hoy hay una sola preferencia. Descartada por YAGNI.

### 2. Cómo se dirige una notificación a una persona

- **A. Codificar el id en `destinatario` (`usuario:42`).** Obliga a parsear texto, no admite FK ni índice útil, y un error de formato deja la fila visible para nadie. Descartada.
- **B. Columna `usuario_id` nullable (FK + índice parcial) más `destinatario = 'usuario'`, con un CHECK que ata los dos.**
  - El CHECK es necesario por el DEFAULT `'todos'`: una fila personal guardada con el default la verían todos, con el motivo del rechazo. Con el CHECK, la BD lo impide.
  - Un binario viejo (que solo lee `todos` y `rol:*`) no muestra las filas personales a nadie.
  - **Elegida.**
- **C. Una tabla aparte de avisos personales.** Duplicaría las lecturas, el contador y el «marcar como leído» de la campana. Descartada.

### 3. Cómo se ve una notificación en el idioma de quien la lee

- **A. Texto fijo en el idioma guardado al crearla.** Si el usuario cambia de idioma, la ve en el anterior, y no cumple la decisión 7 del PO. Descartada como mecanismo principal.
- **B. Guardar `clave` y `parametros` (JSONB); el frontend la traduce al mostrarla.** Los parámetros son datos (el nombre del proceso, de la institución y el motivo), que no se traducen. `titulo` y `cuerpo` se siguen llenando, en el idioma guardado, como respaldo para el panel y los clientes viejos. **Elegida.**

### 4. Cuándo se generan el aviso y el correo

- **A. Después del commit, sin bloquear**, como `auth.notifyUsuario` (`auth/service.go:535-554`). Si el proceso cae entre el commit y el INSERT, el correo se pierde sin registro, y no cumple «queda registrado para reintento». Descartada.
- **B. Dentro de la transacción de la decisión**, solo cuando la guarda cambia una fila (`n == 1`). `email_queue` actúa como *outbox*, de modo que la decisión, el audit, la notificación y el correo encolado se confirman o se revierten juntos.
  - La guarda ya hace que solo se avise en la transición real.
  - Un doble clic da 409 y no genera otro aviso.
  - Una reinscripción vuelve a `pendiente` (`repository.go:123-135`), y la nueva decisión vuelve a avisar.
  - **Elegida.**

### 5. Dónde viven las plantillas

- **A. Exportar plantillas nuevas desde `auth`.** Mete en `auth` correos de otro dominio. Descartada.
- **B. Duplicar el layout en `inscripciones`.** Descartada (DRY y marca).
- **C. Mover el layout (marca, envoltura, botón, recuadro y pie) a un paquete pequeño, `internal/correo`.** `auth` y `inscripciones` lo usan, y cada dominio mantiene sus propias plantillas. **Elegida.**

## Decisión

1. **`usuarios.idioma`** es `VARCHAR(2) NOT NULL DEFAULT 'es'` con `CHECK (idioma IN ('es','en'))` (migración 069).
   - Se fija en verify-email con `locale.Normalize(req.Locale)`.
   - Se cambia con `PUT /auth/idioma`, solo el propio usuario.
   - Se expone en `GET /auth/me` y en el `user` del login.
   - Cualquier otra fuente cae a `es` con `locale.Normalize` (`internal/locale`).
2. **Precedencia de idioma en los correos** (adopta la propuesta de `auth-idioma-correos.md` §5):
   - correo pedido desde la UI: `locale` del request;
   - correo disparado por el admin o el sistema: `usuarios.idioma`;
   - en cualquier caso, si falta: `es`.
3. **Notificaciones personales** (migración 070): `usuario_id`, `clave` y `parametros`, más el CHECK `(destinatario = 'usuario') = (usuario_id IS NOT NULL)`.
   - Las cuatro consultas de usuario (`ListForUser`, `UnreadCount`, `MarkRead` y `MarkAllRead`) incluyen `OR n.usuario_id = $usuario` en su condición de visibilidad.
   - Las del admin (`ListAll`, `Delete`) **excluyen** las personales.
4. **Aviso de decisión (PLT-12):** en la transacción de `UpdateEstado`, cuando `n == 1`, se hacen tres cosas:
   - la notificación personal (`inscripcion_aprobada` o `inscripcion_rechazada`), que se crea siempre;
   - el correo encolado en el idioma guardado, solo si la persona tiene un email legible;
   - el audit, como hoy.

   Un error de la BD en cualquiera de ellas revierte la decisión (500, el admin reintenta). Un fallo del **envío** SMTP no revierte nada: el worker reintenta y la salud del correo lo muestra.
5. **Motivo:** obligatorio al rechazar, de 1 a 500 caracteres (P6). Se envía tal cual, escapado en HTML en el correo. Nunca va a los logs.
6. **Plantillas:** `internal/correo` contiene el layout. Las plantillas de inscripción van en `inscripciones` y las de cuenta siguen en `auth`. PLT-13 pasa el idioma guardado a `emailActivado`, `emailRechazado` y `emailInactivo`.

## Consecuencias

- **Positivas:**
  - Los avisos llegan en el idioma del usuario, también los que dispara un admin.
  - Sin duplicados ni pérdidas, porque todo va en la misma transacción.
  - La campana existente muestra los avisos personales sin otro endpoint.
- **Negativas:**
  - La decisión del admin hace dos INSERT más dentro de su transacción. El costo es despreciable.
  - El frontend mantiene una tabla de `clave` → texto y ruta. Una clave desconocida se muestra con `titulo` y `cuerpo`.
  - `email_queue.body_html` guarda el nombre y el motivo, como ya guarda los correos de cuenta. No hay ningún dato personal nuevo en logs.
- **La 069 no es compatible hacia atrás:** el binario anterior hace `SELECT * FROM usuarios` (`auth/repository.go:438,447`), y sqlx falla con la columna nueva (#285). Por eso:
  - el rollback del código exige revertir antes la base;
  - PLT-11 pasa esas consultas a una lista explícita de columnas;
  - el orden de despliegue está en el contrato §4.
- **Down de la 069:** pierde las preferencias de idioma.
- **Down de la 070:** borra las notificaciones personales antes de quitar las columnas, porque sin `usuario_id` quedarían huérfanas e invisibles para todos.
- **Fuera:**
  - preferencias para desactivar avisos;
  - avisos de otras transiciones;
  - aplicar el idioma guardado a la cookie al iniciar sesión (posible historia aparte);
  - el correo de «correo cambiado» (`emailCorreoCambiado`), que sigue en `es` (P7).
