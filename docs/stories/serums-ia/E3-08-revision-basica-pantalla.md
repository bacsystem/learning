---
id: SIA-E3-08
titulo: Revisión básica de preguntas — pantalla y avisos en la administración de preguntas
epica: E3
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [SIA-E3-07]
---

# SIA-E3-08 Revisión básica de preguntas — pantalla y avisos

**Como** admin, **quiero** ver el estado de cada pregunta, citarla, aprobarla, rechazarla, reabrirla y revalidarla desde la administración de preguntas, y que las pantallas existentes me avisen del nuevo ciclo de vida, **para** publicar preguntas sin usar la API y sin sorpresas.

## Contexto
Usa la API de SIA-E3-07. La administración de preguntas existente cambia de comportamiento con SIA-E3-02:
- el alta y el Excel guardan en `borrador`;
- editar el contenido de una pregunta publicada la pasa a `en_revision`;
- eliminar un `borrador` o una `en_revision` da 409.

Esta historia es dueña de esos avisos. Las pantallas usan componentes y tokens de khipu.

## Reglas de negocio
1. **Listado:** insignia de estado (icono + texto, nunca solo color) para `borrador`, `en_revision`, `aprobada`, `rechazada` y `activo`, más origen y nº de citas. Filtros por estado y origen. Por defecto se ve todo, como hoy. Los nombres de estado reutilizan las claves de la UI de SIA-E3-02 (`dashboard.serumsIa.estadosPregunta.*`) y no se duplican.
2. **Detalle / revisión:**
   - Citas actuales, con fuente, versión y extracto, y opción de quitar.
   - Buscador de pasajes, con «Sin resultados» si no hay coincidencias.
   - Solo las acciones válidas para el estado y el origen:
     - **Aprobar:** solo en `borrador` de origen `manual`, con la casilla «He verificado la pregunta y su clave contra la cita».
     - **Revalidar:** en `en_revision`, con la misma casilla, más una nota obligatoria si el origen es `balotario` o `ia`.
     - **Rechazar:** con un motivo de 5–500 caracteres.
     - **Reabrir.**
   - En un `borrador` de `balotario` o `ia`, en lugar de «Aprobar» se ve «Se revisa en su cola», con enlace si la cola existe.
3. **Avisos en las pantallas existentes:**
   - **Alta y Excel:** después de guardar, «Guardada como borrador. Añade una cita y apruébala para que la vean los postulantes.» En el Excel: «N preguntas importadas como borrador».
   - **Edición de una `aprobada` o `activo`:** si el cambio toca enunciado, alternativas, clave o explicación, antes de guardar se muestra la confirmación «Al guardar, la pregunta pasará a En revisión y dejará de mostrarse hasta revalidarla con una cita.» Si el cambio es solo de clasificación, no hay aviso.
   - **Eliminar:** en `borrador` y `en_revision` la acción está deshabilitada, con la ayuda «Recházala primero para poder eliminarla».
4. Los errores de la API (`falta_cita`, `falta_area`, `verificacion_requerida`, `nota_obligatoria`, `conflicto_clave`, `revision_especializada`, `fuente_no_activa`, `limite_citas`, `estado_invalido`) se muestran con un texto traducido, nunca con el código crudo. Ante un 409 la pantalla recarga el estado actual.
5. Estados de UI: carga, vacío («No hay preguntas con ese filtro») y error.

## Criterios de aceptación
```gherkin
Escenario: Aprobar desde la pantalla
  Dado una pregunta "borrador" manual con área
  Cuando el admin busca "hipertensión", añade un pasaje, marca la casilla y pulsa Aprobar
  Entonces la insignia pasa a «Aprobada» y la cita aparece en el detalle

Escenario: Botón deshabilitado sin casilla
  Dado una pregunta "borrador" manual con una cita
  Cuando el admin no marca la casilla
  Entonces el botón Aprobar está deshabilitado

Escenario: Borrador de balotario
  Dado una pregunta "borrador" de origen "balotario"
  Cuando el admin abre su detalle
  Entonces no ve «Aprobar» sino «Se revisa en su cola»

Escenario: Alta manual
  Dado el formulario de alta de pregunta
  Cuando el admin guarda una pregunta nueva
  Entonces ve «Guardada como borrador. Añade una cita y apruébala para que la vean los postulantes.»

Escenario: Aviso al editar contenido publicado
  Dado una pregunta "aprobada"
  Cuando el admin cambia una alternativa y pulsa Guardar
  Entonces ve la confirmación de que pasará a En revisión; si confirma, la insignia pasa a «En revisión»; si cancela, nada cambia

Escenario: Editar solo la clasificación
  Dado una pregunta "aprobada"
  Cuando el admin cambia solo su tema y guarda
  Entonces no ve aviso y sigue «Aprobada»

Escenario: Eliminar un borrador
  Dado una pregunta "borrador"
  Cuando el admin abre sus acciones
  Entonces «Eliminar» está deshabilitado con la ayuda «Recházala primero para poder eliminarla»

Escenario: Revalidar con nota
  Dado una pregunta de origen "ia" en "en_revision" con una cita activa
  Cuando el admin marca la casilla sin escribir nota
  Entonces el botón Revalidar está deshabilitado y el campo nota muestra que es obligatorio

Escenario: Error de la API traducido
  Dado una pregunta que otro admin acaba de rechazar
  Cuando el admin pulsa Aprobar
  Entonces ve «Esta pregunta cambió de estado; se actualizó la vista» y la insignia muestra «Rechazada»

Escenario: Filtro por estado
  Dado 5 preguntas en "borrador" y 20 "aprobada"
  Cuando el admin filtra por «Borrador»
  Entonces ve solo las 5

Escenario: Idioma
  Dado el idioma en inglés
  Cuando el admin usa la administración de preguntas
  Entonces estados, acciones, casilla, avisos, errores y búsqueda se ven en inglés

Escenario: Acceso
  Dado un usuario con rol usuario
  Cuando abre la administración de preguntas
  Entonces la ruta lo redirige
```

## Fuera de alcance
Aprobación masiva, visor completo del pasaje con resaltado, y colas especializadas (SIA-E3-05, SIA-E6-05).

## Dependencias
SIA-E3-07. Las insignias de estado son de khipu; si falta un componente, va primero un PR a khipu.

## Seguridad y privacidad
Guard de rol admin. La pantalla no muestra rutas de archivo ni códigos de error crudos.

## i18n
Todo en es/en, bajo `dashboard.serumsIa.revision.*`. Los nombres de estado van en `dashboard.serumsIa.estadosPregunta.*` (de la UI de SIA-E3-02). Cada clave existe en ambos archivos.

## Definición de terminado
`tsc`, `lint` y `build` en verde. Estados de UI verificados. Avisos de alta, Excel, edición y eliminación verificados. Claves es/en. Accesibilidad: el estado no depende solo del color. Review limpio.
