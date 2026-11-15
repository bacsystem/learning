---
id: PLT-08
titulo: Regla de grado exigido genérica
epica: PLT-E2
fase: Diferida
estado: Diferida (alcance de la ronda: solo PLT-06, 11, 12, 13)
prioridad: Alta
estimacion: 2
roles: [admin]
depende_de: [PLT-07]
---

# PLT-08 Regla de grado exigido genérica

**Como** admin, **quiero** que un proceso pueda exigir un grado de su institución, **para** modelar procesos como el pase de efectividad sin que el código conozca «SO 3°».

## Contexto
`catalogs/validation.go:8-22` exige hoy que el grado de un proceso `pase_efectividad` tenga abreviatura «SO 3°». Decisión de la épica: el tipo de proceso tiene `exige_grado` ([README, PLT-E2](README.md#épica-plt-e2-multi-institución), decisión 6).

## Reglas de negocio
1. Se elimina `ValidateSO3Grado`. Nueva validación: si el tipo del proceso tiene `exige_grado`, `grado_id` es obligatorio y el grado debe pertenecer a la **misma institución** que el proceso.
2. Si el tipo no exige grado, `grado_id` es opcional; si viene, debe ser de la misma institución.
3. Los procesos existentes de pase de efectividad siguen válidos con «SO 3°» (es un dato, no una regla).
4. Cambiar el tipo de un proceso a uno que exige grado sin enviar grado: 422.

## Criterios de aceptación
```gherkin
Escenario: Tipo que exige grado, sin grado
  Dado un tipo con exige_grado de la institución A
  Cuando el admin crea un proceso de ese tipo sin grado_id
  Entonces recibe 422

Escenario: Grado de otra institución
  Dado un proceso de A y un grado de B
  Cuando el admin los asocia
  Entonces recibe 422

Escenario: Cualquier grado de la institución
  Dado un tipo con exige_grado y un grado "Capitán" de A
  Cuando el admin crea el proceso con ese grado
  Entonces se crea (ya no se exige "SO 3°")

Escenario: Procesos existentes
  Dado un proceso de pase_efectividad con "SO 3°" creado antes del cambio
  Cuando el admin lo edita sin tocar el grado
  Entonces se guarda sin error

Escenario: Tipo que no exige grado
  Dado el tipo "admision" sin exige_grado
  Cuando el admin crea un proceso sin grado_id
  Entonces se crea
```

## Fuera de alcance
Exigir más de un grado o rangos de grado; reglas por especialidad.

## Seguridad y privacidad
Solo admin, validación en servidor.

## i18n
Mensajes de validación por código; la UI los muestra en es/en (ver PLT-09).

## Definición de terminado
`validation.go` sin referencias a «SO 3°»; tests de los cinco escenarios. Review limpio.
