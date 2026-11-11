---
id: SIA-E3-01
titulo: Parser del balotario con marca de clave
epica: E3
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin]
depende_de: [SIA-E3-02, SIA-E2-05]
---

# SIA-E3-01 Parser del balotario con marca de clave

**Como** admin, **quiero** que el sistema extraiga de un balotario en .md cada pregunta, sus alternativas y la respuesta correcta marcada con `<mark>`, **para** no digitar cientos de preguntas a mano.

## Contexto
Formato observado: preguntas numeradas, alternativas A–D, clave dentro de `<mark>…</mark>`, sin área ni tema, con alternativas pegadas en una misma línea y viñetas inconsistentes. El parser es determinista (sin IA) y reporta lo que no pudo interpretar en vez de adivinar.

## Reglas de negocio
1. Una pregunta válida tiene enunciado no vacío, 2–6 alternativas etiquetadas A, B, C… y al menos una marcada con `<mark>`.
2. Alternativas pegadas en una línea («A) x B) y») se separan; viñetas distintas (`-`, `*`, `•`, `a)`, `A.`) se normalizan a letras.
3. Una marca → tipo `unica`; dos o más → `multiple`.
4. Preguntas sin marca, con alternativas duplicadas o con numeración rota se devuelven como «con problemas» con el número de línea y el motivo; nunca se descartan en silencio.
5. Límite: 1 000 preguntas por archivo; enunciado máx. 2 000 caracteres, alternativa máx. 500.
6. El resultado guarda el rango de líneas de origen de cada pregunta (trazabilidad).
7. Se excluye cualquier texto fuera de las preguntas (encabezados, pies de página).
8. Cada pregunta válida lleva una lista de `alertas_estructura` cuando el parser tuvo que intervenir: `alternativas_separadas` (venían pegadas en una línea), `vinetas_normalizadas`, `html_eliminado`. Sirve para que el admin revise la consistencia estructural en SIA-E3-05 (decisión P2, issue #65); una pregunta sin intervención lleva la lista vacía.

## Criterios de aceptación
```gherkin
Escenario: Pregunta de clave única
  Dado un bloque con enunciado y alternativas A–D, una con <mark>
  Cuando se parsea
  Entonces produce una pregunta tipo "unica" con la alternativa marcada como correcta

Escenario: Alternativas pegadas
  Dado "A) Uno B) Dos C) Tres D) Cuatro" en una sola línea
  Cuando se parsea
  Entonces produce 4 alternativas separadas

Escenario: Selección múltiple
  Dado una pregunta con dos alternativas marcadas con <mark>
  Cuando se parsea
  Entonces el tipo es "multiple" con ambas como correctas

Escenario: Sin clave
  Dado una pregunta sin <mark>
  Cuando se parsea
  Entonces se lista como "con problemas" con número de línea y motivo "sin_clave"

Escenario: Exceso de preguntas
  Dado un archivo con 1 001 preguntas
  Cuando se parsea
  Entonces responde 422 "limite_preguntas" sin crear nada

Escenario: Conservación del texto
  Dado un enunciado y alternativas con espacios y viñetas irregulares
  Cuando se parsea
  Entonces el texto de enunciado y alternativas no se reescribe (solo se normalizan espacios y viñetas)

Escenario: Resumen
  Dado un archivo con preguntas válidas y con problemas
  Cuando termina el parseo
  Entonces devuelve totales: válidas, con problemas, tipo unica/multiple y número de preguntas con alertas de estructura

Escenario: Alerta de estructura
  Dado "A) Uno B) Dos C) Tres D) Cuatro" en una sola línea
  Cuando se parsea
  Entonces la pregunta es válida y lleva la alerta "alternativas_separadas"

Escenario: Sin intervención
  Dado una pregunta con una alternativa por línea, etiquetas A–D y sin HTML
  Cuando se parsea
  Entonces la pregunta lleva alertas_estructura vacía
```

## Fuera de alcance
Clasificar por área (SIA-E3-04), persistir (SIA-E3-03), PDF.

## Dependencias
SIA-E3-02 (modelo de borrador), SIA-E2-05.

## Seguridad y privacidad
El contenido se trata como datos; HTML residual se escapa o elimina salvo `<mark>`.

## i18n
Motivos de problema mapeados a claves es/en para la UI (SIA-E3-03).

## Definición de terminado
Suite de pruebas con fixtures sintéticos (sin datos de assets/) cubriendo cada regla; review limpio.
