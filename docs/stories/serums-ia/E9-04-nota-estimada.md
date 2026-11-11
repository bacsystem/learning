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
3. Se muestra solo con configuración de pesos `completa` (SIA-E1-03; con pesos estimados se muestra «Datos insuficientes: faltan los pesos oficiales de tu profesión»), ≥ 100 respuestas efectivas totales y dominio con confianza ≥ media en las áreas que suman ≥ 70 % del peso; si no, «Datos insuficientes» con el faltante concreto.
4. Si hay ≥ 2 simulacros con formato oficial (SIA-E9-05), se compara la estimación con su promedio y se avisa si difieren más de 10 puntos.
5. Siempre acompañada del texto «Estimación orientativa; no garantiza tu resultado».
6. Decisión P7 (issue #65): solo se muestran la nota estimada y su intervalo; **nunca una probabilidad de aprobar**. Como referencia visual puede mostrarse la nota mínima del proceso, tomada de la columna existente `configuracion_examen.nota_minima` (escala 0–100) de la configuración activa del proceso; no se crea otro campo de nota mínima. Si el proceso no tiene configuración de examen activa, no se muestra referencia.
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

Escenario: Sin probabilidad de aprobar
  Dado una nota estimada disponible y un proceso con configuración de examen activa
  Cuando el usuario abre su nota estimada
  Entonces ve la nota estimada y su intervalo y en ninguna parte de la pantalla ni de la respuesta de la API aparece una probabilidad de aprobar

Escenario: Referencia de nota mínima
  Dado un proceso cuya configuración de examen activa tiene nota_minima 55
  Cuando el usuario abre su nota estimada
  Entonces ve la línea de referencia «Nota mínima: 55» tomada de configuracion_examen.nota_minima

Escenario: Sin configuración de examen activa
  Dado un proceso sin configuración de examen activa
  Cuando el usuario abre su nota estimada
  Entonces no se muestra referencia de nota mínima ni probabilidad

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
Predicción con aprendizaje automático; comparación con otros postulantes; probabilidad de aprobar (decisión P7).

## Dependencias
SIA-E4-04, SIA-E9-05, SIA-E1-03; `configuracion_examen` existente (solo lectura de `nota_minima`).

## Seguridad y privacidad
Datos solo del dueño.

## i18n
Textos, advertencia y rangos en es/en.

## Definición de terminado
Tests con casos numéricos de intervalo; revisión de copy por el PO; review limpio.
