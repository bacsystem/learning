---
id: SIA-E9-03
titulo: Calibración de dificultad con estadísticas
epica: E9
fase: v2
prioridad: Media
estimacion: 3
roles: [admin]
depende_de: [SIA-E4-04]
---

# SIA-E9-03 Calibración de dificultad con estadísticas

**Como** admin, **quiero** comparar la dificultad etiquetada de cada pregunta con su tasa real de acierto, **para** corregir etiquetas y mejorar el diagnóstico y las prácticas.

## Contexto
Usa `pregunta_estadisticas` (existente). Sin IRT: reglas simples sobre la tasa de acierto. El sistema sugiere; el admin decide.

## Reglas de negocio
1. Se evalúa una pregunta solo con ≥ 30 respuestas de al menos 10 usuarios distintos.
2. Tasa de acierto observada: ≥ 0,75 → sugiere `facil`; 0,45–0,74 → `medio`; < 0,45 → `dificil`.
3. Se marca «discrepante» si la sugerida difiere de la etiquetada; con tasa < 0,15 en una pregunta de clave única se marca además «posible clave errónea» y pasa a la cola de revisión (SIA-E6-05 / `en_revision`).
4. Listado de discrepancias paginado (20 por defecto, máx. 100) con tasa, n de respuestas y etiqueta actual/sugerida; filtros por área y por tipo de discrepancia.
5. Acciones del admin: aplicar la sugerencia (individual o masiva, máx. 50) o descartarla; todo se audita.
6. Las estadísticas cuentan a cada usuario una sola vez por pregunta, con su primer intento.

## Criterios de aceptación
```gherkin
Escenario: Discrepancia
  Dado una pregunta "dificil" con 60 respuestas de 20 usuarios y tasa 0,82
  Cuando el admin abre el listado de discrepancias
  Entonces aparece con sugerencia "facil"

Escenario: Datos insuficientes
  Dado una pregunta con 25 respuestas
  Cuando el admin abre el listado de discrepancias
  Entonces no se evalúa

Escenario: Posible clave errónea
  Dado una pregunta de clave única con tasa 0,10 en 50 respuestas
  Cuando corre la calibración
  Entonces se marca "posible_clave_erronea" y pasa a "en_revision"

Escenario: Aplicar sugerencia
  Dado una discrepancia con sugerencia "facil"
  Cuando el admin aplica la sugerencia
  Entonces cambia la dificultad y se audita antes/después

Escenario: Límite masivo
  Dado 51 discrepancias seleccionadas
  Cuando el admin las aplica a la vez
  Entonces recibe 422 "limite_masivo"

Escenario: Un usuario cuenta una vez
  Dado un usuario que respondió la misma pregunta 5 veces
  Cuando se calcula la tasa de acierto
  Entonces solo su primer intento cuenta para la tasa

Escenario: Permisos
  Dado un usuario con rol usuario
  Cuando consulta las discrepancias
  Entonces recibe 403
```

## Fuera de alcance
IRT; ajuste automático sin aprobación.

## Dependencias
SIA-E4-04, `pregunta_estadisticas`.

## Seguridad y privacidad
Datos agregados, sin identificar usuarios.

## i18n
Etiquetas de discrepancia en es/en.

## Definición de terminado
Tests de umbrales (0,44 / 0,45 / 0,74 / 0,75) y de conteo por usuario; review limpio.
