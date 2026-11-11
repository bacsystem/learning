---
id: SIA-E5-04
titulo: Acción rápida - práctica dirigida
epica: E5
fase: MVP
prioridad: Alta
estimacion: 3
roles: [usuario]
depende_de: [SIA-E5-03]
---

# SIA-E5-04 Acción rápida: práctica dirigida

**Como** postulante, **quiero** crear una práctica de mis áreas débiles con un clic, **para** entrenar justo lo que más necesito.

## Contexto
Reutiliza `practicas` (inscripción aprobada obligatoria). Solo cambia la selección de preguntas: áreas priorizadas en lugar de elegidas a mano.

## Reglas de negocio
1. Por defecto 20 preguntas (editable 5–100) repartidas entre las áreas débiles (máx. 3) en proporción a su puntaje.
2. Solo preguntas aprobadas; se prefieren las no vistas en los últimos 14 días y, si hay temas bajos, se sesga hacia ellos.
3. Si el área tiene menos preguntas que las pedidas, se usan las disponibles y se informa; si el total es 0, 422 `sin_preguntas`.
4. Exige inscripción aprobada (403 si no); la práctica creada es del usuario.
5. Se puede elegir manualmente un área desde el dashboard («Practicar esta área»).
6. Si los pesos de la profesión no están completos (SIA-E1-03), la priorización usa pesos iguales; la práctica se crea igual, guarda `pesos_estimados: true` y muestra el aviso «Pesos estimados» (decisión P6, issue #65).

## Criterios de aceptación
```gherkin
Escenario: Práctica dirigida
  Dado 3 áreas débiles con puntajes 0,18 / 0,12 / 0,06
  Cuando pulsa «Practicar mis áreas débiles» con 20 preguntas
  Entonces se crea una práctica con 10, 7 y 3 preguntas (proporcional, redondeado a 20)

Escenario: Menos preguntas disponibles
  Dado un área con 4 preguntas aprobadas
  Cuando el usuario pide 10 preguntas de esa área entre sus áreas débiles
  Entonces se crean 4 de esa área, el resto se reparte entre las demás y se informa

Escenario: Sin preguntas
  Dado ninguna pregunta aprobada en las áreas débiles
  Cuando el usuario pulsa «Practicar mis áreas débiles»
  Entonces recibe 422 "sin_preguntas"

Escenario: Límite de cantidad
  Dado un usuario con inscripción aprobada
  Cuando pide 101 preguntas
  Entonces recibe 422

Escenario: Sin inscripción aprobada
  Dado un usuario sin inscripción aprobada
  Cuando pulsa «Practicar mis áreas débiles»
  Entonces recibe 403

Escenario: Pesos estimados
  Dado una profesión con configuración de pesos "incompleta"
  Cuando el usuario pulsa «Practicar mis áreas débiles»
  Entonces se crea la práctica con "pesos_estimados": true y se muestra el aviso «Pesos estimados»

Escenario: Área manual
  Dado el dashboard con varias áreas
  Cuando pulsa «Practicar esta área» en una sola área
  Entonces las preguntas son solo de esa área
```

## Fuera de alcance
Simulacro dirigido (SIA-E5-05).

## Dependencias
SIA-E5-03, prácticas existentes.

## Seguridad y privacidad
Estado de preguntas verificado en servidor; autorización por dueño.

## i18n
Botón, mensajes de reparto y errores en es/en.

## Definición de terminado
Tests de reparto proporcional, límites e inscripción; `tsc`, `lint`, `build`; review limpio.
