---
id: PLT-04
titulo: Latido del worker de ingesta de fuentes
epica: PLT-E1
fase: v2
prioridad: Media
estimacion: 2
roles: [admin]
depende_de: [PLT-01, PLT-02, SIA-E2-03, SIA-E2-05]
---

# PLT-04 Latido del worker de ingesta de fuentes

**Como** admin, **quiero** ver cuándo dio señal de vida por última vez el worker que procesa las fuentes, **para** saber si una fuente quedó en «procesando» porque el worker está detenido.

## Contexto
El worker de ingesta procesa las fuentes subidas. Está definido en `docs/api/serums-ia-e2-fuentes.md` (SIA-E2-03, normalización SIA-E2-05) y arranca junto al de email. **En su contrato no registra ningún latido**, y esta historia lo incluye: el worker guarda periódicamente la hora de su último ciclo, sin datos personales. Entra en v2 o cuando ese worker exista en producción.

## Reglas de negocio
1. El worker registra su último latido al menos cada 30 s, también cuando no tiene trabajo.
2. Servicio `worker_ingesta`:
   - `operativo`: último latido hace ≤ 2 min.
   - `degradado` (`atrasado`): último latido hace > 2 min.
   - `caido` (`atrasado`): último latido hace > 10 min.
   - `desconocido` (`sin_datos`): nunca registró un latido.
3. Datos visibles: hora del último latido (America/Lima) y nº de fuentes en `procesando`.
4. Si hay varias instancias del worker, cuenta el latido más reciente. El detalle por instancia queda fuera de alcance.

## Criterios de aceptación
```gherkin
Escenario: Worker activo
  Dado el último latido fue hace 20 s
  Cuando un admin abre «Salud de servicios»
  Entonces «Worker de ingesta» está «Operativo» con la hora del último latido

Escenario: Worker atrasado
  Dado el último latido fue hace 5 min
  Cuando un admin abre la pantalla
  Entonces «Worker de ingesta» está «Degradado — Atrasado»

Escenario: Worker detenido
  Dado el último latido fue hace 15 min y hay 3 fuentes en "procesando"
  Cuando un admin abre la pantalla
  Entonces «Worker de ingesta» está «Caído — Atrasado» y muestra 3 fuentes en procesamiento

Escenario: Sin latidos
  Dado el worker nunca registró un latido
  Cuando un admin abre la pantalla
  Entonces «Worker de ingesta» está «Desconocido — Sin datos» y el estado global es al menos «Degradado»
```

## Fuera de alcance
Reiniciar el worker, reprocesar fuentes desde este panel, detalle por instancia e histórico de latidos.

## Dependencias
PLT-01, PLT-02 y el worker de ingesta de SIA-E2-03/E2-05. El architect define dónde se guarda el latido (tabla o registro).

## Seguridad y privacidad
El latido solo guarda la hora (y, si hace falta, un identificador opaco de instancia). Ni hostname ni IP.

## i18n
es/en: «Worker de ingesta», «Último latido» y «Fuentes en procesamiento».

## Definición de terminado
Tests de los 4 estados con tiempo simulado. El worker registra su latido aunque no tenga trabajo. `tsc`, `lint` y `build` en verde. Claves es/en. Review limpio.
