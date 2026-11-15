---
id: PLT-06
titulo: Elegibilidad por institución
epica: PLT-E2
fase: Crítica (antes de la ola 1)
prioridad: Crítica
estimacion: 3
roles: [postulante, admin]
depende_de: []
---

# PLT-06 Elegibilidad por institución

**Como** postulante, **quiero** ver e inscribirme solo en los procesos de mi institución, **para** no practicar por error en un proceso que no es mío.

## Contexto
`inscripciones/repository.go:46-57` lista todos los procesos activos sin filtrar por institución, y `service.go` (`ListProcesosActivos`, `Inscribir`) solo compara `nivel`. qa reprodujo que una persona de una institución se inscribió a un proceso de otra. Decisiones de la épica (institución del postulante, alcance, vocabulario): ver [README, PLT-E2](README.md#épica-plt-e2-multi-institución).

## Reglas de negocio
1. `GET /procesos`: solo procesos con `institucion_id` igual al de la persona de quien consulta, más el filtro de nivel actual (vacío = todos los niveles). Aplica a **todos los roles** (P4): las pantallas de admin usan `/admin/procesos`, que no cambia.
2. Inscribir a un proceso de otra institución: 403 `institucion_no_permitida` (contrato en [plataforma-multi-institucion.md](../../api/plataforma-multi-institucion.md) §2.3), distinto de `nivel_no_permitido`. Aplica aunque el cliente conozca el id (no basta con ocultar). Orden de errores: proceso inexistente → inactivo → otra institución → nivel.
3. Crear una **práctica** o un **simulacro** nuevo sobre un proceso de otra institución: mismo 403 (E2 decisión 9), aunque exista una inscripción cruzada ya aprobada (P2).
4. Los intentos de práctica o simulacro **ya creados** no se cortan (P2). Las inscripciones cruzadas aprobadas siguen apareciendo en `/inscripciones/me`.
5. Si no hay procesos para el postulante, /procesos muestra un estado vacío claro.
6. En `/admin/procesos` el admin sigue viendo todos los procesos. `GET /procesos/publico` no cambia aquí (ver PLT-14).
7. Inscripciones cruzadas **pendientes** existentes (P10): el admin todavía puede aprobarlas; no se bloquea. El PR trae el conteo y los ids (consulta en contrato §2.5, sin datos personales) y el usuario decide caso por caso antes de aprobar.

## Criterios de aceptación
```gherkin
Escenario: Solo veo los procesos de mi institución
  Dado un postulante de la institución A y procesos activos de A y de B
  Cuando abre /procesos
  Entonces ve solo los de A que cumplen su nivel y ninguno de B

Escenario: Inscripción a proceso de otra institución
  Dado un postulante de A y un proceso activo de B
  Cuando llama directamente a la inscripción con el id de ese proceso
  Entonces recibe 403 con el código de institución no permitida y no se crea ninguna inscripción

Escenario: Misma institución, nivel distinto
  Dado un postulante de A de nivel X y un proceso de A para nivel Y
  Cuando intenta inscribirse
  Entonces recibe el error de nivel actual, no el de institución

Escenario: Proceso sin restricción de nivel
  Dado un proceso de A con nivel vacío
  Cuando un postulante de A de cualquier nivel abre /procesos
  Entonces lo ve y puede inscribirse

Escenario: Práctica sobre proceso de otra institución
  Dado un postulante de A y un proceso de B
  Cuando intenta crear una práctica sobre ese proceso
  Entonces recibe 403 con el código de institución no permitida

Escenario: Simulacro sobre proceso de otra institución
  Dado un postulante de A y un proceso de B
  Cuando intenta crear un simulacro sobre ese proceso
  Entonces recibe 403 con el código de institución no permitida

Escenario: Inscripción cruzada ya aprobada
  Dado un postulante de A con una inscripción aprobada a un proceso de B, creada antes del arreglo
  Cuando intenta crear una práctica o un simulacro nuevo sobre ese proceso
  Entonces recibe 403 con el código de institución no permitida; los intentos que ya tenía siguen accesibles y la inscripción sigue en /inscripciones/me

Escenario: Todos los roles ven solo su institución
  Dado un usuario admin cuya persona es de la institución A
  Cuando llama a GET /procesos
  Entonces ve solo los procesos activos de A; en /admin/procesos sigue viendo todos

Escenario: Sin procesos de mi institución
  Dado un postulante de A y ningún proceso activo de A
  Cuando abre /procesos
  Entonces ve el estado vacío traducido y ningún proceso de otra institución

Escenario: Landing sin cambios
  Dado un visitante sin sesión
  Cuando llama a GET /procesos/publico
  Entonces recibe lo mismo que antes de esta historia
```

## Fuera de alcance
Catálogos por institución (PLT-07..09); nombre de la institución en pantalla y agrupación de la landing (PLT-14); mover o borrar inscripciones cruzadas existentes (E2 decisión 7); preguntas (todos sus endpoints son admin, sin hueco); lecturas por `proceso_id` de configuración de simulacros y asignaturas (`GET /simulacros/configuracion`, `GET /procesos/{id}/asignaturas`: sin datos personales, P8, van a un issue de prioridad baja); cerrar `/serums-ia/profesion` para inscripciones cruzadas existentes (lo decide el usuario caso por caso); admins por institución; cambio de institución por el propio postulante.

## Seguridad y privacidad
La validación vive en el servidor; la UI solo refleja. Test de regresión con el caso que reprodujo qa.

## i18n
Estado vacío y mensaje del 403 (inscripción, práctica y simulacro) en es/en, en `dashboard`, ambas claves en los dos archivos. Sin «oficial», «certificado» ni «acreditado» referidos a Capacitate (E2 decisión 10).

## Definición de terminado
Tests de API: listado filtrado, 403 por institución en inscripción, práctica y simulacro (también con inscripción cruzada aprobada sembrada), nivel intacto, nivel vacío, orden de errores. `tsc`/`lint`/`build`. Contrato en `docs/api/plataforma-multi-institucion.md`. El PR pega el conteo y los ids de inscripciones cruzadas existentes. Review limpio.
