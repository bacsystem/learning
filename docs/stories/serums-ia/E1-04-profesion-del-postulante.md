---
id: SIA-E1-04
titulo: Profesión del postulante para SERUMS
epica: E1
fase: MVP
prioridad: Alta
estimacion: 2
roles: [usuario]
depende_de: [SIA-E1-03]
---

# SIA-E1-04 Profesión del postulante para SERUMS

**Como** postulante, **quiero** indicar mi profesión y el proceso SERUMS al que me preparo, **para** que el diagnóstico, el dashboard y el plan usen las áreas y pesos correctos.

## Contexto
Se reutiliza la inscripción aprobada y la especialidad del catálogo. Si el usuario ya tiene especialidad en su inscripción, se preselecciona.

## Reglas de negocio
1. Solo se puede elegir entre procesos donde el usuario tiene inscripción aprobada (verificado en servidor).
2. La profesión es una especialidad activa del catálogo con configuración de pesos (completa o estimada).
3. Cambiar de profesión reinicia el modelo de dominio de esa (proceso, especialidad) tras confirmación; los intentos históricos se conservan.
4. Máximo un par (proceso, profesión) activo por usuario.

## Criterios de aceptación
```gherkin
Escenario: Elegir profesión
  Dado un usuario con inscripción aprobada en SERUMS 2026-I
  Cuando elige "Enfermería"
  Entonces se guarda y el módulo muestra las áreas de Enfermería

Escenario: Sin inscripción aprobada
  Dado un usuario con inscripción pendiente
  Cuando intenta elegir profesión
  Entonces recibe 403 y la pantalla explica que necesita inscripción aprobada

Escenario: Cambio de profesión
  Dado un usuario con dominio calculado
  Cuando cambia de profesión y confirma
  Entonces el dominio de la profesión anterior se archiva y el nuevo comienza vacío

Escenario: Profesión inválida
  Dado un usuario con inscripción aprobada
  Cuando envía una especialidad inexistente
  Entonces recibe 422

Escenario: Aislamiento
  Dado dos usuarios con profesión elegida
  Cuando un usuario consulta la profesión de otro usuario
  Entonces recibe 404
```

## Fuera de alcance
Múltiples profesiones activas a la vez.

## Dependencias
SIA-E1-03; inscripciones existentes.

## Seguridad y privacidad
La profesión no es dato sensible, pero no se registra junto a datos personales en logs.

## i18n
Textos del selector y confirmación en es/en.

## Definición de terminado
Estados carga/vacío/error; tests de inscripción y aislamiento; claves es/en; review limpio.
