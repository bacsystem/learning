---
id: SIA-E6-08
titulo: Explicaciones con cita por respuesta
epica: E6
fase: v2
prioridad: Alta
estimacion: 5
roles: [admin, usuario]
depende_de: [SIA-E6-05]
---

# SIA-E6-08 Explicaciones con cita por respuesta

**Como** postulante, **quiero** que cada pregunta revisada tenga una explicación con su fuente, **para** entender por qué la clave es correcta y por qué las demás no.

## Contexto
Extiende `pregunta_retroalimentacion` (retroalimentación existente). La explicación se genera con IA, se valida (SIA-E6-04) y la aprueba un admin; mientras no exista una aprobada, la revisión muestra solo la clave.

## Reglas de negocio
1. Una explicación pertenece a una pregunta aprobada; contiene: razón de la clave (≤120 palabras), motivo breve por cada alternativa incorrecta (≤40 palabras c/u) y ≥1 cita con extracto.
2. Estados: `borrador`, `aprobada`, `rechazada`, `en_revision` (mismas reglas de aprobación y de incertidumbre que SIA-E6-05).
3. Generación por admin: individual o por lote de máx. 20 preguntas; consume límites.
4. Si la fuente citada se retira, la explicación pasa a `en_revision` y deja de mostrarse.
5. En la revisión de práctica, simulacro o diagnóstico, el postulante ve la explicación solo si está aprobada y después de finalizar el intento.
6. Las retroalimentaciones manuales existentes siguen mostrándose.

## Criterios de aceptación
```gherkin
Escenario: Generar explicaciones por lote
  Dado 20 preguntas aprobadas sin explicación
  Cuando el admin genera explicaciones
  Entonces se crean 20 borradores con citas válidas o se descartan las que no las tengan

Escenario: Lote excesivo
  Dado un admin autenticado
  Cuando pide explicaciones para 21 preguntas
  Entonces recibe 422

Escenario: Aprobar
  Dado una explicación en borrador con citas válidas
  Cuando el admin marca la verificación y aprueba
  Entonces la explicación se muestra en las revisiones

Escenario: Incierta sin nota
  Dado una explicación marcada incierta
  Cuando el admin intenta aprobarla sin nota
  Entonces recibe 422 "nota_obligatoria"

Escenario: Mostrar en revisión
  Dado un simulacro finalizado con una pregunta con explicación aprobada
  Cuando el usuario abre la revisión
  Entonces la revisión muestra clave, mi respuesta, explicación y citas

Escenario: Intento en curso
  Dado un simulacro en curso
  Cuando el cliente pide los datos de una pregunta
  Entonces ninguna explicación se envía al cliente

Escenario: Sin explicación aprobada
  Dado una pregunta sin explicación aprobada
  Cuando el usuario abre la revisión de un intento finalizado
  Entonces la revisión muestra solo la clave y la retroalimentación manual si existe

Escenario: Fuente retirada
  Dado una explicación aprobada cuya fuente se retira
  Cuando el usuario abre la revisión
  Entonces la explicación deja de mostrarse hasta nueva aprobación
```

## Fuera de alcance
Explicaciones conversacionales o chat tutor.

## Dependencias
SIA-E6-05.

## Seguridad y privacidad
La explicación nunca viaja al cliente antes de finalizar el intento; sin datos personales en prompts.

## i18n
Rótulos «Explicación», «Fuente», «Por qué no» en es/en; el texto está en español.

## Definición de terminado
Tests de visibilidad por estado y por intento; migración con Down; `tsc`, `lint`, `build`; review limpio.
