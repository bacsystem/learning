---
id: SIA-E1-03
titulo: Pesos por profesión y proceso SERUMS
epica: E1
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [SIA-E1-01]
---

# SIA-E1-03 Pesos por profesión y proceso SERUMS

**Como** admin, **quiero** definir para el proceso SERUMS 2026-I y cada profesión el peso y el número de preguntas de cada área (y opcionalmente de cada tema), **para** que dashboard, plan y simulacro reflejen el examen real.

## Contexto
Extiende `proceso_asignaturas` (peso, numero_preguntas, coeficiente), que hoy es por proceso y asignatura. Hay que poder diferenciarlo por especialidad y, opcionalmente, por tema.

## Reglas de negocio
1. Clave del peso: (proceso, especialidad, asignatura) y opcional tema.
2. Pesos de áreas de una (proceso, especialidad) suman 100 % con tolerancia 0,01; si no, no se puede marcar la configuración como «completa».
3. `numero_preguntas` ≥ 1 y suma por (proceso, especialidad) ≤ 200.
4. Mientras no haya configuración completa, el sistema usa pesos iguales entre las áreas con preguntas y lo informa al usuario.
5. Configuración existente de proceso_asignaturas sigue vigente para procesos que no son SERUMS (compatibilidad).

## Criterios de aceptación
```gherkin
Escenario: Configuración completa
  Dado el proceso SERUMS 2026-I y la especialidad Medicina con 6 áreas
  Cuando el admin guarda pesos que suman 100
  Entonces la configuración queda "completa" y se registra en audit_log

Escenario: Suma inválida
  Dado el proceso SERUMS 2026-I y la especialidad Medicina con 6 áreas
  Cuando el admin guarda pesos que suman 95
  Entonces responde 422 "pesos_no_suman_100" y no marca completa

Escenario: Exceso de preguntas
  Dado el proceso SERUMS 2026-I y la especialidad Medicina con 6 áreas
  Cuando el admin guarda numero_preguntas que suman 210
  Entonces responde 422 "limite_preguntas"

Escenario: Compatibilidad
  Dado un proceso no SERUMS con proceso_asignaturas existente
  Cuando un usuario crea un simulacro
  Entonces el comportamiento no cambia

Escenario: Sin configuración
  Dado una especialidad sin pesos
  Cuando el dashboard pide los pesos
  Entonces recibe pesos iguales y la bandera "pesos_estimados": true

Escenario: Permisos
  Dado un usuario con rol usuario
  Cuando intenta guardar pesos
  Entonces recibe 403
```

## Fuera de alcance
Obtener los pesos oficiales automáticamente; el admin los carga a mano.

## Dependencias
SIA-E1-01. Proceso «SERUMS 2026-I» dado de alta en el catálogo existente.

## Seguridad y privacidad
Solo admin escribe. Auditoría de cada cambio (valores antes/después).

## i18n
Mensajes y etiquetas es/en, incluida la nota «pesos estimados».

## Definición de terminado
Migración con Down; tests de suma, límites y compatibilidad; contrato en docs/api/; review limpio.
