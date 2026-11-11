---
id: SIA-E2-02
titulo: Puerto SourceRepository y adaptador propio
epica: E2
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin]
depende_de: []
---

# SIA-E2-02 Puerto SourceRepository y adaptador propio

**Como** equipo de producto, **quiero** que las fuentes y sus pasajes se accedan mediante un puerto `SourceRepository` con un adaptador propio sobre Postgres, **para** no depender de un proveedor externo y poder citar pasajes con exactitud.

## Contexto
El puerto define: guardar fuente, listar fuentes, obtener pasajes de una fuente, buscar pasajes relevantes para (profesión, consulta, con área y tema opcionales) y resolver una cita (id de pasaje) a su texto y metadatos. Búsqueda propia con texto completo de Postgres.

## Reglas de negocio
1. Una cita es la tupla (fuente, versión, pasaje); es inmutable: la versión de una cita nunca cambia aunque la fuente se actualice.
2. La búsqueda devuelve máx. 20 pasajes, ordenados por relevancia, solo de fuentes en estado `activa` y de la profesión pedida.
3. La consulta de búsqueda: 3–300 caracteres.
4. El puerto no expone datos del usuario; solo contenido académico.
5. Tablas con migración goose y Down funcional.
6. Área y tema en la búsqueda (decisión PO, contrato MVP E1+E2): en el MVP las fuentes no tienen área ni tema, así que la búsqueda los acepta pero no filtra por ellos. Solo filtran la profesión y el estado `activa`. Quien busca (SIA-E6-03, SIA-E6-06) pone el nombre del área o del tema en el texto de la consulta para que cuente en la relevancia.

## Criterios de aceptación
```gherkin
Escenario: Buscar pasajes
  Dado 3 fuentes activas con pasajes sobre "vigilancia epidemiológica"
  Cuando el servicio busca "vigilancia epidemiológica" para Medicina
  Entonces recibe hasta 20 pasajes con id de fuente, versión, id de pasaje y texto, ordenados por relevancia

Escenario: Fuente inactiva excluida
  Dado una fuente en estado "retirada" con pasajes coincidentes
  Cuando el servicio busca un texto que esos pasajes contienen
  Entonces sus pasajes no aparecen

Escenario: Área y tema no filtran en el MVP
  Dado una fuente activa de Medicina con pasajes sobre "vigilancia epidemiológica" y sin área ni tema
  Cuando el servicio busca "vigilancia epidemiológica" para Medicina indicando el área "Salud pública" y un tema
  Entonces recibe los mismos pasajes que sin indicar área ni tema

Escenario: Resolver cita histórica
  Dado una cita a la versión 1 de una fuente que ya va por la versión 3
  Cuando el servicio resuelve la cita
  Entonces devuelve el texto exacto de la versión 1

Escenario: Consulta fuera de límite
  Dado el puerto SourceRepository disponible
  Cuando el servicio busca con una consulta de 2 caracteres o de más de 300
  Entonces el puerto devuelve el error "consulta_invalida"

Escenario: Intercambiabilidad
  Dado un adaptador de prueba que implementa el puerto
  Cuando se ejecuta la suite de contrato sobre ambos adaptadores
  Entonces el adaptador propio y el de prueba pasan los mismos casos
```

## Fuera de alcance
Búsqueda semántica (pgvector o NotebookLM): se reevalúa en v3 (SIA-E2-01, SIA-E2-08; decisión P1, issue #65). El MVP y la v2 operan al 100 % sobre este adaptador propio. Asignar área o tema a fuentes o pasajes y filtrar por ellos.

## Dependencias
Ninguna.

## Seguridad y privacidad
Consultas parametrizadas; el contenido de fuentes no se registra en logs (solo ids).

## i18n
Sin texto visible.

## Definición de terminado
Suite de contrato del puerto; migración con Down; tests de versión histórica; review limpio.
