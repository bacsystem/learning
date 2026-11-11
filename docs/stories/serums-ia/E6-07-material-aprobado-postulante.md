---
id: SIA-E6-07
titulo: Material de estudio aprobado para el postulante
epica: E6
fase: v2
prioridad: Media
estimacion: 3
roles: [usuario]
depende_de: [SIA-E6-06]
---

# SIA-E6-07 Material de estudio aprobado para el postulante

**Como** postulante, **quiero** leer material de estudio aprobado de mis áreas con sus fuentes visibles, **para** estudiar con contenido confiable.

## Reglas de negocio
1. Solo se listan y abren materiales `aprobado` de mi profesión; cualquier otro estado responde 404.
2. Cada párrafo muestra sus citas (título de fuente, versión, extracto); si la fuente fue retirada, el material desaparece de la lista (pasa a revisión).
3. Etiqueta visible «Revisado por el equipo» con la fecha de aprobación.
4. Filtros por área y tema; listado paginado (20 por defecto, máx. 100).
5. Botón «Reportar un error» (SIA-E6-09).
6. Exige inscripción aprobada.

## Criterios de aceptación
```gherkin
Escenario: Listar
  Dado 3 materiales aprobados y 2 en borrador de mi profesión
  Cuando abro el listado de material
  Entonces veo solo los 3 aprobados

Escenario: Leer con citas
  Dado un material aprobado con citas por párrafo
  Cuando abro el material
  Entonces cada párrafo muestra sus citas con título de fuente y extracto

Escenario: Material en revisión
  Dado un material que pasó a "en_revision"
  Cuando abro el listado o pido su id
  Entonces desaparece de la lista y pedir su id responde 404

Escenario: Otra profesión
  Dado un material aprobado de otra profesión
  Cuando pido ese material
  Entonces recibe 404

Escenario: Sin inscripción
  Dado un usuario sin inscripción aprobada
  Cuando abre el listado de material
  Entonces recibe 403

Escenario: Sin materiales
  Dado un área sin material aprobado
  Cuando abro el área
  Entonces veo «Aún no hay material para esta área» y el botón «Solicitar al equipo» (SIA-E5-06)

Escenario: Estados de UI
  Dado el listado de material
  Cuando la petición está en curso o falla
  Entonces hay carga y error con reintento
```

## Fuera de alcance
Descarga sin conexión, marcadores, notas personales.

## Dependencias
SIA-E6-06.

## Seguridad y privacidad
Estado verificado en servidor; autorización por profesión e inscripción.

## i18n
Interfaz es/en; el contenido del material está en español (se indica con la etiqueta de idioma en la versión en).

## Definición de terminado
`tsc`, `lint`, `build`; tests de estado y profesión; review limpio.
