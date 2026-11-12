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
8. Cada pregunta válida lleva una lista de `alertas_estructura` cuando el parser tuvo que intervenir: `alternativas_separadas` (venían pegadas en una línea), `vinetas_normalizadas`, `html_eliminado`. Sirve para que el admin revise la consistencia estructural en SIA-E3-05 (decisión P2, issue #65); una pregunta sin intervención lleva la lista vacía. `vinetas_normalizadas` se marca solo cuando la pregunta tiene alternativas con viñetas sin letra (`-`, `*`, `•`) o mezcla estilos de etiqueta. Un estilo uniforme con letra (`a)`, `A.`, `a.`) se normaliza en silencio y no genera la alerta (decisión del PO P-B5, 12-nov: así no se mete ruido en la revisión de SIA-E3-05).
9. **Entrada** (decisión del PO, 12-nov): el parser lee el `contenido` original de la versión vigente de la fuente, no los pasajes. Los números de línea son los del archivo original, los mismos de `pasajes.linea_inicio/linea_fin` (ADR 0003).
10. **Motivos de «con problemas»** (lista cerrada de diez): `sin_clave`, `enunciado_vacio`, `alternativas_insuficientes` (menos de 2), `alternativas_excedidas` (más de 6), `alternativas_duplicadas`, `alternativa_vacia` (etiqueta sin texto, por ejemplo «B)» sola; nunca se descarta en silencio), `etiquetas_ambiguas` (regla 17), `numeracion_rota`, `enunciado_largo` (más de 2 000 caracteres) y `alternativa_larga` (más de 500). Si una pregunta tiene varios motivos, se informan todos.
    - Etiquetas fuera de orden (A, B, D): se reetiquetan por posición, cada una con su texto y su marca, y la pregunta lleva la alerta `vinetas_normalizadas`. No es un problema.
    - `<mark>` sin cerrar: no marca clave. Se elimina como HTML (alerta `html_eliminado`) y, si no queda ninguna clave, el motivo es `sin_clave`.
11. **Numeración rota:** se compara solo con la pregunta inmediatamente anterior. Si su número no es el anterior + 1 (salto o repetición), va a «con problemas» con `numeracion_rota` y el parseo sigue contando desde ese número. Una numeración que vuelve a 1 tras un encabezado de sección cuenta como inicio de secuencia nueva, no como numeración rota.
12. **Límite de 1 000:** cuentan todos los bloques de pregunta detectados, válidos y con problemas. El parser devuelve el error `limite_preguntas` y no produce resultado; SIA-E3-03 lo traduce a 422.
13. **Marca de clave:** una alternativa es correcta si cualquier parte de su texto, incluida la etiqueta, está dentro de `<mark>…</mark>`. Las etiquetas `<mark>` no quedan en el texto guardado.
14. **Determinista:** la misma entrada produce siempre la misma salida (mismo orden, mismos textos, mismas alertas), para que la vista previa y la confirmación de SIA-E3-03 coincidan.
15. El parser **no** detecta duplicados contra el banco ni crea `conflicto_clave`: eso es de SIA-E3-06, sobre los borradores ya importados.
16. **HTML residual:** solo se eliminan las etiquetas HTML reconocibles (`<b>dosis</b>` queda «dosis»). Las etiquetas `<script>` y `<style>` se eliminan junto con su contenido. Todo esto genera la alerta `html_eliminado`. Un `<` o `>` que no forma una etiqueta («PA < 90», «Hb <7 g/dL») es texto y se conserva tal cual, sin alerta.
17. **Ante la duda, «con problemas»** (decisión del PO, 12-nov, por el caso «C. difficile»): el parser nunca une, parte ni reetiqueta alternativas si eso puede cambiar qué texto queda marcado como clave. Una vez abierta la primera alternativa de una pregunta, va a «con problemas» con `etiquetas_ambiguas` si ocurre cualquiera de estas dos cosas:
    - una línea empieza con algo que parece una etiqueta (una letra con cualquiera de las formas `A)`, `A.`, `A-` o `(A)`), pero el parser la trataría como continuación de la alternativa anterior;
    - las letras de las etiquetas retroceden o se repiten (por ejemplo A, C, B).
    
    Dentro del enunciado, antes de la primera alternativa, una línea como «E. coli…» sigue siendo continuación sin problema. Un salto hacia adelante sin retroceso (A, B, D) sigue la regla 10: se reetiqueta con la alerta `vinetas_normalizadas`. Preferimos que el admin corrija el archivo antes que importar una clave equivocada. **Letras G–Z** (decisión del PO, 12-nov, borde encontrado por qa): una vez abierta una alternativa, una línea con forma de etiqueta y letra G–Z nunca se une en silencio a la anterior. Si es justo la letra siguiente a la última alternativa abierta (G después de F), abre una alternativa nueva, y la pregunta va a «con problemas» con `alternativas_excedidas` (más de 6). En cualquier otro caso va a `etiquetas_ambiguas`. Esto vale solo para las líneas pegadas a una alternativa. **Tras una línea en blanco** (decisión del PO, 12-nov, review del #232), una línea G–Z con forma de etiqueta que sea justo la letra siguiente continúa la pregunta (y da `alternativas_excedidas`). Cualquier otra cierra la pregunta y queda como texto fuera de ella (regla 7), por ejemplo una referencia «S. Pérez, 2020» al pie. No se une a nada, así que la clave no puede corromperse. Excepción a «se informan todos» (regla 10): si la pregunta es ambigua pero hay una `<mark>` en su rango, solo se informa `etiquetas_ambiguas`, no `sin_clave`. Así no se le dice al admin que falta una clave que sí está.

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
  Dado un archivo con 995 preguntas válidas y 6 con problemas (1 001 bloques)
  Cuando se parsea
  Entonces devuelve el error "limite_preguntas" sin resultado parcial

Escenario: Varios motivos
  Dado una pregunta sin <mark> y con una sola alternativa
  Cuando se parsea
  Entonces se lista como "con problemas" con los motivos "sin_clave" y "alternativas_insuficientes"

Escenario: Numeración rota
  Dado las preguntas 1, 2, 4 y 5 seguidas
  Cuando se parsea
  Entonces la 4 se lista como "con problemas" con motivo "numeracion_rota" y su número de línea, y la 5 es válida

Escenario: Numeración por secciones
  Dado las preguntas 1 a 30, un encabezado de sección y otra vez las preguntas 1 a 20
  Cuando se parsea
  Entonces ninguna tiene el motivo "numeracion_rota"

Escenario: Textos demasiado largos
  Dado una pregunta con un enunciado de 2 001 caracteres y otra con una alternativa de 501
  Cuando se parsea
  Entonces la primera tiene el motivo "enunciado_largo" y la segunda "alternativa_larga"

Escenario: Marca sobre la etiqueta
  Dado la alternativa "<mark>C)</mark> Tres"
  Cuando se parsea
  Entonces C es la correcta y su texto guardado es "Tres", sin etiquetas <mark>

Escenario: HTML residual
  Dado el enunciado "Ajuste la <b>dosis</b><script>x</script> diaria"
  Cuando se parsea
  Entonces el texto guardado es "Ajuste la dosis diaria" y la pregunta lleva la alerta "html_eliminado"

Escenario: Signos menor y mayor que no son HTML
  Dado el enunciado "Paciente con PA < 90 mmHg y Hb <7 g/dL"
  Cuando se parsea
  Entonces el texto guardado es idéntico y la pregunta no lleva la alerta "html_eliminado"

Escenario: Líneas del original
  Dado una pregunta que ocupa las líneas 120 a 126 del archivo original
  Cuando se parsea
  Entonces su rango de origen es 120–126

Escenario: Determinismo
  Dado el mismo archivo parseado dos veces
  Cuando se comparan los resultados
  Entonces son idénticos

Escenario: Conservación del texto
  Dado un enunciado y alternativas con espacios y viñetas irregulares
  Cuando se parsea
  Entonces el texto de enunciado y alternativas no se reescribe (solo se normalizan espacios y viñetas y se quitan las etiquetas de las reglas 13 y 16)

Escenario: Resumen
  Dado un archivo con preguntas válidas y con problemas
  Cuando termina el parseo
  Entonces devuelve totales: válidas, con problemas, tipo unica/multiple y número de preguntas con alertas de estructura

Escenario: Alerta de estructura
  Dado "A) Uno B) Dos C) Tres D) Cuatro" en una sola línea
  Cuando se parsea
  Entonces la pregunta es válida y lleva la alerta "alternativas_separadas"

Escenario: Alternativa vacía
  Dado una pregunta con "A) Uno", "B)" y "C) Tres", con C marcada
  Cuando se parsea
  Entonces se lista como "con problemas" con motivo "alternativa_vacia"

Escenario: Etiquetas fuera de orden
  Dado una pregunta con alternativas "A) Uno", "B) Dos" y "<mark>D) Tres</mark>"
  Cuando se parsea
  Entonces queda válida con A–C, "Tres" como la correcta y la alerta "vinetas_normalizadas"

Escenario: Marca sin cerrar
  Dado una pregunta cuya única marca es "<mark>C) Tres" sin cierre
  Cuando se parsea
  Entonces se lista como "con problemas" con motivo "sin_clave"

Escenario: Nombre científico con inicial en el enunciado
  Dado el enunciado "¿Cuál es el tratamiento de la infección por" seguido de la línea "E. coli enterohemorrágica?" y luego las alternativas A–D
  Cuando se parsea
  Entonces "E. coli enterohemorrágica?" sigue siendo parte del enunciado y la pregunta tiene 4 alternativas A–D

Escenario: Nombre científico dentro de las alternativas
  Dado las líneas "A) Antibiótico contra", "C. difficile con vancomicina", "<mark>B) Metronidazol</mark>" y "C) Otro"
  Cuando se parsea
  Entonces la pregunta se lista como "con problemas" con motivo "etiquetas_ambiguas" y nunca como válida con una clave unida a otro texto

Escenario: Etiqueta entre paréntesis que retrocede
  Dado las líneas "(a) Uno", "(c) Dos", "<mark>(b) Tres</mark>" y "(d) Cuatro"
  Cuando se parsea
  Entonces se lista como "con problemas" solo con el motivo "etiquetas_ambiguas", sin "sin_clave", porque la marca existe

Escenario: Séptima alternativa con letra
  Dado una pregunta con alternativas A) a F) y "<mark>G) Siete</mark>"
  Cuando se parsea
  Entonces se lista como "con problemas" con motivo "alternativas_excedidas" y ninguna alternativa contiene "G) Siete" unido a otro texto

Escenario: Letra lejana dentro de las alternativas
  Dado las líneas "A) Uno", "S. aureus resistente", "B) Dos" y "C) Tres"
  Cuando se parsea
  Entonces se lista como "con problemas" con motivo "etiquetas_ambiguas"

Escenario: Referencia al pie tras línea en blanco
  Dado la última pregunta con alternativas A) a D), una línea en blanco y la línea "S. Pérez, 2020"
  Cuando se parsea
  Entonces la pregunta es válida con 4 alternativas y "S. Pérez, 2020" queda fuera de ella, sin motivo "etiquetas_ambiguas"

Escenario: Letras que retroceden
  Dado una pregunta con alternativas etiquetadas A, C, B y D
  Cuando se parsea
  Entonces se lista como "con problemas" con motivo "etiquetas_ambiguas"

Escenario: Etiquetas uniformes con letra
  Dado una pregunta con alternativas "a.", "b.", "c." y "d.", una por línea
  Cuando se parsea
  Entonces se normalizan a A–D y la pregunta no lleva la alerta "vinetas_normalizadas"

Escenario: Viñetas sin letra o mezcladas
  Dado una pregunta con alternativas "- Uno", "B) Dos", "• Tres" y "D. Cuatro"
  Cuando se parsea
  Entonces se normalizan a A–D y la pregunta lleva la alerta "vinetas_normalizadas"

Escenario: Sin intervención
  Dado una pregunta con una alternativa por línea, etiquetas A–D y sin HTML
  Cuando se parsea
  Entonces la pregunta lleva alertas_estructura vacía
```

## Fuera de alcance
Clasificar por área (SIA-E3-04), persistir (SIA-E3-03), deduplicar y `conflicto_clave` (SIA-E3-06), PDF.

## Dependencias
SIA-E3-02 (modelo de borrador), SIA-E2-05.

## Seguridad y privacidad
El contenido se trata como datos; el HTML residual se elimina (regla 16), y `<mark>` solo se usa para detectar la clave.

## i18n
Los diez motivos de la regla 10 y las tres alertas de la regla 8 se muestran en la UI de SIA-E3-03 con claves es/en; el parser solo devuelve los códigos.

## Definición de terminado
Suite de pruebas con fixtures sintéticos (sin datos de assets/) cubriendo cada regla; review limpio.
