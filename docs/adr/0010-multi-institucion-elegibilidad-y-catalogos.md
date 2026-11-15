# ADR 0010: multi-institución, con elegibilidad por institución y catálogos de nivel y tipo por institución

- **Estado:** propuesto (2026-11-15). Pendiente de la revisión del tech-lead. El PO confirmó P1–P11 con las decisiones por defecto (2026-11-15; contrato, §0).
  - **Vigente en esta ronda:** solo la **decisión 1** (elegibilidad, PLT-06), la **2** (códigos de error) y la **9** (inscripciones cruzadas).
  - **Diferido** por decisión del usuario (2026-11-15): las decisiones 3 a 8 (catálogos PLT-07, grado exigido PLT-08 y su migración), y también PLT-09, PLT-10 y PLT-14. Se conservan como diseño. La migración del catálogo no tiene número reservado: tomará el siguiente libre.
- **Fecha:** 2026-11-15
- **Historias:** PLT-06 (vigente); PLT-07, PLT-08, PLT-10 y PLT-14 (diferidas). Ver `docs/stories/plataforma/PLT-06-elegibilidad-institucion.md` y los archivos siguientes; las decisiones de la épica están en `docs/stories/plataforma/README.md`, sección PLT-E2.
- **Contrato:** [docs/api/plataforma-multi-institucion.md](../api/plataforma-multi-institucion.md)
- **Relacionado:** [ADR 0011](0011-idioma-guardado-y-avisos-personales.md) (avisos de decisión; usa la institución en el texto)

## Contexto

Capacitate sirve a varias instituciones. Es una herramienta de estudio, no una certificadora. El código asume una sola institución con vocabulario militar:

1. **Elegibilidad sin institución.**
   - `GET /procesos` lista todos los procesos activos (`backend/internal/inscripciones/repository.go:46-57`, sin `WHERE institucion_id`).
   - `Inscribir` solo compara el nivel (`inscripciones/service.go:123-141`).
   - Crear una práctica o un simulacro exige el mismo nivel y una inscripción aprobada (`practices/service.go:76-89`, `simulations/service.go:74-89`). El hueco es **transitivo**: quien se inscribe a un proceso de otra institución y es aprobado puede practicar sobre él.
   - La lista completa de endpoints está en el contrato, §1.
2. **De dónde sale la institución del postulante.**
   - El JWT trae `InstitucionID` y `Nivel` (`internal/jwt/jwt.go:12-19`) y dura 24 h (`jwt.go:10`). `RequireAuth` solo refresca `Rol` desde la BD (`auth/middleware.go:70-71`).
   - Hoy la institución del JWT no queda obsoleta, porque `UpdatePersona` revoca las sesiones si cambia `institucion_id` o `grado_id` (`auth/repository.go:1243-1279`).
   - El **nivel**, en cambio, sale de `grados.nivel` y cambia con `PUT /grados/{id}` (`catalogs/repository.go:242`) sin revocar ninguna sesión.
3. **Vocabulario fijo.**
   - `grados.nivel` tiene `CHECK (nivel IN ('suboficial','oficial'))` (`migrations/012_add_catalog_constraints.sql:5`), con `oneof` en `catalogs/dto.go:42,64`.
   - `procesos.tipo` tiene `oneof=ascenso pase_efectividad admision` (`dto.go:98`), sin CHECK en la BD (`001_create_catalogs.sql:47`).
   - La regla «pase de efectividad exige el grado SO 3°» está escrita a mano (`catalogs/validation.go:8-25`, llamada en `catalogs/service.go:218,235`).
4. **`procesos.nivel` no se escribe por la API.**
   - La columna existe (`025_add_nivel_to_procesos.sql`), pero `CreateProcesoRequest` no la tiene (`dto.go:95-102`) y el INSERT y el UPDATE no la tocan (`catalogs/repository.go:610,629`).
   - Solo la escriben los tests (`questions/questionstest/seed.go:135`, `practices/integration_test.go:175`).
   - En una base real vale NULL, es decir, «todos los niveles».
5. **Datos de prueba y seeds.**
   - Hay 23 INSERT directos por SQL a `grados`, `procesos` o `especialidades` en 13 archivos de test y seed (`db/dbtest/dbtest.go`, `questions/questionstest/seed.go`, `auth/fixture_test.go`, `cmd/seed/admin/main_test.go`, …), repartidos entre los dos backends. Una FK nueva los rompería todos.
   - Otros crean esos registros por el service o por HTTP: `catalogs/routes_test.go` y `cmd/seed/demo/sembrar.go:175,200`. Con cualquier validación contra el catálogo, esos tienen que sembrar antes el nivel o el tipo.

## Opciones consideradas

### 1. Dónde se comprueba la elegibilidad

- **A. En cada servicio, con los claims del JWT.** Es lo más corto, pero el nivel queda obsoleto hasta 24 h (contexto 2) y la regla se repite en tres paquetes. Descartada.
- **B. Refrescar en `RequireAuth` la institución y el nivel desde la BD.** Cambia `FindActiveSesion` para todos los endpoints y toca `auth`, el dominio del otro backend, en la historia crítica. Descartada para PLT-06: no hace falta para cerrar el hueco.
- **C. Un helper en `internal/examen` que lee de la BD, en una consulta, si el proceso existe, si está activo, si es de la misma institución y si el nivel es compatible.** Lo usan inscripciones, prácticas y simulacros. `examen` ya es el paquete compartido de los controles de acceso a un proceso (`examen/examen.go:129-149`). **Elegida.**

### 2. Cómo se modela el catálogo de niveles y tipos

- **A. Una tabla genérica `catalogo_valores(institucion_id, dominio, codigo, …)`.** Mezcla columnas que solo aplican a un dominio (`exige_grado`) y obliga a filtrar por `dominio` en todas partes. Descartada.
- **B. Dos tablas: `institucion_niveles` e `institucion_tipos_proceso`.** Cada una tiene sus columnas, y el tipo lleva `exige_grado`. Las columnas existentes (`grados.nivel`, `especialidades.nivel`, `procesos.nivel`, `procesos.tipo`) **siguen guardando el código** (texto), así que ninguna fila cambia. **Elegida.**
- **C. Reemplazar las columnas de texto por `nivel_id` y `tipo_id`.** Cambiaría todas las filas, las consultas y los DTO (`nivel` y `tipo` viajan como texto en `ProcesoDTO`, `inscripciones/service.go:56-64`) y obligaría a un expand/contract en varios despliegues. Descartada.

### 3. Integridad entre el catálogo y las columnas de texto

- **A. FK compuestas** `grados(institucion_id, nivel)` → `institucion_niveles(institucion_id, codigo)`, y lo mismo para especialidades y procesos.
  - Dan integridad en la BD.
  - Pero no evitan la validación en la app: «un inactivo no se ofrece en altas», el 422 con código y `exige_grado` siguen viviendo allí.
  - Obligan a reescribir los 23 INSERT de prueba (contexto 5).
  - Fallan con un `''` en `procesos.nivel` o `especialidades.nivel` (se leen con `COALESCE`, `examen.go:134`, `catalogs/repository.go:300`). Normalizar `''` a NULL choca con «ninguna fila cambia» (PLT-07, criterio 1).
  - Descartada **por ahora**.
- **B. Validación en el servicio de `catalogs`** contra la tabla de catálogo de la institución del registro, y comprobación de uso antes de borrar. **Elegida.**

### 4. Expand/contract

- El cambio no renombra ni borra columnas. Solo **añade** dos tablas y **quita** un CHECK (`grados_nivel_check`), así que no hace falta un despliegue en dos fases.
- **Binario viejo con el esquema nuevo** (ventana de despliegue): sigue aplicando su `oneof`, más estricto que el catálogo, y escribe valores que ya están en el catálogo de su institución. Es compatible.
- **Binario nuevo con el esquema viejo:** no ocurre, porque `migrate` corre antes de arrancar el servidor.
- **Paso de contract futuro (opcional):** las FK de la opción 3A, cuando las fixtures usen un helper común que siembre el catálogo. Su precondición es que esta consulta devuelva 0 filas:

  ```sql
  SELECT 'grados', id FROM grados g WHERE NOT EXISTS (SELECT 1 FROM institucion_niveles n WHERE n.institucion_id=g.institucion_id AND n.codigo=g.nivel)
  UNION ALL SELECT 'procesos', id FROM procesos p WHERE COALESCE(p.nivel,'')<>'' AND NOT EXISTS (SELECT 1 FROM institucion_niveles n WHERE n.institucion_id=p.institucion_id AND n.codigo=p.nivel)
  UNION ALL SELECT 'procesos', id FROM procesos p WHERE NOT EXISTS (SELECT 1 FROM institucion_tipos_proceso t WHERE t.institucion_id=p.institucion_id AND t.codigo=p.tipo)
  UNION ALL SELECT 'especialidades', id FROM especialidades e WHERE COALESCE(e.nivel,'')<>'' AND NOT EXISTS (SELECT 1 FROM institucion_niveles n WHERE n.institucion_id=e.institucion_id AND n.codigo=e.nivel);
  ```

## Decisión

1. **Elegibilidad (PLT-06).**
   - `examen.AccesoProceso(ctx, q, usuarioID, procesoID)` devuelve `{Activo, MismaInstitucion, NivelOK}` en una sola consulta, o `ErrProcesoNoEncontrado` si el proceso no existe (contrato §2.1).
   - La institución sale de `personas.institucion_id` y el nivel de `grados.nivel` de la persona, **leídos de la BD**, nunca del JWT.
   - El orden de comprobación es fijo: existe → (activo, solo al inscribirse) → institución → nivel → inscripción aprobada (prácticas y simulacros).
   - `GET /procesos` filtra en SQL con la misma regla.
2. **Códigos de error** (formato 2, README §3): `403 institucion_no_permitida`, distinto de `403 nivel_no_permitido`. `POST /inscripciones` pasa entero al formato 2 y separa su 409 en `ya_inscrito` y `sin_monto_configurado` (#269).
> **Decisiones 3 a 8: DIFERIDAS** (2026-11-15). No se implementan en esta ronda.

3. **Catálogos (PLT-07).**
   - Dos tablas: `institucion_niveles(codigo ≤ 30)` e `institucion_tipos_proceso(codigo ≤ 50, exige_grado)`. Los anchos son los de las columnas que guardan el código (`001:17,47`, `025`, `026`).
   - Cada código es único por institución y no cambia una vez creado.
   - Un valor se crea, se renombra, se reordena, se activa o se inactiva. Se borra solo si no está en uso (`409 valor_en_uso`).
   - Toda mutación se audita.
4. **Validación contra el catálogo.**
   - Los `POST` y `PUT` de grados, especialidades y procesos validan `nivel` y `tipo` contra el catálogo **de la institución del registro**: `422 nivel_invalido` o `422 tipo_invalido`.
   - Un valor inactivo solo se acepta si el registro ya lo tenía, en la misma institución.
   - Se quitan los `oneof` de `dto.go:42,64,98` y el CHECK de la 012.
5. **`procesos.nivel` pasa a escribirse por la API** (PLT-07, P5): campo opcional en `POST` y `PUT /admin/procesos`, donde `null` o ausente significa «todos los niveles».
6. **Grado exigido (PLT-08).** `ValidateSO3Grado` se reemplaza por esta regla:
   - si el tipo tiene `exige_grado`, falta `grado_id` → `422 grado_requerido`;
   - un `grado_id` de otra institución → `422 grado_otra_institucion`.

   Sin migración.
7. **Migración de datos (sin número reservado, guiada por los datos).**
   - Por institución, se crea en su catálogo cada valor distinto de nivel en uso (en grados, especialidades y procesos) y cada tipo en uso (en procesos). NULL y `''` no se siembran, porque significan «todos».
   - `exige_grado = (codigo = 'pase_efectividad')`.
   - No se modifica ninguna fila de `grados`, `especialidades`, `procesos` ni `inscripciones`.
   - Detalle en el contrato, §8.
8. **Carrera entre borrar y dar de alta.**
   - `DELETE` comprueba el uso y borra en una transacción con `SELECT … FOR UPDATE` sobre la fila del catálogo. Un alta concurrente que validó el valor antes del borrado puede terminar con un código huérfano.
   - **Se acepta:** solo lo hace un admin, es improbable y el efecto es acotado. El registro muestra el código y, al editarlo, hay que elegir un valor válido. El paso de contract (opción 3A) lo cierra.
9. **Inscripciones cruzadas existentes:** no se tocan. El PR de PLT-06 adjunta el conteo y los ids (contrato §2.5); el PO decide caso por caso.

## Consecuencias

- **Positivas:**
  - Se cierra la fuga en inscripción, práctica y simulacro con un solo helper y sin migración, así que PLT-06 no depende de la numeración.
  - Una institución nueva define su vocabulario sin cambios de código.
  - Ninguna fila existente cambia.
- **Negativas:**
  - La integridad entre el catálogo y las columnas de texto la garantiza la app, no la BD (decisión 8).
  - Una institución nueva no puede tener grados ni procesos hasta que el admin define su catálogo (decisión del PO 7). PLT-09 lo explica en la UI.
  - Los seeds y los tests que crean grados, especialidades o procesos por el servicio o por HTTP (`cmd/seed/demo/sembrar.go:175,200`, `catalogs/routes_test.go`) tienen que crear antes el valor en el catálogo, o reciben `422`. La firma de `Inscribir` cambia, y su uso en `cmd/seed/demo/sembrar.go:581` se ajusta en PLT-06.
  - **Dependencia de despliegue:** si el backend de PLT-07 llega a producción sin PLT-09, una institución nueva (con el catálogo vacío) no puede tener grados ni procesos desde la UI. Las instituciones que ya tienen datos no se ven afectadas, porque la migración siembra su catálogo. Conviene desplegar PLT-07 y PLT-09 en el mismo release.
- *(Diferido)* **El Down de la migración del catálogo pierde** nombres, orden, estado y `exige_grado` del catálogo. Recrea `grados_nivel_check` como `NOT VALID`: si ya existen niveles propios (p. ej. «cadete»), esas filas quedan fuera del CHECK pero no se borran.
- **Fuera de este ADR:**
  - admins por institución;
  - que una persona pertenezca a dos instituciones;
  - las lecturas de configuración por `proceso_id` (contrato §1, filas 6 y 7; P8).
