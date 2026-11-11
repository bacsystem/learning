---
id: SIA-E7-03
titulo: Vista del plan y registro de cumplimiento
epica: E7
fase: MVP
prioridad: Alta
estimacion: 3
roles: [usuario]
depende_de: [SIA-E7-02]
---

# SIA-E7-03 Vista del plan y registro de cumplimiento

**Como** postulante, **quiero** ver mi plan por día y marcar lo que cumplí, **para** seguir mi avance.

## Reglas de negocio
1. Vista «Hoy» y vista semanal; cada bloque con área, tipo, minutos y acceso directo (material, práctica dirigida).
2. Estados de bloque: `pendiente`, `hecho`, `omitido`. Se puede marcar hecho/omitido en el día actual y hasta 7 días atrás; no en el futuro.
3. Si el bloque es de práctica y el usuario completó una práctica de esa área ese día, se sugiere marcarlo hecho (no automático).
4. Progreso: % de minutos hechos en la semana y total del plan.
5. Estados de UI: carga, vacío (sin plan: llamada a configurar), error.
6. Solo el dueño ve y modifica su plan.

## Criterios de aceptación
```gherkin
Escenario: Ver hoy
  Dado un plan activo
  Cuando el usuario abre la vista «Hoy»
  Entonces muestra los bloques del día con minutos y accesos directos

Escenario: Marcar hecho
  Dado un bloque pendiente de hoy
  Cuando marca el bloque como hecho
  Entonces el progreso semanal se actualiza

Escenario: Marcar el futuro
  Dado un bloque programado para mañana
  Cuando intenta marcarlo como hecho
  Entonces recibe 422 "fecha_futura"

Escenario: Marcar hace 8 días
  Dado un bloque de hace 8 días
  Cuando intenta marcarlo como hecho
  Entonces recibe 422 "fuera_de_ventana"

Escenario: Sin plan
  Dado un usuario sin plan activo
  Cuando abre la vista del plan
  Entonces ve «Aún no tienes un plan» y el botón «Crear mi plan»

Escenario: Bloque ajeno
  Dado un bloque del plan de otro usuario
  Cuando un usuario lo marca como hecho
  Entonces recibe 404

Escenario: Estados de UI
  Dado la vista del plan
  Cuando la petición está en curso o falla
  Entonces hay carga y error con reintento
```

## Fuera de alcance
Reprogramar bloques manualmente.

## Dependencias
SIA-E7-02.

## Seguridad y privacidad
Autorización por dueño.

## i18n
Todo en es/en; fechas y días de la semana según locale.

## Definición de terminado
`tsc`, `lint`, `build`; tests de ventana de edición; review limpio.
