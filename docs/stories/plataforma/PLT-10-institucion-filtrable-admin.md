---
id: PLT-10
titulo: Institución filtrable en el admin; formulario de persona sin institución fija
epica: PLT-E2
fase: Diferida
estado: Diferida (alcance de la ronda: solo PLT-06, 11, 12, 13)
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: []
---

# PLT-10 Institución filtrable en el admin; formulario de persona sin institución fija

**Como** admin, **quiero** ver y filtrar por institución en inscripciones, usuarios y personas, **para** no mezclar datos de instituciones distintas.

## Contexto
/admin/inscripciones, /admin/usuarios y /admin/personas no tienen columna ni filtro de institución, y el formulario de persona arranca con `institucion_id: 1` fijo (`admin/personas/page.tsx:40`). Puede ir en paralelo a PLT-07..09. Decisiones: [README, PLT-E2](README.md#épica-plt-e2-multi-institución).

## Reglas de negocio
1. Los tres listados tienen una **columna Institución** y un **filtro por institución** (valor por defecto «Todas»). El filtro se aplica en servidor (parámetro `institucion_id`) y se combina con la búsqueda y los filtros que ya existen, sin romper la paginación.
2. En usuarios, la institución sale de la persona vinculada (siempre existe). **En /admin/inscripciones, la institución mostrada y filtrada es la del proceso**, no la de la persona (P3).
3. Formulario de persona: el campo institución es **obligatorio y sin valor inicial** (se elimina `institucion_id: 1`). Al editar, muestra la actual.
4. El cambio de institución de una persona existente lo hace solo un admin y se audita; no cambia inscripciones previas (E2 decisión 7).
5. `PUT /admin/personas/{id}` valida que el grado y la especialidad sean de la **misma institución** que la persona; si no, 422 `referencia_invalida` (P9). Una persona ya incoherente se corrige al editarla.

## Criterios de aceptación
```gherkin
Escenario: Columna y filtro en inscripciones
  Dado inscripciones de las instituciones A y B
  Cuando el admin filtra por B en /admin/inscripciones
  Entonces ve solo las de B, con la columna Institución, y la paginación cuenta solo ese conjunto

Escenario: Filtro en usuarios y personas
  Dado usuarios y personas de A y B
  Cuando el admin filtra por A en cada pantalla
  Entonces ve solo los de A y puede combinarlo con la búsqueda por texto

Escenario: Persona nueva sin institución por defecto
  Dado el admin abre «Nueva persona»
  Cuando mira el campo institución
  Entonces está vacío y guardar sin elegir muestra el error de campo requerido

Escenario: Editar persona
  Dado una persona de la institución B
  Cuando el admin abre su edición
  Entonces el campo institución muestra B, no la institución 1

Escenario: Institución del proceso en inscripciones
  Dado una inscripción cruzada de una persona de A a un proceso de B
  Cuando el admin filtra por B en /admin/inscripciones
  Entonces la ve, con la columna Institución mostrando B

Escenario: Grado de otra institución al editar persona
  Dado una persona de la institución B
  Cuando el admin la guarda con un grado o una especialidad de A
  Entonces recibe 422 referencia_invalida con el campo y no se guarda nada

Escenario: Filtro por API
  Dado el parámetro institucion_id de una institución inexistente
  Cuando el admin consulta cualquiera de los tres listados
  Entonces recibe lista vacía (no error)

Escenario: Permisos
  Dado un usuario con rol usuario
  Cuando llama a los listados de admin con institucion_id
  Entonces recibe 403
```

## Fuera de alcance
Filtro por institución en otras pantallas de admin (procesos ya tiene la suya); exportaciones; admins por institución; cambio de institución por el propio postulante.

## Seguridad y privacidad
Filtro validado en servidor; sin datos personales nuevos expuestos. El nombre de institución no es dato personal.

## i18n
Etiquetas «Institución», «Todas» y errores en es/en.

## Definición de terminado
Tests de API (filtro, combinación con búsqueda y paginación, 403); `tsc`/`lint`/`build`; contrato actualizado en `docs/api/`. Review limpio.
