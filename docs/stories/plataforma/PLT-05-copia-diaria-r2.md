---
id: PLT-05
titulo: Última copia diaria a Cloudflare R2
epica: PLT-E1
fase: v2
prioridad: Media
estimacion: 2
roles: [admin]
depende_de: [PLT-01, PLT-02, ADR-0004]
---

# PLT-05 Última copia diaria a Cloudflare R2

**Como** admin, **quiero** ver cuándo se hizo la última copia correcta de la base de datos, **para** saber que, si hay que restaurar, tenemos una copia reciente.

## Contexto
La copia diaria está decidida en la [ADR 0004, Decisión 11](../../adr/0004-alojamiento-base-de-datos-produccion.md), aprobada por el usuario. Es un cron job de Railway que hace un `pg_dump` con el rol de solo lectura `backup_ro`, lo cifra con `age` y lo sube a un bucket privado de Cloudflare R2. Devops lo implementa en `deploy/backup/`. Según la ADR, el script registra solo fecha, tamaño y resultado.

**Registro de ejecuciones** (diseño del architect, aprobado por el tech-lead y coordinado con devops):
- Al terminar, el cron hace un INSERT en una tabla nueva, **`copias_bd`**, con la hora de fin, el resultado (`correcta` o `fallida`) y el tamaño. No guarda datos personales, bucket, rutas ni credenciales.
- Usa el mismo rol `backup_ro`, que sigue siendo de solo lectura sobre los datos, más **un único permiso de INSERT sobre `copias_bd`**.
- La API lee esa tabla. No consulta R2 ni las APIs de Railway, y el backend no recibe las credenciales de R2.
- Si una ejecución falla porque la base de datos no responde, no queda fila. El estado sale de la antigüedad de la última copia correcta (`atrasado`).

## Reglas de negocio
1. Servicio `copia_r2`:
   - `operativo`: última copia correcta hace ≤ 26 h.
   - `degradado` (`atrasado`): hace > 26 h, o la última ejecución falló pero hay una correcta dentro de las 26 h.
   - `caido` (`atrasado`): última copia correcta hace > 50 h, o hay ejecuciones registradas pero **ninguna correcta** (contrato `docs/api/plataforma-salud.md`).
   - `desconocido` (`sin_datos`): **solo** cuando `copias_bd` no tiene ninguna fila.
2. Datos visibles: fecha y hora de la última copia correcta (America/Lima), resultado de la última ejecución (correcta o fallida) y tamaño de la copia en MB.
3. Nunca se muestran el nombre del bucket, la ruta o clave del objeto, el endpoint ni las credenciales de R2.

## Criterios de aceptación
```gherkin
Escenario: Copia al día
  Dado la última copia correcta fue hace 6 h
  Cuando un admin abre «Salud de servicios»
  Entonces «Copia diaria» está «Operativo» con la fecha de la copia y su tamaño

Escenario: Última ejecución fallida
  Dado la ejecución de hoy falló y la correcta anterior fue hace 25 h
  Cuando un admin abre la pantalla
  Entonces «Copia diaria» está «Degradado» y muestra que la última ejecución falló

Escenario: Sin copia reciente
  Dado la última copia correcta fue hace 60 h
  Cuando un admin abre la pantalla
  Entonces «Copia diaria» está «Caído — Atrasado» y el estado global es «Caído»

Escenario: Ejecución sin registro por base caída
  Dado la última copia correcta registrada fue hace 30 h y la ejecución de hoy no dejó fila porque la base no respondía
  Cuando un admin abre la pantalla
  Entonces «Copia diaria» está «Degradado — Atrasado»

Escenario: Ejecuciones sin ninguna correcta
  Dado copias_bd tiene 2 ejecuciones, ambas fallidas, y ninguna correcta
  Cuando un admin abre la pantalla
  Entonces «Copia diaria» está «Caído — Atrasado», no «Desconocido», y el estado global es «Caído»

Escenario: Sin ejecuciones
  Dado copias_bd no tiene filas
  Cuando un admin abre la pantalla
  Entonces «Copia diaria» está «Desconocido — Sin datos»

Escenario: Sin datos de R2
  Dado una copia registrada
  Cuando un admin consulta el estado de salud
  Entonces la respuesta no contiene bucket, ruta del objeto, endpoint ni credenciales
```

## Fuera de alcance
Lanzar una copia o restaurar desde el panel, listar o descargar copias, verificar la integridad de la copia y política de retención (devops).

## Dependencias
- PLT-01 y PLT-02.
- Cron de copia diaria de la ADR 0004 (Decisión 11), que implementa devops en `deploy/backup/`.
- Migración de la tabla `copias_bd` y permiso INSERT de `backup_ro` sobre ella. El contrato es del architect y el permiso se da al aprovisionar (devops, `docs/ops`).
- El cron hace el INSERT al terminar cada ejecución (devops).

Las alertas por fallo del cron (notificación de Railway, ADR 0004) siguen siendo de devops: este panel no las reemplaza.

## Seguridad y privacidad
Ni credenciales, ni bucket, ni rutas, ni endpoint. La copia contiene datos personales: este panel solo muestra metadatos (fecha, resultado y tamaño).

## i18n
es/en: «Copia diaria», «Última copia correcta», «Última ejecución», «Correcta», «Fallida» y «Tamaño».

## Definición de terminado
Tests de los 4 estados con tiempo simulado (incluidas la ejecución sin fila y las ejecuciones sin ninguna correcta) y de la respuesta sin datos de R2. `tsc`, `lint` y `build` en verde. Claves es/en. Review limpio.
