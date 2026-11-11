---
id: SIA-E5-03
titulo: Priorización de áreas débiles
epica: E5
fase: MVP
prioridad: Alta
estimacion: 3
roles: [usuario]
depende_de: [SIA-E5-01]
---

# SIA-E5-03 Priorización de áreas débiles

**Como** postulante, **quiero** que el sistema ordene mis áreas por urgencia, **para** invertir mi tiempo donde más suma a mi nota.

## Contexto
Cálculo determinista, reutilizado por el dashboard, las acciones rápidas y el plan (SIA-E7-02). Se usa siempre el dominio bayesiano de SIA-E4-03: su prior uniforme (1,1) ya regulariza los pocos datos (sin respuestas da 0,50 y con pocas respuestas se acerca a 0,50), así que el diagnóstico de 4 preguntas por área sí cuenta en la priorización (corrección de auditoría, hallazgo A).

```mermaid
flowchart TD
  A[Para cada area] --> B[Dominio usado es el dominio bayesiano de SIA-E4-03]
  B --> C{Confianza baja}
  C -- Si --> D[Marca pocos datos sin cambiar el dominio]
  C -- No --> E[Puntaje es peso por 1 menos dominio usado]
  D --> E
  E --> F{Mas de 14 dias sin practicar el area}
  F -- Si --> G[Puntaje por 1,10]
  F -- No --> H[Puntaje sin cambio]
  G --> I[Ordenar de mayor a menor]
  H --> I
  I --> J{Puntaje igual}
  J -- Si --> K[Desempata por mayor peso y luego por nombre]
  J -- No --> L[Resultado]
  K --> L
```

## Reglas de negocio
1. `puntaje = peso_area × (1 − dominio_usado)`; peso en fracción (0–1). `dominio_usado` es siempre el dominio bayesiano de SIA-E4-03, sin sustituirlo por un valor fijo.
2. Con confianza baja (SIA-E4-03, n_efectivo < 5) solo se muestra la marca «pocos datos»; el puntaje no cambia.
3. Bono de 10 % si el área lleva más de 14 días sin práctica.
4. «Área débil» = dominio_usado < 0,50 (nivel bajo) o estar entre las 3 primeras por puntaje; máx. 3 áreas destacadas.
5. Resultado reproducible: mismas entradas, mismo orden; desempate por mayor peso y luego nombre.
6. Si todas las áreas tienen dominio ≥ 0,75, no hay áreas débiles y se informa.

## Criterios de aceptación
```gherkin
Escenario: Orden por puntaje
  Dado Área A peso 0,30 dominio 0,40 y Área B peso 0,10 dominio 0,20
  Cuando el sistema calcula la prioridad
  Entonces A tiene puntaje 0,18, B tiene 0,08 y A va primero

Escenario: Pocos datos
  Dado un área de peso 0,20 con 1 acierto en 4 respuestas de hoy, dominio bayesiano 2/6 ≈ 0,33 y confianza baja
  Cuando el sistema calcula la prioridad
  Entonces el puntaje usa ese dominio (0,20 × 4/6 ≈ 0,133) y el área se marca «pocos datos»

Escenario: El diagnóstico cuenta en la priorización
  Dado un usuario que solo rindió el diagnóstico: Área A peso 0,20 con 0 de 4 aciertos y Área B peso 0,20 con 4 de 4 aciertos
  Cuando el sistema calcula la prioridad
  Entonces A tiene dominio 1/6 ≈ 0,17 y puntaje ≈ 0,167, B tiene dominio 5/6 ≈ 0,83 y puntaje ≈ 0,033, y A va primero

Escenario: Sin respuestas
  Dado un área sin respuestas y peso 0,30
  Cuando el sistema calcula la prioridad
  Entonces el dominio usado es 0,50 por el prior, el puntaje es 0,15 y se marca «pocos datos»

Escenario: Bono por inactividad
  Dado un área con 20 días sin práctica y puntaje 0,20
  Cuando el sistema calcula la prioridad
  Entonces el puntaje es 0,22

Escenario: Empate
  Dado dos áreas con igual puntaje
  Cuando el sistema ordena las áreas
  Entonces va primero la de mayor peso

Escenario: Todo alto
  Dado todas las áreas con dominio mayor o igual a 0,75
  Cuando el sistema calcula la prioridad
  Entonces se muestra «Sin áreas débiles» y ninguna área destacada

Escenario: Reproducibilidad
  Dado los mismos datos de dominio y pesos
  Cuando se calcula dos veces la prioridad
  Entonces el orden es idéntico
```

## Fuera de alcance
Priorización por tema.

## Dependencias
SIA-E5-01, SIA-E1-03, SIA-E4-03 (dominio bayesiano).

## Seguridad y privacidad
Cálculo por usuario; sin datos de otros.

## i18n
Etiquetas «área débil», «pocos datos», «sin áreas débiles» en es/en.

## Definición de terminado
Tests con tabla de casos (incluye empates, bono, confianza baja y usuario con solo diagnóstico); review limpio.
