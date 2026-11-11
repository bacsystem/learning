---
id: SIA-E9-04
titulo: Nota estimada con intervalo de confianza
epica: E9
fase: v3
prioridad: Media
estimacion: 5
roles: [usuario]
depende_de: [SIA-E4-04, SIA-E9-05]
---

# SIA-E9-04 Nota estimada con intervalo de confianza

**Como** postulante, **quiero** una nota estimada para el examen con su rango de incertidumbre, **para** saber qué tan cerca estoy de mi meta.

## Contexto
Estimación orientativa. Nunca promete resultado; la UI debe decirlo.

## Reglas de negocio
1. Nota estimada = Σ sobre áreas (peso_área × dominio_área) × 100, usando el dominio de SIA-E4-03.
2. Intervalo del 80 %: para cada área, intervalo Beta sobre (1 + aciertos efectivos, 1 + fallos efectivos) y combinación ponderada por peso; se reportan límites inferior y superior redondeados a 1 punto.
3. Se muestra solo con ≥ 100 respuestas efectivas totales y dominio con confianza ≥ media en las áreas que suman ≥ 70 % del peso; si no, «Datos insuficientes» con el faltante concreto.
4. Si hay ≥ 2 simulacros con formato oficial (SIA-E9-05), se compara la estimación con su promedio y se avisa si difieren más de 10 puntos.
5. Siempre acompañada del texto «Estimación orientativa; no garantiza tu resultado».
6. Si hay una nota mínima configurada por admin para el proceso/profesión, se muestra la probabilidad de superarla como rango (inferior–superior), no como promesa; si no hay, no se muestra.
7. Histórico semanal conservado (máx. 52 puntos).

## Criterios de aceptación
```gherkin
Escenario: Estimación disponible
  Dado 150 respuestas efectivas y confianza media en áreas que suman 80 % del peso
  Cuando el usuario abre su nota estimada
  Entonces ve una nota estimada con límites inferior y superior

Escenario: Datos insuficientes
  Dado 60 respuestas efectivas
  Cuando el usuario abre su nota estimada
  Entonces ve «Datos insuficientes» y «Te faltan 40 respuestas»

Escenario: Intervalo
  Dado una nota estimada calculada
  Cuando el sistema devuelve el resultado
  Entonces el límite inferior es menor o igual a la estimación y esta menor o igual al superior

Escenario: Divergencia con simulacros
  Dado simulacros oficiales con promedio 58 y estimación 72
  Cuando el usuario abre su nota estimada
  Entonces ve el aviso de divergencia

Escenario: Texto de advertencia
  Dado una nota estimada disponible
  Cuando el usuario abre la pantalla
  Entonces el aviso de orientativo siempre es visible

Escenario: Sin nota mínima
  Dado un proceso sin nota mínima configurada
  Cuando el usuario abre su nota estimada
  Entonces no se muestra probabilidad de aprobar

Escenario: Aislamiento
  Dado la nota estimada de otro usuario
  Cuando un usuario la consulta
  Entonces recibe 404

Escenario: Reproducibilidad
  Dado los mismos datos de dominio y pesos
  Cuando se calcula dos veces
  Entonces el resultado es idéntico
```

## Fuera de alcance
Predicción con aprendizaje automático; comparación con otros postulantes.

## Dependencias
SIA-E4-04, SIA-E9-05, SIA-E1-03.

## Seguridad y privacidad
Datos solo del dueño.

## i18n
Textos, advertencia y rangos en es/en.

## Definición de terminado
Tests con casos numéricos de intervalo; revisión de copy por el PO; review limpio.
