---
id: SIA-E6-02
titulo: Control de costos - presupuesto, límites y caché
epica: E6
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin, usuario]
depende_de: [SIA-E6-01]
---

# SIA-E6-02 Control de costos: presupuesto, límites y caché

**Como** admin, **quiero** límites de uso de IA por usuario y por admin, un presupuesto global y una caché, **para** que el costo sea predecible y no haya abusos.

## Reglas de negocio
1. Límite por postulante: 5 operaciones de IA por día y 60 por mes (configurables); por admin: 200 operaciones de generación por día.
2. Presupuesto global mensual (en unidades de costo configurables); al llegar al 80 % se notifica a admins y al 100 % se bloquean las operaciones no esenciales.
3. Verificación en servidor antes de cada llamada; excedido = 429 `limite_ia` con la hora de reinicio (día/mes en America/Lima).
4. Caché: misma operación + misma entrada normalizada + misma versión de fuente = respuesta reutilizada, sin contar contra el límite de usuario; vigencia 30 días o hasta cambio de fuente.
5. El reintento de una operación fallida no consume límite.
6. Admins configuran límites desde un panel y cada cambio se audita.
7. El postulante ve su consumo restante (día y mes).
8. Operaciones concurrentes no pueden superar el límite (control atómico).

## Criterios de aceptación
```gherkin
Escenario: Dentro del límite
  Dado un usuario con 2 operaciones hoy
  Cuando solicita otra
  Entonces se ejecuta y su consumo muestra 3 de 5

Escenario: Límite diario
  Dado un usuario con 5 operaciones hoy
  Cuando solicita otra
  Entonces recibe 429 "limite_ia" con la hora de reinicio y no se llama al proveedor

Escenario: Límite mensual
  Dado un usuario con 60 operaciones en el mes
  Cuando solicita otra
  Entonces recibe 429 "limite_ia_mensual"

Escenario: Caché
  Dado una operación idéntica ya hecha con la misma versión de fuente
  Cuando un usuario la solicita de nuevo
  Entonces se sirve de caché, no se llama al proveedor y no consume límite

Escenario: Invalidación
  Dado una respuesta en caché asociada a una fuente
  Cuando la fuente cambia de versión y se repite la operación
  Entonces la caché asociada deja de usarse

Escenario: Concurrencia
  Dado un usuario con 4 operaciones y 3 peticiones simultáneas
  Cuando se procesan las 3 peticiones a la vez
  Entonces solo 1 se ejecuta y las demás reciben 429

Escenario: Alerta de presupuesto
  Dado un presupuesto global mensual en curso
  Cuando el gasto llega al 80 %
  Entonces los admins reciben una notificación una sola vez en el mes

Escenario: Presupuesto agotado
  Dado el 100 % del presupuesto consumido
  Cuando se solicita una operación no esencial
  Entonces responde 429 "presupuesto_agotado"

Escenario: Auditoría
  Dado un admin en el panel de límites
  Cuando cambia un límite
  Entonces audit_log registra valor anterior y nuevo
```

## Fuera de alcance
Cobro al usuario o planes de pago.

## Dependencias
SIA-E6-01.

## Seguridad y privacidad
Los contadores son por id opaco; el consumo de un usuario solo lo ve él y los admins agregados.

## i18n
Consumo, errores y panel en es/en.

## Definición de terminado
Tests de concurrencia y de reinicio por zona horaria Lima; migración con Down; `tsc`, `lint`, `build`; review limpio.
