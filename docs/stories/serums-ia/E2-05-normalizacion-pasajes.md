---
id: SIA-E2-05
titulo: Normalización y pasajes citables
epica: E2
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [SIA-E2-02]
---

# SIA-E2-05 Normalización y pasajes citables

**Como** admin, **quiero** que cada fuente se normalice y se divida en pasajes con ubicación, **para** que cada cita apunte a un fragmento verificable.

## Contexto
Los textos extraídos de PDF traen formato roto (alternativas pegadas, viñetas inconsistentes). La normalización es determinista y no usa IA.

## Reglas de negocio
1. Normalización: UTF-8, saltos de línea `\n`, colapso de espacios, unificación de viñetas, eliminación de caracteres de control. Las marcas `<mark>` se conservan.
2. Segmentación por encabezados y párrafos; cada pasaje tiene 150–800 palabras. Un fragmento de menos de 150 palabras (sección corta o resto final) se une al pasaje anterior (si es el primero, al siguiente); si al unirse pasa de 800, el conjunto se divide en partes de 150–800. Los balotarios usan la misma segmentación, con unión incluida. Cada pregunta importada cita los pasajes que se solapan con su rango de líneas (SIA-E3-03). Decisión del PO P-B4 (12-nov, ADR 0008): revierte la Q7 «un pasaje por pregunta», porque el hash único impide reingerir los balotarios ya cargados. Única excepción (decisión PO, contrato MVP E1+E2): si el documento entero tiene de 1 a 149 palabras, se publica como un único pasaje.
3. Cada pasaje guarda: orden, rango de líneas de origen, encabezado/sección y texto.
4. Máx. 5 000 pasajes por fuente; si se excede, la ingesta falla sin publicar nada.
5. El proceso es idempotente: mismo contenido, mismos pasajes y mismo orden.

## Criterios de aceptación
```gherkin
Escenario: Segmentación
  Dado un .md de 40 000 palabras con encabezados
  Cuando se normaliza
  Entonces todos los pasajes tienen entre 150 y 800 palabras y conservan rango de líneas y sección

Escenario: Sección corta
  Dado un .md con una sección de 60 palabras entre dos secciones largas
  Cuando se normaliza
  Entonces esas 60 palabras quedan unidas al pasaje anterior y ningún pasaje tiene menos de 150 palabras

Escenario: Documento corto
  Dado un .md de 120 palabras
  Cuando se normaliza
  Entonces la fuente queda "activa" con un único pasaje de 120 palabras

Escenario: Idempotencia
  Dado un contenido ya normalizado una vez
  Cuando se procesa de nuevo el mismo contenido
  Entonces los pasajes son idénticos en texto y orden

Escenario: Conserva marcas
  Dado un texto con "<mark>respuesta</mark>"
  Cuando se normaliza
  Entonces el pasaje conserva la marca

Escenario: Exceso de pasajes
  Dado un contenido que produciría 5 001 pasajes
  Cuando se normaliza
  Entonces la fuente queda "fallida" y no hay pasajes publicados

Escenario: Contenido sin texto
  Dado un archivo vacío o solo con espacios
  Cuando se normaliza
  Entonces la fuente queda "fallida" con motivo "sin_contenido"
```

## Fuera de alcance
OCR, embeddings, traducción.

## Dependencias
SIA-E2-02.

## Seguridad y privacidad
El texto se almacena tal cual salvo normalización; no se envía a la IA en esta historia.

## i18n
Sin texto visible (motivos de fallo mapeados a claves es/en en SIA-E2-07).

## Definición de terminado
Tests con casos de formato roto (alternativas pegadas, viñetas mixtas); idempotencia verificada; review limpio.
