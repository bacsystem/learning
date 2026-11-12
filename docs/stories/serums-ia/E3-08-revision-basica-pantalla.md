---
id: SIA-E3-08
titulo: Revisión básica de preguntas — listado y detalle con citas
epica: E3
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [SIA-E3-07]
---

# SIA-E3-08 Revisión básica de preguntas — listado y detalle con citas

**Como** admin, **quiero** ver en la administración de preguntas el estado, el origen y las citas de cada pregunta, filtrarlas y añadir o quitar citas buscando pasajes, **para** preparar una pregunta antes de aprobarla o revalidarla, sin usar la API.

## Contexto
Usa la API de SIA-E3-07 (búsqueda de pasajes y `PUT …/citas`). Es la primera de tres historias de pantalla, por la decisión del PO en el review del PR #153 (H4: ninguna historia supera 3 pts):
- **SIA-E3-08 (esta):** listado y detalle con citas.
- **SIA-E3-09:** acciones de revisión (aprobar, revalidar, rechazar y reabrir).
- **SIA-E3-10:** avisos en las pantallas existentes (alta, Excel, edición y eliminación).

La tarea de frontend F1 de SIA-E3-02 (rama `feature/sia-e3-02-estados-preguntas`) ya adelanta las insignias de estado, el origen y el nº de citas en el listado. Eso se reutiliza, no se rehace, pero se acepta contra los criterios de esta historia.

Las pantallas usan componentes y tokens de khipu.

## Reglas de negocio
1. **Listado:**
   - Insignia de estado (icono + texto, nunca solo color) para `borrador`, `en_revision`, `aprobada`, `rechazada` y `activo`, más el origen y el nº de citas.
   - Filtros por estado y por origen. Por defecto se ve todo, como hoy.
   - Los nombres de estado reutilizan las claves de SIA-E3-02 (`dashboard.serumsIa.estadosPregunta.*`) y no se duplican.
2. **Detalle, citas actuales:** cada cita muestra fuente, versión y extracto, con la opción de quitarla.
3. **Detalle, buscador de pasajes:**
   - Búsqueda por texto con hasta 20 resultados (SIA-E3-07). Cada uno muestra título de la fuente, versión y extracto, y se añade como cita con una acción.
   - «Sin resultados» si no hay coincidencias.
   - Nunca muestra rutas de archivo.
4. **Errores de citas:** `fuente_no_activa`, `limite_citas` y `ultima_cita_activa` se muestran con un texto traducido, nunca con el código crudo. Tras el error la lista de citas se recarga. Un error desconocido muestra un texto genérico traducido.
5. **Estados de UI:**
   - Listado: carga, vacío («No hay preguntas con ese filtro») y error.
   - Buscador: carga, «Sin resultados» y error.
6. El detalle deja visible la zona de acciones de revisión, que implementa SIA-E3-09. Esta historia no añade botones de transición.

## Criterios de aceptación
```gherkin
Escenario: Insignias y origen en el listado
  Dado preguntas en "borrador", "en_revision", "aprobada", "rechazada" y "activo"
  Cuando el admin abre la administración de preguntas
  Entonces cada fila muestra su insignia con icono y texto, su origen y su nº de citas

Escenario: Filtro por estado
  Dado 5 preguntas en "borrador" y 20 "aprobada"
  Cuando el admin filtra por «Borrador»
  Entonces ve solo las 5

Escenario: Filtro por origen
  Dado preguntas de origen "manual" y "balotario"
  Cuando el admin filtra por origen «Balotario»
  Entonces ve solo las de balotario

Escenario: Filtro sin resultados
  Dado ninguna pregunta "rechazada"
  Cuando el admin filtra por «Rechazada»
  Entonces ve «No hay preguntas con ese filtro»

Escenario: Añadir una cita
  Dado una pregunta "borrador" manual sin citas
  Cuando el admin busca "hipertensión" y añade un pasaje
  Entonces la cita aparece en el detalle con fuente, versión y extracto, y el nº de citas del listado pasa a 1

Escenario: Búsqueda sin resultados
  Dado ningún pasaje que contenga "zzzz"
  Cuando el admin busca "zzzz"
  Entonces ve «Sin resultados»

Escenario: Quitar la última cita de una aprobada
  Dado una pregunta "aprobada" con una única cita activa
  Cuando el admin la quita
  Entonces ve el texto traducido de "ultima_cita_activa", no el código, y la cita sigue en la lista

Escenario: Límite de citas
  Dado una pregunta con 20 citas
  Cuando el admin añade otra
  Entonces ve el texto traducido de "limite_citas"

Escenario: Idioma
  Dado el idioma en inglés
  Cuando el admin usa el listado, los filtros, las citas y el buscador
  Entonces estados, orígenes, filtros, citas, búsqueda, estados de UI y errores se ven en inglés

Escenario: Acceso
  Dado un usuario con rol usuario
  Cuando abre la administración de preguntas o el detalle de una pregunta
  Entonces la ruta lo redirige
```

## Fuera de alcance
- Acciones de revisión, «Se revisa en su cola», errores de transición y recarga ante 409 de transición: SIA-E3-09.
- Avisos en alta, Excel, edición y eliminación: SIA-E3-10.
- Aprobación masiva, visor completo del pasaje con resaltado y colas especializadas (SIA-E3-05, SIA-E6-05).

## Dependencias
SIA-E3-07. Las insignias de estado son de khipu; si falta un componente, va primero un PR a khipu.

## Seguridad y privacidad
Guard de rol admin. La pantalla no muestra rutas de archivo ni códigos de error crudos.

## i18n
Todo en es/en. Esta historia es dueña de:
- `dashboard.serumsIa.revision.listado.*`: filtros, origen, nº de citas y estados de UI;
- `dashboard.serumsIa.revision.citas.*`: citas, buscador y «Sin resultados»;
- `dashboard.serumsIa.revision.errores.*`: el texto genérico y los de `fuente_no_activa`, `limite_citas` y `ultima_cita_activa`. SIA-E3-09 añade aquí los de transición.

Los nombres de estado van en `dashboard.serumsIa.estadosPregunta.*` (de SIA-E3-02). Cada clave existe en ambos archivos.

## Definición de terminado
- `tsc`, `lint` y `build` en verde.
- Estados de UI del listado y del buscador verificados.
- Claves en es y en.
- Accesibilidad: el estado no depende solo del color.
- Review limpio.
