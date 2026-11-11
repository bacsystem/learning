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

**Dependencia de esta historia:** el cron debe registrar cada ejecución (inicio, fin, resultado y tamaño), o al menos la **última copia correcta**, en un lugar que la API pueda leer. **Lo definen el architect y devops**, con dos restricciones de la ADR:
- El cron conecta con `backup_ro`, que es de solo lectura: no puede escribir en la base con ese rol.
- El backend no recibe las credenciales de R2: la API no consulta R2 ni hace HEAD al bucket para esto.

## Reglas de negocio
1. Servicio `copia_r2`:
   - `operativo`: última copia correcta hace ≤ 26 h.
   - `degradado` (`atrasado`): hace > 26 h, o la última ejecución falló pero hay una correcta dentro de las 26 h.
   - `caido` (`atrasado`): última copia correcta hace > 50 h.
   - `desconocido` (`sin_datos`): no hay ninguna ejecución registrada.
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

Escenario: Sin ejecuciones
  Dado no hay ninguna ejecución registrada
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
- Registro de la última copia correcta en un lugar que la API pueda leer: lo definen el architect y devops, respetando que `backup_ro` es de solo lectura y que el backend no recibe credenciales de R2.

Las alertas por fallo del cron (notificación de Railway, ADR 0004) siguen siendo de devops: este panel no las reemplaza.

## Seguridad y privacidad
Ni credenciales, ni bucket, ni rutas, ni endpoint. La copia contiene datos personales: este panel solo muestra metadatos (fecha, resultado y tamaño).

## i18n
es/en: «Copia diaria», «Última copia correcta», «Última ejecución», «Correcta», «Fallida» y «Tamaño».

## Definición de terminado
Tests de los 4 estados con tiempo simulado y de la respuesta sin datos de R2. `tsc`, `lint` y `build` en verde. Claves es/en. Review limpio.
