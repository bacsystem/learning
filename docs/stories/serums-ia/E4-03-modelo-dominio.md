---
id: SIA-E4-03
titulo: Modelo de dominio por área y tema
epica: E4
fase: MVP
prioridad: Alta
estimacion: 5
roles: [usuario]
depende_de: [SIA-E4-01]
---

# SIA-E4-03 Modelo de dominio por área y tema

**Como** postulante, **quiero** que el sistema mantenga una estimación de mi dominio por área y tema, **para** que dashboard, plan y repaso se basen en mi desempeño real.

## Contexto
El dominio es un servicio de cálculo determinista (sin IA). Se almacena por (usuario, proceso, especialidad, área, tema nulo para el nivel de área).

## Reglas de negocio
1. Estimación bayesiana simple: `dominio = (1 + Σ wᵢ·cᵢ) / (2 + Σ wᵢ)`, con `cᵢ` ∈ [0,1] (1 acierto; en múltiple, fracción de aciertos correctos sin penalizar erróneas) y prior uniforme (1,1).
2. Peso temporal: `wᵢ = 0,5^(días_desde_respuesta / 30)`; las respuestas de hace 60 días pesan 0,25.
3. `n_efectivo = Σ wᵢ`; confianza `baja` si n_efectivo < 5, `media` 5–14, `alta` ≥ 15.
4. Nivel: `bajo` < 0,50, `medio` 0,50–0,74, `alto` ≥ 0,75.
5. El dominio de área agrega todas las respuestas de sus temas y de preguntas sin tema.
6. Se recalcula de forma incremental por respuesta y puede reconstruirse completo desde el historial (operación admin idempotente).
7. Solo cuentan respuestas a preguntas que estuvieran aprobadas al responderse.
8. Un usuario solo ve su propio dominio.

## Criterios de aceptación
```gherkin
Escenario: Sin datos
  Dado un usuario sin respuestas en un área
  Cuando el sistema calcula el dominio del área
  Entonces el dominio es 0,50, confianza "baja" y n_efectivo 0

Escenario: Cálculo
  Dado 8 respuestas de hoy con 6 aciertos
  Cuando el sistema calcula el dominio
  Entonces dominio = (1+6)/(2+8) = 0,70, nivel "medio", confianza "media"

Escenario: Decaimiento
  Dado una respuesta fallida de hace 30 días
  Cuando el sistema calcula su peso temporal
  Entonces su peso es 0,5

Escenario: Selección múltiple
  Dado una pregunta con 2 claves de las que acierta 1 sin marcar incorrectas
  Cuando el sistema calcula su contribución
  Entonces c = 0,5

Escenario: Reconstrucción
  Dado el historial de respuestas de un usuario
  Cuando el admin ejecuta la reconstrucción dos veces
  Entonces los valores resultantes son idénticos

Escenario: Aislamiento
  Dado el dominio de otro usuario
  Cuando un usuario pide ese dominio
  Entonces recibe 404

Escenario: Cambio de profesión
  Dado un usuario con dominio calculado en una profesión
  Cuando cambia de profesión
  Entonces el dominio de la anterior queda archivado y no se mezcla
```

## Fuera de alcance
Modelos IRT; nota estimada (SIA-E9-04).

## Dependencias
SIA-E4-01 (primeras respuestas), SIA-E1-01.

## Seguridad y privacidad
Datos de aprendizaje personales: acceso solo del dueño; borrado en SIA-E9-07; sin datos personales en logs.

## i18n
Niveles y confianza con claves es/en.

## Definición de terminado
Migración con Down; tests de fórmula con tabla de casos; reconstrucción idempotente; review limpio.
