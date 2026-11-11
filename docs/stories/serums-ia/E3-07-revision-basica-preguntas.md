---
id: SIA-E3-07
titulo: Revisión básica de preguntas — API de citas y aprobación
epica: E3
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [SIA-E3-02, SIA-E2-02]
---

# SIA-E3-07 Revisión básica de preguntas — API de citas y aprobación

**Como** admin, **quiero** buscar pasajes, añadir citas a una pregunta y aprobarla, rechazarla, reabrirla o revalidarla por API, **para** que exista una vía para publicar preguntas manuales ahora que todas nacen en `borrador` (SIA-E3-02). La pantalla es SIA-E3-08.

## Contexto
Decisión del PO (P1 del contrato #152): toda pregunta nueva nace en `borrador`, incluidas el alta manual y la importación Excel (origen `manual`). Las colas especializadas no cubren el origen `manual`: SIA-E3-05 es solo para `balotario` y SIA-E6-05 solo para `ia`. Esta historia da la vía mínima y va justo después de SIA-E3-02. **SIA-E3-02 no entra en un release sin SIA-E3-07 y SIA-E3-08.**

Las transiciones de estado son las del contrato de SIA-E3-02 (#152). Esta historia añade sus guardas, las citas y la búsqueda de pasajes para admin, que hoy existe solo como puerto `BuscarPasajes` (SIA-E2-02) y no por HTTP. El architect define el contrato.

## Reglas de negocio
Todas se validan en el servidor, en la transición o en la operación, sea cual sea la pantalla o el cliente.

1. **Búsqueda de pasajes (admin):** por texto, solo en fuentes `activa`, por relevancia, con máx. 20 resultados. Cada resultado trae ids de fuente, versión y pasaje, título de la fuente, nº de versión y un extracto. Nunca trae rutas de archivo.
2. **Citas:**
   - Añadir o quitar citas a una pregunta en cualquier estado salvo `eliminada`. Máx. 10 citas por pregunta (tabla de Límites): 422 `limite_citas`.
   - Solo se puede añadir una cita a un pasaje de una fuente `activa`: 422 `fuente_no_activa`.
   - Añadir o quitar citas no cambia el estado. Cada cambio se audita.
3. **Aprobar** (`borrador` → `aprobada`) por esta vía: **solo origen `manual`**.
   - Para origen `balotario` u `ia` responde 409 `revision_especializada`: se aprueban en SIA-E3-05 o SIA-E6-05, con sus exigencias (ruta de verificación, nota para lo incierto).
   - Exige ≥1 cita a una fuente `activa` (SIA-E3-02, regla 3): 422 `falta_cita`.
   - Exige área asignada, regla nueva de esta historia, igual que SIA-E3-05 regla 10: 422 `falta_area`.
   - Exige la confirmación «verificado» enviada por el cliente, validada en el servidor: 422 `verificacion_requerida`.
4. **Revalidar** (`en_revision` → `aprobada`) por esta vía: cualquier origen, porque una pregunta llega a `en_revision` por fuente retirada, reportes o edición, y no existe otra pantalla para revalidarla en el MVP.
   - Exige lo mismo que aprobar (cita activa, área y confirmación).
   - Para origen `balotario` o `ia` exige además una **nota de 10–500 caracteres**: 422 `nota_obligatoria`.
   - Si la pregunta tiene `conflicto_clave` sin resolver (SIA-E3-06), responde 409 `conflicto_clave` por cualquier vía.
5. **Rechazar** (`borrador` o `en_revision` → `rechazada`): cualquier origen, con motivo de 5–500 caracteres.
6. **Reabrir** (`rechazada` → `borrador`): cualquier origen.
7. Una transición inválida o concurrente responde 409 `estado_invalido`.
8. Toda acción se audita según SIA-E3-02. La nota y el motivo van solo en `audit_log`, nunca en los logs de aplicación.

## Criterios de aceptación
```gherkin
Escenario: Aprobar una pregunta manual con cita
  Dado una pregunta "borrador" de origen "manual" con área y una cita a una fuente activa
  Cuando el admin la aprueba con la confirmación
  Entonces pasa a "aprobada", se sirve en prácticas nuevas y audit_log registra la aprobación

Escenario: Aprobar sin cita
  Dado una pregunta "borrador" manual sin citas
  Cuando el admin la aprueba
  Entonces recibe 422 "falta_cita"

Escenario: Aprobar sin confirmación
  Dado una pregunta "borrador" manual con cita y área
  Cuando el admin la aprueba sin la confirmación
  Entonces recibe 422 "verificacion_requerida"

Escenario: Origen especializado
  Dado una pregunta "borrador" de origen "balotario"
  Cuando el admin intenta aprobarla por esta vía
  Entonces recibe 409 "revision_especializada" y sigue en "borrador"

Escenario: Revalidar tras fuente retirada
  Dado una pregunta manual "en_revision" porque su única fuente se retiró
  Cuando el admin añade una cita a otra fuente activa y la revalida con la confirmación
  Entonces pasa a "aprobada"

Escenario: Revalidar una de IA sin nota
  Dado una pregunta de origen "ia" en "en_revision" con cita activa
  Cuando el admin la revalida sin nota
  Entonces recibe 422 "nota_obligatoria"

Escenario: Conflicto de clave
  Dado una pregunta "en_revision" con conflicto de clave sin resolver
  Cuando el admin intenta revalidarla
  Entonces recibe 409 "conflicto_clave"

Escenario: Cita a fuente retirada
  Dado un pasaje de una fuente retirada
  Cuando el admin intenta añadirlo como cita
  Entonces recibe 422 "fuente_no_activa"

Escenario: Límite de citas
  Dado una pregunta con 10 citas
  Cuando el admin añade otra
  Entonces recibe 422 "limite_citas"

Escenario: Búsqueda solo en fuentes activas
  Dado una fuente retirada y otra activa que contienen "dengue"
  Cuando el admin busca "dengue"
  Entonces solo recibe pasajes de la fuente activa, sin rutas de archivo

Escenario: Rechazar y reabrir
  Dado una pregunta "borrador"
  Cuando el admin la rechaza con motivo "Clave incorrecta" y luego la reabre
  Entonces pasa a "rechazada" y después vuelve a "borrador", con ambas acciones en audit_log

Escenario: Concurrencia
  Dado una pregunta que otro admin acaba de rechazar
  Cuando un admin intenta aprobarla
  Entonces recibe 409 "estado_invalido"

Escenario: Acceso
  Dado un usuario con rol usuario
  Cuando llama a buscar pasajes, a citas o a cualquier transición
  Entonces recibe 403
```

## Fuera de alcance
Pantalla (SIA-E3-08), aprobación masiva, colas especializadas (SIA-E3-05, SIA-E6-05) y segundo revisor.

## Dependencias
SIA-E3-02 (estados, transiciones y citas) y SIA-E2-02 (puerto `BuscarPasajes`).

## Seguridad y privacidad
Solo admin, con 401/403 y `jsonError`. Guardas en el servidor. La búsqueda no expone rutas ni pasajes de fuentes no activas.

## i18n
Sin texto visible; los códigos de error los traduce SIA-E3-08.

## Definición de terminado
Tests de API de cada regla y escenario (guardas por origen, nota, conflicto de clave, límites de citas, 403 y 409). Contrato en `docs/api/`. Review limpio.
