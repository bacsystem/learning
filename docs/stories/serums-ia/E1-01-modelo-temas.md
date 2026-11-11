---
id: SIA-E1-01
titulo: Modelo de temas bajo asignatura
epica: E1
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin, usuario]
depende_de: []
---

# SIA-E1-01 Modelo de temas bajo asignatura

**Como** admin, **quiero** que cada área (asignatura) pueda tener temas, **para** clasificar preguntas y medir el dominio con más detalle que el área completa.

## Contexto
Hoy existen asignaturas y especialidades pero no un nivel inferior. Se crea el concepto de **tema**, hijo de una asignatura. Un tema puede aplicar a todas las especialidades o solo a algunas (una asignatura como «Salud pública» se comparte entre profesiones).

## Reglas de negocio
1. Un tema pertenece a exactamente una asignatura; nombre único dentro de la asignatura (sin distinguir mayúsculas ni tildes).
2. Nombre: 3–120 caracteres; descripción opcional hasta 500.
3. Un tema se asocia a 0..N especialidades; sin especialidades significa «aplica a todas».
4. Estados del tema: `activo` | `archivado`. No se borra un tema con preguntas; solo se archiva.
5. Una pregunta puede tener 0 o 1 tema principal (campo nuevo `tema_id`, nulo permitido para el banco existente).
6. Máximo 60 temas `activo` por asignatura (decisión PO, Q10 del contrato): los archivados no cuentan, así que archivar libera cupo. La unicidad de nombre de la regla 1 sí incluye los archivados: para volver a usar un nombre archivado se reactiva el tema, no se crea otro.
6a. Reactivar: un admin puede pasar un tema `archivado` a `activo`. Se revalida el límite de la regla 6 (422 `limite_temas` si ya hay 60 activos). El tema reactivado va al final del orden (regla 7) y conserva sus preguntas y especialidades. Reactivar un tema que ya está activo responde 409 `estado_invalido`. Se audita como `tema.reactivar`.
7. Cada tema tiene una posición (`orden`) dentro de su asignatura. Un tema nuevo se coloca al final. Los listados de temas se ordenan por esa posición. El reordenamiento lo define SIA-E1-02.

## Criterios de aceptación
```gherkin
Escenario: Crear tema
  Dado un admin autenticado y la asignatura "Salud pública"
  Cuando crea el tema "Vigilancia epidemiológica"
  Entonces el sistema responde 201 con el tema en estado activo
  Y se registra una entrada en audit_log con actor, acción "tema.crear" y el id del tema

Escenario: Nombre duplicado
  Dado el tema "Vigilancia epidemiológica" en la asignatura
  Cuando el admin crea "vigilancia epidemiologica" en la misma asignatura
  Entonces el sistema responde 422 con el código "tema_duplicado"

Escenario: Límite de temas
  Dado una asignatura con 60 temas activos
  Cuando el admin crea otro
  Entonces el sistema responde 422 con el código "limite_temas"

Escenario: Archivar tema con preguntas
  Dado un tema con 10 preguntas
  Cuando el admin lo archiva
  Entonces queda en estado archivado, las preguntas conservan su tema y no se ofrece para nuevas clasificaciones

Escenario: Archivados no cuentan para el límite
  Dado una asignatura con 59 temas activos y 5 archivados
  Cuando el admin crea otro tema
  Entonces responde 201

Escenario: Nombre de un tema archivado
  Dado el tema archivado "Vigilancia epidemiológica" en la asignatura
  Cuando el admin crea "Vigilancia epidemiológica" en la misma asignatura
  Entonces responde 422 "tema_duplicado"

Escenario: Reactivar tema
  Dado un tema archivado con 10 preguntas en una asignatura con 20 temas activos
  Cuando el admin lo reactiva
  Entonces queda activo, al final del orden, conserva sus 10 preguntas y audit_log registra "tema.reactivar"

Escenario: Reactivar por encima del límite
  Dado una asignatura con 60 temas activos y uno archivado
  Cuando el admin reactiva el archivado
  Entonces responde 422 "limite_temas" y el tema sigue archivado

Escenario: Reactivar un tema activo
  Dado un tema activo
  Cuando el admin intenta reactivarlo
  Entonces responde 409 "estado_invalido"

Escenario: Permisos
  Dado un usuario con rol usuario
  Cuando intenta crear un tema
  Entonces recibe 403

Escenario: Migración reversible
  Dado el esquema actual
  Cuando se aplica la migración y luego su Down
  Entonces las tablas y columnas nuevas desaparecen sin afectar las preguntas existentes
```

## Fuera de alcance
Interfaz de administración (SIA-E1-02), pesos (SIA-E1-03), clasificación de preguntas con IA (SIA-E3-04).

## Dependencias
Catálogos existentes (asignaturas, especialidades). Contrato en docs/api/ (architect).

## Seguridad y privacidad
Solo admin escribe; lectura para usuario autenticado. Sin datos personales.

## i18n
Los nombres de temas son contenido (solo es). Mensajes de error visibles con claves es/en.

## Definición de terminado
Migración con Down probada; endpoints con tests de permisos y límites; audit_log verificado; contrato publicado; review limpio.
