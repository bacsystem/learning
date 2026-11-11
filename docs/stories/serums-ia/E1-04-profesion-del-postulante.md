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
Se reutiliza la inscripción aprobada y la especialidad del catálogo. La inscripción no tiene especialidad; la del usuario está en sus datos de persona (`personas.especialidad_id`). Esa especialidad solo sirve para preseleccionar la profesión: el usuario la confirma o elige otra.

## Reglas de negocio
1. Solo se puede elegir entre procesos donde el usuario tiene inscripción aprobada (verificado en servidor).
2. La profesión es una especialidad activa del catálogo con configuración de pesos (completa o estimada).
3. Cambiar de profesión reinicia el modelo de dominio de esa (proceso, especialidad) tras confirmación; los intentos históricos se conservan.
4. Máximo un par (proceso, profesión) activo por usuario.
5. Preselección (decisión PO, contrato MVP E1+E2): si el usuario aún no eligió profesión, el selector viene con la especialidad de sus datos de persona, siempre que cumpla la regla 2. Si no la cumple (inactiva o inexistente), el selector viene vacío. El servidor toma ese dato de la base de datos, no de la sesión. La preselección es solo una sugerencia: no se guarda nada hasta que el usuario confirma.
6. Elegir o cambiar la profesión en este módulo no modifica los datos de persona del usuario; la elección se guarda aparte por (usuario, proceso).

## Criterios de aceptación
```gherkin
Escenario: Elegir profesión
  Dado un usuario con inscripción aprobada en SERUMS 2026-I
  Cuando elige "Enfermería"
  Entonces se guarda y el módulo muestra las áreas de Enfermería

Escenario: Preselección desde los datos de persona
  Dado un usuario con inscripción aprobada, sin profesión elegida y con especialidad "Enfermería" activa en sus datos de persona
  Cuando abre el selector de profesión
  Entonces "Enfermería" aparece preseleccionada y no se guarda nada hasta que confirma

Escenario: Sin preselección válida
  Dado un usuario cuya especialidad en sus datos de persona está inactiva
  Cuando abre el selector de profesión
  Entonces el selector aparece sin preselección

Escenario: Elegir otra profesión no cambia los datos de persona
  Dado un usuario con especialidad "Enfermería" en sus datos de persona
  Cuando elige "Obstetricia" y confirma
  Entonces se guarda "Obstetricia" para el módulo y su especialidad de persona sigue siendo "Enfermería"

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
Múltiples profesiones activas a la vez; editar la especialidad de los datos de persona.

## Dependencias
SIA-E1-03; inscripciones existentes.

## Seguridad y privacidad
La profesión no es dato sensible, pero no se registra junto a datos personales en logs.

## i18n
Textos del selector y confirmación en es/en.

## Definición de terminado
Estados carga/vacío/error; tests de inscripción y aislamiento; claves es/en; review limpio.
