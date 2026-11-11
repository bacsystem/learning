---
id: SIA-E3-09
titulo: Revisión básica de preguntas — acciones de revisión en pantalla
epica: E3
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [SIA-E3-07, SIA-E3-08]
---

# SIA-E3-09 Revisión básica de preguntas — acciones de revisión en pantalla

**Como** admin, **quiero** aprobar, revalidar, rechazar y reabrir una pregunta desde su detalle, viendo solo las acciones válidas para su estado y su origen, **para** publicar preguntas manuales y devolver a circulación las que quedaron en revisión, sin usar la API.

## Contexto
Usa `PATCH /preguntas/{id}/estado` con las guardas de SIA-E3-07. Se monta en la zona de acciones del detalle de SIA-E3-08, que le aporta las citas. Viene de dividir la antigua SIA-E3-08 (review del PR #153, H4). Las pantallas usan componentes y tokens de khipu.

## Reglas de negocio
1. **Solo las acciones válidas para el estado y el origen:**
   - **Aprobar:** solo en un `borrador` de origen `manual`, con la casilla «He verificado la pregunta y su clave contra la cita». Sin la casilla, el botón está deshabilitado.
   - **Revalidar:** en `en_revision` de cualquier origen, con la misma casilla. Si el origen es `balotario` o `ia`, la nota es obligatoria (10–500 caracteres); sin ella, el botón está deshabilitado y el campo indica que es obligatorio.
   - **Rechazar:** en `borrador` o `en_revision`, con un motivo de 5–500 caracteres.
   - **Reabrir:** en `rechazada`.
   - En `aprobada` y `activo` no hay acciones de revisión.
2. **Borrador de `balotario` o `ia`:** en lugar de «Aprobar» se ve «Se revisa en su cola», con enlace si la cola existe (SIA-E3-05, SIA-E6-05). Si la cola aún no existe, solo el texto.
3. **Errores de la API:** `falta_cita`, `verificacion_requerida`, `nota_obligatoria`, `conflicto_clave`, `revision_especializada` y `estado_invalido` se muestran con un texto traducido, nunca con el código crudo.
4. **Ante un 409** la pantalla recarga el estado actual de la pregunta y muestra «Esta pregunta cambió de estado; se actualizó la vista».
5. **Tras una acción correcta**, la insignia del detalle y la del listado reflejan el nuevo estado sin recargar a mano.
6. Las validaciones del cliente (casilla, nota y motivo) replican las del servidor, pero no lo sustituyen: si el servidor rechaza, se aplica la regla 3.

## Criterios de aceptación
```gherkin
Escenario: Aprobar desde la pantalla
  Dado una pregunta "borrador" manual con una cita a una fuente activa
  Cuando el admin marca la casilla y pulsa Aprobar
  Entonces la insignia pasa a «Aprobada»

Escenario: Botón deshabilitado sin casilla
  Dado una pregunta "borrador" manual con una cita
  Cuando el admin no marca la casilla
  Entonces el botón Aprobar está deshabilitado

Escenario: Aprobar sin cita
  Dado una pregunta "borrador" manual sin citas
  Cuando el admin marca la casilla y pulsa Aprobar
  Entonces ve el texto traducido de "falta_cita", no el código, y sigue «Borrador»

Escenario: Borrador de balotario
  Dado una pregunta "borrador" de origen "balotario"
  Cuando el admin abre su detalle
  Entonces no ve «Aprobar» sino «Se revisa en su cola»

Escenario: Revalidar con nota
  Dado una pregunta de origen "ia" en "en_revision" con una cita activa
  Cuando el admin marca la casilla sin escribir nota
  Entonces el botón Revalidar está deshabilitado y el campo nota indica que es obligatorio

Escenario: Revalidar una manual
  Dado una pregunta manual en "en_revision" con una cita activa
  Cuando el admin marca la casilla y pulsa Revalidar
  Entonces la insignia pasa a «Aprobada», sin pedir nota

Escenario: Rechazar con motivo corto
  Dado una pregunta "borrador"
  Cuando el admin escribe el motivo "malo" (4 caracteres)
  Entonces el botón Rechazar está deshabilitado y el campo indica el mínimo de 5

Escenario: Rechazar y reabrir
  Dado una pregunta "borrador"
  Cuando el admin la rechaza con motivo "Clave incorrecta" y luego la reabre
  Entonces la insignia pasa a «Rechazada» y después a «Borrador»

Escenario: Conflicto de clave
  Dado una pregunta "en_revision" con conflicto de clave sin resolver
  Cuando el admin intenta revalidarla
  Entonces ve el texto traducido de "conflicto_clave", no el código

Escenario: Cambio de estado concurrente
  Dado una pregunta que otro admin acaba de rechazar
  Cuando el admin pulsa Aprobar
  Entonces ve «Esta pregunta cambió de estado; se actualizó la vista» y la insignia muestra «Rechazada»

Escenario: Sin acciones en una aprobada
  Dado una pregunta "aprobada"
  Cuando el admin abre su detalle
  Entonces no ve Aprobar, Revalidar, Rechazar ni Reabrir

Escenario: Idioma
  Dado el idioma en inglés
  Cuando el admin usa las acciones de revisión
  Entonces acciones, casilla, nota, motivo, «Se revisa en su cola» y errores se ven en inglés

Escenario: Acceso
  Dado un usuario con rol usuario
  Cuando abre el detalle de una pregunta
  Entonces la ruta lo redirige
```

## Fuera de alcance
- Listado, filtros, citas y buscador: SIA-E3-08.
- Avisos en alta, Excel, edición y eliminación: SIA-E3-10.
- Aprobación masiva, segundo revisor y colas especializadas (SIA-E3-05, SIA-E6-05).

## Dependencias
SIA-E3-07 (API) y SIA-E3-08 (detalle con citas). Si khipu no tiene un componente necesario, va primero un PR a khipu.

## Seguridad y privacidad
Guard de rol admin. Las guardas viven en el servidor; el cliente solo las anticipa. La nota y el motivo no se escriben en la consola ni en los logs del navegador.

## i18n
Todo en es/en. Esta historia es dueña de `dashboard.serumsIa.revision.acciones.*`: acciones, casilla, nota, motivo, «Se revisa en su cola» y el texto de la recarga por 409. Los errores van en `dashboard.serumsIa.errores.<codigo>`, una clave por código compartida por todas las pantallas (convención de `docs/api/README.md` §8), para `falta_cita`, `verificacion_requerida`, `nota_obligatoria`, `conflicto_clave`, `revision_especializada` y `estado_invalido`. Si la clave ya existe (p. ej. `estado_invalido`, que también usa SIA-E3-10), se reutiliza y no se duplica. Para un código sin clave se usa el genérico de SIA-E3-08. Cada clave existe en ambos archivos.

## Definición de terminado
- `tsc`, `lint` y `build` en verde.
- Acciones por estado y origen verificadas, incluida la recarga ante 409.
- Claves en es y en.
- Review limpio.
