---
id: PLT-07
titulo: Catálogo de niveles y tipos de proceso por institución — migración y API
epica: PLT-E2
fase: Diferida
estado: Diferida (alcance de la ronda: solo PLT-06, 11, 12, 13)
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: []
---

# PLT-07 Catálogo de niveles y tipos de proceso por institución — migración y API

**Como** admin, **quiero** que cada institución tenga su propio catálogo de niveles y de tipos de proceso, **para** usar el vocabulario de esa institución sin depender del modelo militar.

## Contexto
Hoy `nivel` solo admite `suboficial|oficial` (`catalogs/dto.go:42,64` y CHECK `grados_nivel_check` de la migración 012) y el tipo de proceso es `ascenso|pase_efectividad|admision` (`dto.go:98`). El architect define tablas y contrato en `docs/api/` y un ADR si cambia el modelo. Decisiones: [README, PLT-E2](README.md#épica-plt-e2-multi-institución) (5 y 7).

## Reglas de negocio
1. Existen dos catálogos por institución: niveles y tipos de proceso (E2 decisión 5). Los tipos llevan `exige_grado`.
2. `grados.nivel`, `especialidades.nivel`, `procesos.nivel` y `procesos.tipo` pasan a validarse contra el catálogo **de la institución del registro**. Se elimina el `oneof` de `dto.go` y el CHECK de la migración 012. Un valor que no está en el catálogo de esa institución se rechaza con 422 (`nivel_invalido` o `tipo_invalido`). Un valor inactivo solo se acepta en la edición de un registro que ya lo tenía.
3. **`procesos.nivel` hoy no se puede escribir por la API** (falta en `catalogs/dto.go:95-102` y en el INSERT y UPDATE de `repository.go:610,629`), así que en producción vale siempre NULL (P5). Esta historia agrega `nivel` **opcional** a `POST` y `PUT /admin/procesos` (ausente o `null` = todos los niveles) y lo devuelve en la respuesta. Se valida contra el catálogo como los demás.
4. Migración de datos según la E2 decisión 7: reversible, idempotente, sin cambiar ningún valor existente. Vacío en `procesos.nivel` y `especialidades.nivel` sigue significando «todos los niveles».
5. CRUD admin: crear, renombrar, reordenar, inactivar. Código único por institución. No se elimina un valor en uso.
6. Toda acción de admin se audita (`audit_log`), como el resto de catálogos.

## Criterios de aceptación
```gherkin
Escenario: Migración conserva los datos
  Dado la base con grados, especialidades, procesos e inscripciones existentes
  Cuando se aplica la migración
  Entonces, por cada institución, todo nivel y tipo en uso existe en su catálogo (pase_efectividad con exige_grado), y ninguna fila de grados, procesos ni inscripciones cambia

Escenario: Nivel vacío se conserva
  Dado un proceso y una especialidad con nivel vacío
  Cuando se aplica la migración y luego se consultan
  Entonces siguen con nivel vacío y siguen significando «todos los niveles»

Escenario: Migración reversible
  Dado la migración aplicada
  Cuando se revierte y se vuelve a aplicar
  Entonces termina sin error y sin duplicar valores

Escenario: Nivel propio de otra institución
  Dado la institución B con el nivel "cadete"
  Cuando el admin crea un grado de B con nivel "cadete"
  Entonces se crea; y con nivel "cadete" en un grado de A responde 422

Escenario: Proceso con nivel
  Dado el nivel "oficial" activo en la institución A
  Cuando el admin crea un proceso de A con nivel "oficial", y otro sin nivel
  Entonces el primero guarda y devuelve "oficial"; el segundo devuelve nivel vacío y sigue significando «todos los niveles»; con un nivel que no es de A responde 422

Escenario: Valor en uso
  Dado el nivel "oficial" usado por grados
  Cuando el admin intenta eliminarlo
  Entonces recibe 409; puede inactivarlo y los grados existentes lo conservan

Escenario: Código duplicado
  Dado el tipo "ascenso" en la institución A
  Cuando el admin crea otro "ascenso" en A
  Entonces recibe 409; en B se permite

Escenario: Permisos
  Dado un usuario con rol usuario
  Cuando llama al catálogo de administración
  Entonces recibe 403
```

## Fuera de alcance
Pantallas (PLT-09); la regla de grado (PLT-08); traducir los nombres del catálogo (son datos de la institución, texto libre en un solo idioma); catálogos por proceso.

## Seguridad y privacidad
Solo admin; nada de datos personales. Migración probada contra una copia de la base antes del PR.

## i18n
Sin texto visible; los errores de la API usan los códigos del contrato y la UI los traduce.

## Definición de terminado
Tests de migración (subida/bajada), de validación por institución y de permisos. Contrato en `docs/api/`. Review limpio.
