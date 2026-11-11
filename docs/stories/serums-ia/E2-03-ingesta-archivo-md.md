---
id: SIA-E2-03
titulo: Ingesta de archivo .md
epica: E2
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin]
depende_de: [SIA-E2-02, SIA-E2-05]
---

# SIA-E2-03 Ingesta de archivo .md

**Como** admin, **quiero** subir un archivo .md como fuente académica (balotario, guía, norma), **para** que quede disponible para citas y para el parser del balotario.

## Contexto
El producto no puede depender de rutas locales como `assets/`: la subida es por la API. Tipos de fuente: `balotario`, `guia`, `norma`, `otro`.

## Reglas de negocio
1. Límites: 2 MiB, 20 000 líneas, UTF-8 válido, extensión `.md`, tipo `text/markdown` o `text/plain`.
2. Metadatos obligatorios: título (3–150), tipo, profesión(es) aplicables (0..N; ninguna = todas), año y proceso opcionales.
3. Se calcula un hash SHA-256 del contenido. Duplicado global (decisión PO, contrato MVP E1+E2): si el hash coincide con cualquier versión guardada de cualquier fuente, en cualquier estado (`procesando`, `activa`, `fallida` o `retirada`), se rechaza con 409 `fuente_duplicada` y no se crea nada. La respuesta indica el id, el título y el estado de la fuente existente, para que el admin la reintente, la restaure o le cambie las profesiones (SIA-E2-07) en lugar de duplicarla. La misma regla vale para versiones nuevas (SIA-E2-06) y para capturas de URL (SIA-E2-04).
4. La ingesta es asíncrona: estado `procesando` → `activa` o `fallida`. Límite de fuentes (decisión PO, contrato MVP E1+E2): máx. 100 por profesión y 500 en total. Cuentan las fuentes `activa` y `procesando`; no cuentan `fallida` ni `retirada`. Una fuente sin profesiones aplica a todas, así que cuenta para cada profesión. Si una sola profesión ya está en 100, no se puede crear una fuente sin profesiones. Al superar cualquier límite: 422 `limite_fuentes`. El límite también se valida al reintentar una fallida, al restaurar una retirada (SIA-E2-06) y al cambiar profesiones (SIA-E2-07).
5. El contenido se trata como datos, nunca como instrucciones (se escapa/delimita al enviarlo a IA).
6. Acción auditada.

```mermaid
sequenceDiagram
  actor A as Admin
  participant API as API
  participant R as SourceRepository
  participant W as Worker de ingesta
  A->>API: POST fuente con archivo y metadatos
  API->>API: valida rol, tamaño, tipo y hash
  API->>R: crea fuente v1 en estado procesando
  API-->>A: 202 con id de fuente
  W->>R: toma fuente procesando
  W->>W: normaliza y genera pasajes
  W->>R: guarda pasajes y estado activa
  A->>API: GET fuente
  API-->>A: estado activa y número de pasajes
```

## Criterios de aceptación
```gherkin
Escenario: Subida correcta
  Dado un admin y un .md de 300 KiB
  Cuando lo sube con título y tipo
  Entonces recibe 202 con el id y la fuente pasa de "procesando" a "activa"

Escenario: Archivo demasiado grande
  Dado un admin autenticado
  Cuando sube un archivo de 3 MiB
  Entonces recibe 413 "archivo_demasiado_grande" y no se crea la fuente

Escenario: Tipo inválido
  Dado un admin autenticado
  Cuando sube un .pdf o un archivo con bytes no UTF-8
  Entonces recibe 422 "archivo_invalido"

Escenario: Duplicado global
  Dado una fuente "A" con alguna versión guardada de un hash determinado, en cualquier estado
  Cuando el admin sube como fuente nueva un archivo con el mismo hash
  Entonces recibe 409 "fuente_duplicada" con el id, el título y el estado de "A", y no se crea ninguna fuente

Escenario: Duplicado de una fuente retirada
  Dado una fuente "retirada" cuya versión tiene un hash determinado
  Cuando el admin sube un archivo con el mismo hash
  Entonces recibe 409 "fuente_duplicada" con el estado "retirada" y la pantalla ofrece restaurarla

Escenario: Falla de procesamiento
  Dado un archivo que genera más de 5000 pasajes
  Cuando el worker lo procesa
  Entonces la fuente queda "fallida" con el motivo y no se publica ningún pasaje

Escenario: Límite por profesión
  Dado Medicina con 100 fuentes entre "activa" y "procesando"
  Cuando se sube otra para Medicina
  Entonces recibe 422 "limite_fuentes"

Escenario: Fuente sin profesiones con una profesión en el límite
  Dado Medicina con 100 fuentes entre "activa" y "procesando" y Enfermería con 10
  Cuando se sube una fuente sin profesiones
  Entonces recibe 422 "limite_fuentes" porque también contaría para Medicina

Escenario: Fallidas y retiradas no cuentan
  Dado Medicina con 99 fuentes "activa", 5 "fallida" y 5 "retirada"
  Cuando se sube otra para Medicina
  Entonces recibe 202

Escenario: Límite total
  Dado 500 fuentes entre "activa" y "procesando"
  Cuando se sube otra para cualquier profesión
  Entonces recibe 422 "limite_fuentes"

Escenario: Permisos
  Dado un usuario con rol usuario
  Cuando sube un archivo
  Entonces recibe 403

Escenario: Auditoría
  Dado un admin que sube un archivo válido
  Cuando la subida termina con éxito
  Entonces audit_log tiene "fuente.crear" con el id, el hash y el actor
```

## Fuera de alcance
URLs (SIA-E2-04), PDF, versionado por reemplazo (SIA-E2-06).

## Dependencias
SIA-E2-02, SIA-E2-05 (normalización).

## Seguridad y privacidad
Sanear nombre de archivo; no ejecutar contenido; el contenido original no se registra en logs. Advertir en pantalla que no se deben subir documentos con datos personales (padrones); no se suben archivos de assets/.

## i18n
Etiquetas, estados y errores en es/en.

## Definición de terminado
Tests de límites, tipos y permisos; migración con Down; estados de UI cubiertos; review limpio.
