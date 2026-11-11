---
id: SIA-E5-01
titulo: Dashboard de rendimiento por área
epica: E5
fase: MVP
prioridad: Alta
estimacion: 5
roles: [usuario]
depende_de: [SIA-E4-03]
---

# SIA-E5-01 Dashboard de rendimiento por área

**Como** postulante, **quiero** ver mi dominio por área con su peso en el examen, **para** saber cómo voy y qué área atender primero.

## Reglas de negocio
1. Una fila/tarjeta por área de mi profesión: nombre, peso en el examen (%), dominio (%), nivel, confianza, nº de respuestas efectivas y fecha de última actividad.
2. Orden por defecto: prioridad (SIA-E5-03); alternativa: peso o dominio.
3. Resumen superior: días que faltan para el examen (si hay plan), nº de áreas bajas y última actividad.
4. Si no hay diagnóstico, se muestra el estado vacío con llamada a rendir el diagnóstico (SIA-E4-01).
5. Si los pesos son estimados (SIA-E1-03), se indica «pesos estimados».
6. Las áreas con confianza baja muestran «pocos datos».
7. Los datos son solo del usuario autenticado; exige inscripción aprobada.

## Criterios de aceptación
```gherkin
Escenario: Dashboard con datos
  Dado un usuario con diagnóstico y 6 áreas
  Cuando abre el dashboard
  Entonces ve 6 áreas con peso, dominio, nivel y confianza, ordenadas por prioridad

Escenario: Sin diagnóstico
  Dado un usuario sin respuestas
  Cuando abre el dashboard
  Entonces ve «Aún no tenemos datos» y el botón «Rendir diagnóstico»

Escenario: Pesos estimados
  Dado una profesión sin pesos completos
  Cuando el usuario abre el dashboard
  Entonces se muestra la nota «pesos estimados»

Escenario: Pocos datos
  Dado un área con confianza baja
  Cuando el usuario abre el dashboard
  Entonces la fila muestra «pocos datos»

Escenario: Cambio de orden
  Dado el dashboard cargado con orden por prioridad
  Cuando el usuario elige ordenar por peso
  Entonces el orden cambia sin nueva petición ni parpadeo de carga completa

Escenario: Sin inscripción aprobada
  Dado un usuario sin inscripción aprobada
  Cuando abre el dashboard
  Entonces recibe 403 y se muestra el motivo

Escenario: Estados de carga y error
  Dado el dashboard solicitando datos
  Cuando la petición está en curso o falla
  Entonces hay un esqueleto de carga y un error con reintento
```

## Fuera de alcance
Detalle por tema (SIA-E5-02), acciones rápidas (SIA-E5-04 a E5-06).

## Dependencias
SIA-E4-03, SIA-E1-03.

## Seguridad y privacidad
Autorización por dueño; sin datos de otros usuarios ni comparativas.

## i18n
Todos los textos y niveles en es/en; formato de porcentaje y fecha según locale.

## Definición de terminado
`tsc`, `lint`, `build`; estados verificados; claves es/en; review limpio.
