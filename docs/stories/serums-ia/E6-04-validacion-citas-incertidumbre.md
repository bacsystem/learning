---
id: SIA-E6-04
titulo: Validación de citas y marca de incertidumbre
epica: E6
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin]
depende_de: [SIA-E6-03]
---

# SIA-E6-04 Validación de citas y marca de incertidumbre

**Como** admin, **quiero** que todo contenido generado se valide automáticamente contra sus citas y que lo dudoso quede marcado, **para** centrar mi revisión donde hay más riesgo.

## Contexto
Es la barrera automática previa a la revisión humana; no la sustituye. Se aplica a preguntas, material y explicaciones.

## Reglas de negocio
1. Una cita es válida si el pasaje existe, pertenece a una fuente `activa` y a la versión indicada, y su texto contiene respaldo verificable: se comprueba que el fragmento citado literal (campo `extracto`, ≤300 caracteres) aparece en el pasaje tras normalización.
2. Contenido con cualquier cita inválida: se descarta si no le quedan citas válidas; si le quedan, se conserva con la etiqueta `cita_invalida_eliminada`.
3. Marca de incertidumbre (`incierto = true`) cuando ocurra al menos una: el modelo declara confianza < 0,80; una alternativa distractora aparece también respaldada en los pasajes; la clave no tiene extracto de apoyo; las citas provienen de menos de 1 pasaje distinto; hay discrepancia entre pasajes citados.
4. Lo marcado `incierto` guarda sus motivos (lista de códigos) visibles al admin.
5. Contenido `incierto` no puede aprobarse sin revisión explícita (ver SIA-E6-05).
6. Salida de IA que no cumple el esquema se rechaza (SIA-E6-01).

## Criterios de aceptación
```gherkin
Escenario: Cita válida
  Dado una pregunta cuyo extracto aparece textualmente en el pasaje citado
  Cuando el validador la procesa
  Entonces la cita es válida

Escenario: Extracto inventado
  Dado un extracto que no aparece en el pasaje citado
  Cuando el validador lo procesa
  Entonces la cita es inválida y se elimina

Escenario: Sin citas válidas
  Dado una pregunta con todas las citas inválidas
  Cuando el validador la procesa
  Entonces se descarta

Escenario: Fuente retirada
  Dado una cita a un pasaje de una fuente retirada
  Cuando el validador la procesa
  Entonces es inválida

Escenario: Baja confianza
  Dado una pregunta con confianza declarada 0,70
  Cuando el validador la procesa
  Entonces el contenido queda "incierto" con motivo "confianza_baja"

Escenario: Distractor respaldado
  Dado una alternativa incorrecta que también aparece respaldada en los pasajes
  Cuando el validador la procesa
  Entonces queda "incierto" con motivo "distractor_respaldado"

Escenario: Contenido claro
  Dado citas válidas, confianza 0,92 y respaldo de la clave
  Cuando el validador lo procesa
  Entonces queda sin marca de incertidumbre

Escenario: Idempotencia
  Dado un contenido ya validado
  Cuando se valida dos veces el mismo contenido
  Entonces el resultado es idéntico
```

## Fuera de alcance
Verificación clínica de la veracidad (la hace el admin).

## Dependencias
SIA-E6-03, SIA-E2-06.

## Seguridad y privacidad
La validación es determinista y local; no envía datos a terceros.

## i18n
Motivos de incertidumbre visibles al admin con claves es/en.

## Definición de terminado
Tabla de pruebas por cada motivo; sin dependencia de red; review limpio.
