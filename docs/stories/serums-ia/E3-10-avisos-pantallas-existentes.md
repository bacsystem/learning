---
id: SIA-E3-10
titulo: Avisos del ciclo de vida en las pantallas existentes de preguntas
epica: E3
fase: MVP
prioridad: Alta
estimacion: 2
roles: [admin]
depende_de: [SIA-E3-02]
---

# SIA-E3-10 Avisos del ciclo de vida en las pantallas existentes de preguntas

**Como** admin, **quiero** que el alta, la importación Excel, la edición y la eliminación de preguntas me avisen del nuevo ciclo de vida, **para** no sorprenderme cuando una pregunta queda en borrador, deja de mostrarse o no se puede eliminar.

## Contexto
SIA-E3-02 cambia el comportamiento de la administración de preguntas existente:
- el alta y el Excel guardan en `borrador`;
- editar el contenido de una pregunta `aprobada` o `activo` la pasa a `en_revision`;
- eliminar un `borrador` o una `en_revision` da 409 `estado_invalido`.

Esta historia es dueña de esos avisos. Solo consume el contrato de SIA-E3-02, no la API de SIA-E3-07, así que puede ir en paralelo con SIA-E3-08 y SIA-E3-09. Viene de dividir la antigua SIA-E3-08 (review del PR #153, H4).

La tarea de frontend F1 de SIA-E3-02 (rama `feature/sia-e3-02-estados-preguntas`) ya adelanta parte de esto: eliminar según el estado y el aviso de edición. No se duplica: lo que F1 ya entregue se reutiliza, y esta historia completa el resto. En todo caso se acepta contra los criterios de esta historia, no contra los de SIA-E3-02. Las pantallas usan componentes y tokens de khipu.

## Reglas de negocio
1. **Alta:** después de guardar, se muestra «Guardada como borrador. Añade una cita y apruébala para que la vean los postulantes.»
2. **Excel:** después de importar, se muestra «N preguntas importadas como borrador», con N el número importado.
3. **Edición de una `aprobada` o `activo`:**
   - Si el cambio toca enunciado, tipo, alternativas, clave o explicación, antes de guardar se pide la confirmación «Al guardar, la pregunta pasará a En revisión y dejará de mostrarse hasta revalidarla con una cita.»
   - Si se confirma, se guarda. Si se cancela, no se envía nada.
   - Si el cambio es solo de clasificación (área, tema, etiquetas o dificultad), no hay aviso.
   - La comparación del cliente es solo para avisar; el servidor decide el estado (SIA-E3-02, regla 10).
4. **Edición, clave oficial:** si el servidor responde 422 `clave_oficial` (origen `balotario`), se muestra un texto traducido, nunca el código crudo.
5. **Eliminar:**
   - En `borrador` y `en_revision`, la acción está deshabilitada, con la ayuda «Recházala primero para poder eliminarla».
   - Si aun así el servidor responde 409 `estado_invalido` (p. ej. el estado cambió mientras tanto), se muestra el mismo texto traducido y se recarga la pregunta.
6. En `borrador`, `en_revision` y `rechazada`, editar no muestra el aviso de la regla 3, porque el estado no cambia.

## Criterios de aceptación
```gherkin
Escenario: Alta manual
  Dado el formulario de alta de pregunta
  Cuando el admin guarda una pregunta nueva
  Entonces ve «Guardada como borrador. Añade una cita y apruébala para que la vean los postulantes.»

Escenario: Importación Excel
  Dado un Excel válido con 12 preguntas
  Cuando el admin lo importa
  Entonces ve «12 preguntas importadas como borrador»

Escenario: Aviso al editar contenido publicado
  Dado una pregunta "aprobada"
  Cuando el admin cambia una alternativa y pulsa Guardar
  Entonces ve la confirmación de que pasará a En revisión; si confirma, se guarda y la pregunta queda en "en_revision"; si cancela, no se envía nada y sigue "aprobada"

Escenario: Editar solo la clasificación
  Dado una pregunta "aprobada"
  Cuando el admin cambia solo su tema y guarda
  Entonces no ve aviso y sigue "aprobada"

Escenario: Editar un borrador
  Dado una pregunta "borrador"
  Cuando el admin cambia su enunciado y guarda
  Entonces no ve el aviso de En revisión

Escenario: Clave oficial del balotario
  Dado una pregunta de origen "balotario"
  Cuando el admin cambia la clave y guarda
  Entonces ve el texto traducido de "clave_oficial", no el código, y la pregunta no cambia

Escenario: Eliminar un borrador
  Dado una pregunta "borrador"
  Cuando el admin abre sus acciones
  Entonces «Eliminar» está deshabilitado con la ayuda «Recházala primero para poder eliminarla»

Escenario: Eliminar una rechazada
  Dado una pregunta "rechazada"
  Cuando el admin abre sus acciones
  Entonces «Eliminar» está habilitado

Escenario: Idioma
  Dado el idioma en inglés
  Cuando el admin da de alta, importa, edita o intenta eliminar una pregunta
  Entonces los avisos, la confirmación, la ayuda y los errores se ven en inglés

Escenario: Acceso
  Dado un usuario con rol usuario
  Cuando abre el alta, la importación o la edición de preguntas
  Entonces la ruta lo redirige
```

El estado tras guardar se verifica con la respuesta de la API o con la insignia de SIA-E3-08, si ya está entregada.

## Fuera de alcance
- Listado con insignias, filtros, citas y buscador: SIA-E3-08.
- Acciones de revisión: SIA-E3-09.
- Cambiar los formularios de alta, Excel o edición más allá de los avisos.

## Dependencias
SIA-E3-02 (estados y guardas de editar y eliminar). Si khipu no tiene el diálogo de confirmación necesario, va primero un PR a khipu.

## Seguridad y privacidad
Las guardas viven en el servidor; el cliente solo avisa. No se muestran códigos de error crudos.

## i18n
Todo en es/en. Esta historia es dueña de `dashboard.serumsIa.avisosPregunta.*`: alta, Excel (con plural), confirmación de edición y la ayuda de eliminar, que también se muestra ante el 409 al eliminar. El error `clave_oficial` va en `dashboard.serumsIa.errores.clave_oficial`, con la convención de `docs/api/README.md` §8 (una clave por código, compartida). Si la clave ya existe, se reutiliza y no se duplica. Cada clave existe en ambos archivos.

## Definición de terminado
- `tsc`, `lint` y `build` en verde.
- Avisos de alta, Excel, edición (confirmar y cancelar) y eliminación verificados.
- Claves en es y en.
- Review limpio.
