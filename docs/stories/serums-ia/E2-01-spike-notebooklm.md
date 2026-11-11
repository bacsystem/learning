---
id: SIA-E2-01
titulo: Spike - reevaluar búsqueda semántica (pgvector vs NotebookLM)
epica: E2
fase: v3
prioridad: Baja
estimacion: 2
roles: [admin]
depende_de: [SIA-E2-02]
---

# SIA-E2-01 Spike: reevaluar búsqueda semántica (pgvector vs NotebookLM)

**Como** equipo de producto, **quiero** comparar con evidencia la búsqueda semántica sobre PostgreSQL (pgvector) y la API de NotebookLM frente al `SourceRepository` propio, **para** decidir en v3 si conviene añadir un adaptador de recuperación semántica (SIA-E2-08) sin cambiar lo que ya funciona.

## Contexto
Decisión del usuario (P1, issue #65): no se autoriza cuenta ni proyecto de NotebookLM por ahora (costo $0 USD). El MVP y la v2 operan al 100 % sobre el `SourceRepository` propio en PostgreSQL (SIA-E2-02). Este spike se reevalúa en v3. Habilitadora: ninguna historia de MVP ni de v2 depende de ella; solo SIA-E2-08. Time-box: 4 horas.

## Reglas de negocio
1. No se crean cuentas, proyectos ni recursos en servicios externos ni se llama a APIs de pago; el costo del spike es $0 USD. Lo que requiera cuenta o pago queda como pregunta al usuario.
2. Se compara: (a) recuperación actual del adaptador propio (línea base), (b) pgvector en el mismo PostgreSQL, (c) NotebookLM (solo documentación pública oficial de Google).
3. Por opción se documenta: calidad esperada de recuperación, costo mensual estimado en USD, dependencia externa, residencia de datos, términos sobre uso de contenido, esfuerzo de implementación y efecto en el presupuesto de IA de SIA-E6-02.
4. Cada afirmación lleva enlace y fecha de consulta; lo no verificado se marca «no verificado».
5. La prueba de pgvector, si se hace, usa solo fixtures sintéticos en local (nunca datos de `assets/`).
6. El resultado termina en una recomendación única: mantener solo adaptador propio, añadir pgvector o añadir NotebookLM, con sus condiciones (costo, cuenta requerida, aprobación del usuario).

## Criterios de aceptación
```gherkin
Escenario: Informe comparativo
  Dado el time-box de 4 horas
  Cuando termina el spike
  Entonces existe docs/design/busqueda-semantica-spike.md con una tabla que compara adaptador propio, pgvector y NotebookLM, cada fila con enlace oficial y fecha de consulta

Escenario: Recomendación
  Dado el informe del spike terminado
  Cuando el po y el tech-lead lo revisan
  Entonces el informe concluye una sola opción recomendada y lista sus condiciones (costo en USD, cuenta requerida, aprobación del usuario)

Escenario: Costo cero
  Dado el spike en ejecución
  Cuando se revisa el historial de acciones del spike
  Entonces no se crearon cuentas, proyectos ni llamadas de pago, y lo que hacía falta quedó como pregunta al usuario

Escenario: Sin efecto en MVP ni v2
  Dado el informe del spike terminado
  Cuando el po revisa las dependencias de las historias
  Entonces ninguna historia de fase MVP o v2 depende de SIA-E2-01 ni de SIA-E2-08
```

## Fuera de alcance
Implementar cualquier adaptador; crear cuentas o proyectos en Google Cloud.

## Dependencias
SIA-E2-02 (línea base del adaptador propio).

## Seguridad y privacidad
No se envían datos de usuarios ni documentos reales a ningún servicio.

## i18n
No aplica (documento interno).

## Definición de terminado
Informe revisado por tech-lead y po; decisión registrada en el README de la épica y, si implica costo o cuenta externa, aprobada por el usuario.
