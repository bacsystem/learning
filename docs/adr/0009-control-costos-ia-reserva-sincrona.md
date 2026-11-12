# ADR 0009: control de costos de IA con reserva síncrona de acciones y corte del presupuesto por gasto registrado

- **Estado:** propuesto (2026-11-12). Pendiente de revisión del tech-lead.
- **Fecha:** 2026-11-12
- **Historia:** SIA-E6-02. La usan E3-04, E6-03, E6-06, E6-08 y las siguientes
- **Contrato:** [docs/api/serums-ia-e6-02-control-costos.md](../api/serums-ia-e6-02-control-costos.md)
- **Relacionadas:** [ADR 0002](0002-secreto-hmac-contadores-antifraude.md) (HMAC de los contadores) y [ADR 0006](0006-puerto-servicio-ia.md) (puerto de IA y llamadas fuera del request)
- **Decisiones de producto de origen:** D14–D21 del PO (supuesto 25 del README de historias)

## Contexto

SIA-E6-02 fija un límite de acciones por actor y un presupuesto global en USD. El PO precisó cómo deben comportarse:

- **D17, límites:** atómicos y todo o nada. Un lote de N acciones (E3-04 regla 6) se reserva entero o no se reserva nada, con `429`.
- **D14, presupuesto:** se corta cuando el gasto registrado es ≥ presupuesto, **antes** de llamar. Se acepta que se pase por las llamadas que ya estaban en curso. La regla 10 estricta vale para las acciones, no para los USD.
- **D15, fallos:** una llamada fallida libera su acción, pero su costo sí se suma.
- **D16, orden:** caché → presupuesto → límite mensual → límite diario. La caché se sirve aunque todo lo demás esté agotado.
- **D18, borrado de datos:** el gasto del mes no baja cuando SIA-E9-07 borra o anonimiza filas de `ia_usos`.

Esto choca con el código y los contratos existentes:

- **Las llamadas van fuera del request.** La ADR 0006 las prohíbe en un request síncrono (hasta 6 llamadas de 60 s frente al `WriteTimeout` de 15 s): se hacen en un job. Pero el `429` tiene que llegar **al pedir la acción**. Si el límite se verificara solo en el job, el escenario «Concurrencia» aceptaría las 3 peticiones y fallaría 2 en segundo plano, sin `429`.
- **Los contadores del postulante van por `HMAC(usuario_id)`** y sobreviven al borrado (ADR 0002). Siguen el patrón de la tabla de 056.
- **El gasto no puede depender de `ia_usos`.** Esas filas llevan `actor_id`, así que E9-07 las borra (contrato E6-01 §6), y además se escriben aparte y pueden fallar (E6-01, paso 7).

## Opciones consideradas

### Acciones

1. **Verificar en el request y contar en el job.** No es atómico. Descartada.
2. **Reservar en el request y confirmar o liberar en el job.** La reserva incrementa los contadores con `INSERT … ON CONFLICT DO UPDATE … WHERE usados + n <= límite`. **Elegida.**

### Presupuesto

1. **Reservar el costo máximo de cada invocación.** Es estricto, pero el PO descartó el estimado (D14). Descartada.
2. **Cortar cuando `gastado ≥ presupuesto`, verificando antes de cada llamada.** Es lo que pide D14. **Elegida.**
3. **Calcular `gastado` como `SUM(ia_usos.costo_usd)`.** Bajaría con el borrado de E9-07 (D18). Descartada en favor de un acumulador propio del mes.

### Configuración

1. **`configuracion_sistema`** (clave y valor `TEXT`, sin `CHECK`; `027`, `auth/repository.go:725-775`). Descartada.
2. **Tabla `ia_configuracion` de una sola fila, tipada y con `CHECK`.** **Elegida.**

### Contadores del admin

El PO no exige HMAC para ellos. Aun así se guardan en **la misma tabla y con la misma clave HMAC** que los del postulante (con otro límite): una sola ruta de código y ningún dato en claro. Es lo más simple.

## Decisión

- **En el request (síncrono, sin LLM):**
  1. Se consulta la caché. Si hay respuesta, se sirve: sin reservar, sin gasto y aunque el presupuesto o el límite estén agotados.
  2. Si no, `Reservar(actor, n)`, todo o nada, en el orden D16:
     - presupuesto agotado → `429 presupuesto_agotado`;
     - mes → `429 limite_ia_mensual`;
     - día → `429 limite_ia`.
  3. Se encola el job con la reserva (`acciones`, inicio de la ventana del día y del mes).
- **En el job, por invocación:**
  1. Caché: si ya está, libera 1 acción.
  2. Presupuesto (`gastado ≥ presupuesto`): si está agotado, libera la acción y devuelve `presupuesto_agotado`.
  3. Llamada al LLM.
  4. En una transacción: `gastado += costo real` de todos los intentos, **también si la llamada falló**, y evaluación de la alerta del 80 %.
  5. Si salió bien, la acción se consume y se escribe la caché. Si falló, se libera la acción.
- **Reintento:** un job reintentado reutiliza su reserva.
- **Reservas huérfanas de acciones:** si un job muere sin liberar, sus acciones siguen contadas hasta que vence la ventana. Es una pérdida conservadora para el usuario, nunca un exceso. El consumidor libera lo que no use cuando termina, también tras un corte de presupuesto.
- **La clave HMAC no se guarda en el job:** se recalcula al liberar, con la clave vigente y, si la fila no existe, con la anterior.
- **El gasto es un acumulador propio** (`ia_gasto_mes.gastado_usd`), sin `actor_id`. E9-07 no lo toca (D18). `ia_usos` sigue siendo el detalle por operación.
- **Alerta del 80 % (D19, D20):**
  - `ia_gasto_mes` guarda para qué valor del presupuesto ya se envió la alerta.
  - Se notifica una vez cuando `gastado ≥ 0,8 × presupuesto` y ese valor todavía no fue avisado. Se evalúa al sumar gasto y también al cambiar el presupuesto.
  - Cambiar el presupuesto, para arriba o para abajo, rearma la alerta para el nuevo valor. Si el gasto ya está sobre el 80 % del nuevo valor, se envía en ese momento.
  - Bajar el presupuesto a un valor ≤ al gasto corta las llamadas al instante (regla 18).
  - Va por la campana (`notificaciones`, `destinatario = 'rol:admin'`, `creado_por = NULL`; los lectores ya resuelven `rol:<rol>` en `notifications/repository.go:76-77`).
- **Zona horaria:** las ventanas van en `America/Lima`, con `time/tzdata` (librería estándar) importado en `cmd/server`.
- **Caché compartida entre actores**, sin guardar el actor:
  - La clave es el SHA-256 de la operación, el esquema, las instrucciones y los datos normalizados.
  - Al escribir, se guarda la versión vigente de cada fuente usada.
  - La entrada vale 30 días y mientras esas fuentes sigan `activa` con la misma versión vigente.
  - Se compara con la versión vigente al escribir, no con la versión del contenido. Así, una explicación sobre un pasaje de una versión citada anterior (E6-08) no nace inválida.

## Consecuencias

- **Positivas:**
  - El `429` de acciones llega al pedir y es atómico.
  - El presupuesto es simple: sin estimados ni reservas.
  - El gasto no baja con el borrado de datos.
  - No hay dependencias nuevas.
- **Negativas:**
  - **Exceso aceptado (D14):** las invocaciones que ya estaban en curso cuando el gasto llega al 100 % terminan y suman su costo. El exceso está acotado por la concurrencia de los workers, multiplicada por el costo de una invocación.
  - Un job que muere sin liberar le cuesta al usuario las acciones reservadas hasta que vence la ventana.
  - Cada consumidor guarda la reserva en su job y libera lo que no usa.
  - `gastado_usd` y la suma de `ia_usos.costo_usd` pueden diferir. El acumulador manda.
- **Fuera de esta ADR:** el proveedor real (SIA-E6-10, #173), que necesita la aprobación del usuario y su propia ADR. Hasta entonces, el doble expone un `CostoUSD` configurable para los tests.
